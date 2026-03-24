package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
)

//go:embed frontend
var frontendFS embed.FS

const (
	slopspacesRoot = "/workspaces/slopspaces"
	listenAddr     = ":8080"
)

// ── Types ─────────────────────────────────────────────────────────────────────

type Session struct {
	ID       string     `json:"id"`
	Path     string     `json:"path"`
	Category string     `json:"category"`
	Status   string     `json:"status"`
	Type     string     `json:"type"`
	Modified time.Time  `json:"modified"`
	Files    []FileInfo `json:"files"`
}

type FileInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

type Record struct {
	ID       string     `json:"id"`
	Path     string     `json:"path"`
	Kind     string     `json:"kind"` // invocation, session, worker
	Modified time.Time  `json:"modified"`
	Files    []FileInfo `json:"files,omitempty"`
}

type WSEvent struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Op   string `json:"op"`
	Time string `json:"time"`
}

// ── WebSocket Hub ─────────────────────────────────────────────────────────────

type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
}

var hub = &Hub{clients: make(map[*websocket.Conn]struct{})}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (h *Hub) add(c *websocket.Conn) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *Hub) broadcast(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		_ = c.WriteMessage(websocket.TextMessage, data)
	}
}

// ── Session Discovery ─────────────────────────────────────────────────────────

type scanTarget struct {
	dir      string
	category string
	status   string
}

var scanTargets = []scanTarget{
	{filepath.Join(slopspacesRoot, "input", "any"), "input", "active"},
	{filepath.Join(slopspacesRoot, "heuristic", "pending"), "heuristic", "pending"},
	{filepath.Join(slopspacesRoot, "heuristic", "processed"), "heuristic", "processed"},
	{filepath.Join(slopspacesRoot, "output"), "output", "complete"},
	{filepath.Join(slopspacesRoot, "working"), "working", "active"},
	{filepath.Join(slopspacesRoot, "requests"), "requests", "pending"},
	{filepath.Join(slopspacesRoot, "dispatcher", "live", "flows"), "dispatcher", "live"},
}

func listSessions() []Session {
	var sessions []Session
	for _, t := range scanTargets {
		entries, err := os.ReadDir(t.dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			sessionPath := filepath.Join(t.dir, e.Name())
			info, _ := e.Info()
			s := Session{
				ID:       e.Name(),
				Path:     sessionPath,
				Category: t.category,
				Status:   t.status,
				Type:     detectSessionType(sessionPath),
				Files:    listTopFiles(sessionPath),
			}
			if info != nil {
				s.Modified = info.ModTime()
			}
			sessions = append(sessions, s)
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Modified.After(sessions[j].Modified)
	})
	return sessions
}

func detectSessionType(dir string) string {
	if data, err := os.ReadFile(filepath.Join(dir, "DISPATCH.json")); err == nil {
		var m map[string]interface{}
		if json.Unmarshal(data, &m) == nil {
			if t, ok := m["type"].(string); ok {
				return t
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "DISPATCH_RECORD.json")); err == nil {
		var m map[string]interface{}
		if json.Unmarshal(data, &m) == nil {
			if t, ok := m["type"].(string); ok {
				return t
			}
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "INSTRUCTION.json")); err == nil {
		return "repo-isolation"
	}
	return "unknown"
}

func listTopFiles(dir string) []FileInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []FileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		fi := FileInfo{
			Name: e.Name(),
			Path: filepath.Join(dir, e.Name()),
		}
		if info != nil {
			fi.Size = info.Size()
			fi.Modified = info.ModTime()
		}
		files = append(files, fi)
	}
	return files
}

// ── Agent Records Discovery ───────────────────────────────────────────────────

func listRecords() []Record {
	recordsDir := filepath.Join(slopspacesRoot, "agent-records")

	entries, err := os.ReadDir(recordsDir)
	if err != nil {
		return nil
	}

	var records []Record
	for _, e := range entries {
		if e.Name() == "worker" {
			continue // handled separately
		}
		info, _ := e.Info()
		kind := "invocation"
		if strings.HasPrefix(e.Name(), "session-") {
			kind = "session"
		}
		r := Record{
			ID:   e.Name(),
			Path: filepath.Join(recordsDir, e.Name()),
			Kind: kind,
		}
		if info != nil {
			r.Modified = info.ModTime()
		}
		if e.IsDir() {
			r.Files = listTopFiles(r.Path)
		}
		records = append(records, r)
	}

	// Worker records
	workerDir := filepath.Join(recordsDir, "worker")
	if workerEntries, err := os.ReadDir(workerDir); err == nil {
		for _, e := range workerEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			info, _ := e.Info()
			r := Record{
				ID:   "worker/" + e.Name(),
				Path: filepath.Join(workerDir, e.Name()),
				Kind: "worker",
			}
			if info != nil {
				r.Modified = info.ModTime()
			}
			records = append(records, r)
		}
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Modified.After(records[j].Modified)
	})
	return records
}

// ── File Watcher ──────────────────────────────────────────────────────────────

func startWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("watcher init error: %v", err)
		return
	}

	addDir := func(path string) {
		if err := watcher.Add(path); err != nil {
			log.Printf("watch add %s: %v", path, err)
		}
	}

	// Watch top-level and scan target directories
	addDir(slopspacesRoot)
	for _, t := range scanTargets {
		addDir(t.dir)
		// Watch existing session subdirectories
		if entries, err := os.ReadDir(t.dir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					addDir(filepath.Join(t.dir, e.Name()))
				}
			}
		}
	}
	addDir(filepath.Join(slopspacesRoot, "agent-records"))

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				op := "write"
				switch {
				case event.Has(fsnotify.Create):
					op = "create"
					// Auto-watch newly created directories
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						addDir(event.Name)
					}
				case event.Has(fsnotify.Remove):
					op = "remove"
				case event.Has(fsnotify.Rename):
					op = "rename"
				}
				rel, _ := filepath.Rel(slopspacesRoot, event.Name)
				hub.broadcast(WSEvent{
					Type: "file_event",
					Path: rel,
					Op:   op,
					Time: time.Now().UTC().Format(time.RFC3339),
				})
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("watcher error: %v", err)
			}
		}
	}()
}

// ── HTTP Handlers ─────────────────────────────────────────────────────────────

func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	hub.add(conn)
	defer func() {
		hub.remove(conn)
		conn.Close()
	}()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func handleSessions(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listSessions())
}

func handleSession(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	id := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	for _, s := range listSessions() {
		if s.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s)
			return
		}
	}
	http.NotFound(w, r)
}

func handleFileContent(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, slopspacesRoot) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if strings.HasSuffix(abs, ".json") {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	w.Write(data)
}

func handleRecords(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listRecords())
}

func handleRecord(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	id := strings.TrimPrefix(r.URL.Path, "/api/records/")
	for _, rec := range listRecords() {
		if rec.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rec)
			return
		}
	}
	http.NotFound(w, r)
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", handleWS)
	mux.HandleFunc("/api/sessions", handleSessions)
	mux.HandleFunc("/api/sessions/", handleSession)
	mux.HandleFunc("/api/file", handleFileContent)
	mux.HandleFunc("/api/records", handleRecords)
	mux.HandleFunc("/api/records/", handleRecord)

	sub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	go startWatcher()

	log.Printf("Agent Viewer listening on http://localhost%s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}
