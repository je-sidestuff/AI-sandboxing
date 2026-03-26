package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
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
	slopspacesRoot    = "/workspaces/slopspaces"
	listenAddr        = ":8080"
	githubPollInterval = 150 * time.Second // 2.5 minutes
)

// ── Flow Types ─────────────────────────────────────────────────────────────────

// FlowState represents the current state of a flow
type FlowState string

const (
	FlowStatePending      FlowState = "pending"       // Heuristic submitted, awaiting processing
	FlowStateProcessing   FlowState = "processing"    // Heuristic being processed by AI
	FlowStateProposed     FlowState = "proposed"      // Dispatch created, awaiting approval
	FlowStateApproved     FlowState = "approved"      // Approved, ready for execution
	FlowStateExecuting    FlowState = "executing"     // Agent executing work
	FlowStateIsolated     FlowState = "isolated"      // Work in isolation repo (PR open)
	FlowStateRevising     FlowState = "revising"      // Responding to revision request
	FlowStateReintegrating FlowState = "reintegrating" // Reintegration PR open in target repo
	FlowStateMerged       FlowState = "merged"        // Successfully merged
	FlowStateClosed       FlowState = "closed"        // PR closed without merge
	FlowStateFailed       FlowState = "failed"        // Failed at some stage
	FlowStateComplete     FlowState = "complete"      // Completed (direct dispatch)
)

// FlowType represents the dispatch type
type FlowType string

const (
	FlowTypeDirect        FlowType = "direct"
	FlowTypeInRepo        FlowType = "in-repo"
	FlowTypeRepoIsolation FlowType = "repo-isolation"
	FlowTypeUnknown       FlowType = "unknown"
)

// FlowStage represents a stage in the flow lifecycle
type FlowStage struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"` // "pending", "active", "completed", "failed", "skipped"
	StartTime  time.Time `json:"start_time,omitempty"`
	EndTime    time.Time `json:"end_time,omitempty"`
	Path       string    `json:"path,omitempty"`
	PRUrl      string    `json:"pr_url,omitempty"`
	PRState    string    `json:"pr_state,omitempty"` // "open", "merged", "closed"
	PRTitle    string    `json:"pr_title,omitempty"`
	Files      []FileInfo `json:"files,omitempty"`
	Details    map[string]interface{} `json:"details,omitempty"`
}

// Flow represents a complete flow lifecycle
type Flow struct {
	ID              string      `json:"id"`
	State           FlowState   `json:"state"`
	Type            FlowType    `json:"type"`
	Created         time.Time   `json:"created"`
	Modified        time.Time   `json:"modified"`

	// Origin information
	HeuristicPath   string      `json:"heuristic_path,omitempty"`
	HeuristicText   string      `json:"heuristic_text,omitempty"`

	// Dispatch information
	DispatchPath    string      `json:"dispatch_path,omitempty"`
	Instruction     string      `json:"instruction,omitempty"`
	TargetRepo      string      `json:"target_repo,omitempty"`

	// PR information
	IsolationPRUrl   string     `json:"isolation_pr_url,omitempty"`
	IsolationPRState string     `json:"isolation_pr_state,omitempty"`
	ReintegrationPRUrl   string `json:"reintegration_pr_url,omitempty"`
	ReintegrationPRState string `json:"reintegration_pr_state,omitempty"`

	// Stage tracking
	Stages          []FlowStage `json:"stages"`
	CurrentStage    string      `json:"current_stage"`

	// Error information
	Error           string      `json:"error,omitempty"`

	// All paths associated with this flow
	Paths           []string    `json:"paths,omitempty"`
}

// FileInfo represents a file in a flow directory
type FileInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// PRInfo represents cached PR information from GitHub
type PRInfo struct {
	URL       string    `json:"url"`
	State     string    `json:"state"` // "open", "merged", "closed"
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
	Comments  []PRComment `json:"comments,omitempty"`
}

type PRComment struct {
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	Author    string    `json:"author"`
}

// ── WebSocket Types ────────────────────────────────────────────────────────────

type WSEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
	Time    string      `json:"time"`
}

type FileEvent struct {
	Path string `json:"path"`
	Op   string `json:"op"`
}

type FlowUpdate struct {
	FlowID string `json:"flow_id"`
	State  string `json:"state"`
	Stage  string `json:"stage,omitempty"`
}

// ── Global State ───────────────────────────────────────────────────────────────

var (
	flowCache     = make(map[string]*Flow)
	flowCacheMu   sync.RWMutex
	prCache       = make(map[string]*PRInfo)
	prCacheMu     sync.RWMutex
	lastPRPoll    time.Time
	lastPRPollMu  sync.Mutex
)

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

func broadcastFlowUpdate(flowID string, state FlowState, stage string) {
	hub.broadcast(WSEvent{
		Type: "flow_update",
		Payload: FlowUpdate{
			FlowID: flowID,
			State:  string(state),
			Stage:  stage,
		},
		Time: time.Now().UTC().Format(time.RFC3339),
	})
}

func broadcastFileEvent(path, op string) {
	hub.broadcast(WSEvent{
		Type: "file_event",
		Payload: FileEvent{
			Path: path,
			Op:   op,
		},
		Time: time.Now().UTC().Format(time.RFC3339),
	})
}

// ── Flow Discovery ─────────────────────────────────────────────────────────────

func discoverFlows() []Flow {
	flowCacheMu.Lock()
	defer flowCacheMu.Unlock()

	discovered := make(map[string]*Flow)

	// 1. Scan heuristic/pending for new heuristic requests
	scanHeuristicPending(discovered)

	// 2. Scan heuristic/processed for processed heuristics
	scanHeuristicProcessed(discovered)

	// 3. Scan dispatcher/live/flows for active terraform flows
	scanDispatcherFlows(discovered)

	// 4. Scan input/any for pending work
	scanInputAny(discovered)

	// 5. Scan output for completed flows
	scanOutput(discovered)

	// 6. Scan working directory for in-progress work
	scanWorking(discovered)

	// Merge into cache
	for id, flow := range discovered {
		if existing, ok := flowCache[id]; ok {
			// Merge stages and update state
			mergeFlow(existing, flow)
		} else {
			flowCache[id] = flow
		}
	}

	// Convert to slice and sort
	var flows []Flow
	for _, flow := range flowCache {
		flows = append(flows, *flow)
	}
	sort.Slice(flows, func(i, j int) bool {
		return flows[i].Modified.After(flows[j].Modified)
	})

	return flows
}

func scanHeuristicPending(discovered map[string]*Flow) {
	dir := filepath.Join(slopspacesRoot, "heuristic", "pending")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		flowPath := filepath.Join(dir, e.Name())
		info, _ := e.Info()

		flow := &Flow{
			ID:            e.Name(),
			State:         FlowStatePending,
			Type:          FlowTypeUnknown,
			HeuristicPath: flowPath,
			Paths:         []string{flowPath},
			CurrentStage:  "heuristic",
			Stages: []FlowStage{{
				Name:   "heuristic",
				Status: "active",
				Path:   flowPath,
				Files:  listTopFiles(flowPath),
			}},
		}

		if info != nil {
			flow.Created = info.ModTime()
			flow.Modified = info.ModTime()
			flow.Stages[0].StartTime = info.ModTime()
		}

		// Check if being processed
		if _, err := os.Stat(filepath.Join(flowPath, "PROCESSING.md")); err == nil {
			flow.State = FlowStateProcessing
		}

		// Read heuristic content
		if data, err := os.ReadFile(filepath.Join(flowPath, "HEURISTIC.md")); err == nil {
			flow.HeuristicText = truncate(string(data), 200)
		}

		discovered[e.Name()] = flow
	}
}

func scanHeuristicProcessed(discovered map[string]*Flow) {
	dir := filepath.Join(slopspacesRoot, "heuristic", "processed")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		flowPath := filepath.Join(dir, e.Name())
		info, _ := e.Info()

		if existing, ok := discovered[e.Name()]; ok {
			// Update existing flow
			existing.Stages[0].Status = "completed"
			if info != nil {
				existing.Stages[0].EndTime = info.ModTime()
			}
			continue
		}

		// New flow we haven't seen yet
		flow := &Flow{
			ID:            e.Name(),
			State:         FlowStateProposed,
			Type:          FlowTypeUnknown,
			HeuristicPath: flowPath,
			Paths:         []string{flowPath},
			CurrentStage:  "dispatch",
			Stages: []FlowStage{{
				Name:   "heuristic",
				Status: "completed",
				Path:   flowPath,
				Files:  listTopFiles(flowPath),
			}},
		}

		if info != nil {
			flow.Created = info.ModTime()
			flow.Modified = info.ModTime()
			flow.Stages[0].EndTime = info.ModTime()
		}

		// Read heuristic content
		if data, err := os.ReadFile(filepath.Join(flowPath, "HEURISTIC.md")); err == nil {
			flow.HeuristicText = truncate(string(data), 200)
		}

		discovered[e.Name()] = flow
	}
}

func scanDispatcherFlows(discovered map[string]*Flow) {
	baseDir := filepath.Join(slopspacesRoot, "dispatcher", "live", "flows")

	// Scan each dispatch type directory
	for _, dtype := range []string{"repo-isolation", "in-repo", "direct"} {
		dir := filepath.Join(baseDir, dtype)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}

			flowPath := filepath.Join(dir, e.Name())
			flowID := e.Name()
			info, _ := e.Info()

			// Read flow record files
			flowType := FlowType(dtype)
			flowState := FlowStateExecuting
			var prURL, prState, reintPRUrl, reintPRState, targetRepo, instruction string

			// Look for flow_*.json files
			flowFiles, _ := filepath.Glob(filepath.Join(flowPath, "flow_*.json"))
			for _, ff := range flowFiles {
				if data, err := os.ReadFile(ff); err == nil {
					var m map[string]interface{}
					if json.Unmarshal(data, &m) == nil {
						if s, ok := m["status"].(string); ok {
							switch s {
							case "running":
								flowState = FlowStateExecuting
							case "monitoring":
								flowState = FlowStateIsolated
							case "completed":
								flowState = FlowStateMerged
							case "failed":
								flowState = FlowStateFailed
							}
						}
						if pr, ok := m["pr_url"].(string); ok {
							prURL = pr
						}
						if cs, ok := m["conclusion_state"].(string); ok {
							prState = cs
							switch cs {
							case "merged":
								flowState = FlowStateMerged
							case "closed":
								flowState = FlowStateClosed
							}
						}
						if reint, ok := m["reintegration_url"].(string); ok {
							reintPRUrl = reint
						}
						if repo, ok := m["target_repo"].(string); ok {
							targetRepo = repo
						}
					}
				}
			}

			// Read DISPATCH.json if present
			if data, err := os.ReadFile(filepath.Join(flowPath, "DISPATCH.json")); err == nil {
				var m map[string]interface{}
				if json.Unmarshal(data, &m) == nil {
					if inst, ok := m["instruction"].(string); ok {
						instruction = truncate(inst, 200)
					}
					if repo, ok := m["target_repo"].(string); ok && targetRepo == "" {
						targetRepo = repo
					}
				}
			}

			// Determine current stage based on state
			currentStage := "dispatch"
			stages := []FlowStage{}

			// Build stages based on flow type and state
			switch flowType {
			case FlowTypeRepoIsolation:
				stages = buildRepoIsolationStages(flowState, prURL, prState, reintPRUrl, reintPRState, flowPath)
				currentStage = getCurrentStageRepoIsolation(flowState)
			case FlowTypeInRepo:
				stages = buildInRepoStages(flowState, prURL, prState, flowPath)
				currentStage = getCurrentStageInRepo(flowState)
			case FlowTypeDirect:
				stages = buildDirectStages(flowState, flowPath)
				currentStage = "execution"
			}

			flow := &Flow{
				ID:                   flowID,
				State:                flowState,
				Type:                 flowType,
				DispatchPath:         flowPath,
				Instruction:          instruction,
				TargetRepo:           targetRepo,
				IsolationPRUrl:       prURL,
				IsolationPRState:     prState,
				ReintegrationPRUrl:   reintPRUrl,
				ReintegrationPRState: reintPRState,
				Paths:                []string{flowPath},
				CurrentStage:         currentStage,
				Stages:               stages,
			}

			if info != nil {
				flow.Modified = info.ModTime()
			}

			// Merge with existing if found
			if existing, ok := discovered[flowID]; ok {
				mergeFlow(existing, flow)
			} else {
				discovered[flowID] = flow
			}
		}
	}
}

func buildRepoIsolationStages(state FlowState, prURL, prState, reintPRUrl, reintPRState, path string) []FlowStage {
	stages := []FlowStage{
		{Name: "dispatch", Status: "completed"},
		{Name: "execution", Status: "completed"},
		{Name: "isolation", Status: "pending", PRUrl: prURL, PRState: prState},
		{Name: "reintegration", Status: "pending", PRUrl: reintPRUrl, PRState: reintPRState},
		{Name: "complete", Status: "pending"},
	}

	switch state {
	case FlowStateExecuting:
		stages[2].Status = "pending"
	case FlowStateIsolated, FlowStateRevising:
		stages[2].Status = "active"
	case FlowStateReintegrating:
		stages[2].Status = "completed"
		stages[3].Status = "active"
	case FlowStateMerged:
		stages[2].Status = "completed"
		stages[3].Status = "completed"
		stages[4].Status = "completed"
	case FlowStateClosed, FlowStateFailed:
		if prState == "closed" {
			stages[2].Status = "failed"
		} else if reintPRState == "closed" {
			stages[2].Status = "completed"
			stages[3].Status = "failed"
		}
		stages[4].Status = "failed"
	}

	// Set path on the isolation stage
	stages[2].Path = path
	stages[2].Files = listTopFiles(path)

	return stages
}

func getCurrentStageRepoIsolation(state FlowState) string {
	switch state {
	case FlowStateExecuting:
		return "execution"
	case FlowStateIsolated, FlowStateRevising:
		return "isolation"
	case FlowStateReintegrating:
		return "reintegration"
	default:
		return "complete"
	}
}

func buildInRepoStages(state FlowState, prURL, prState, path string) []FlowStage {
	stages := []FlowStage{
		{Name: "dispatch", Status: "completed"},
		{Name: "execution", Status: "completed"},
		{Name: "review", Status: "pending", PRUrl: prURL, PRState: prState},
		{Name: "complete", Status: "pending"},
	}

	switch state {
	case FlowStateExecuting:
		stages[2].Status = "pending"
	case FlowStateIsolated:
		stages[2].Status = "active"
	case FlowStateMerged:
		stages[2].Status = "completed"
		stages[3].Status = "completed"
	case FlowStateClosed, FlowStateFailed:
		stages[2].Status = "failed"
		stages[3].Status = "failed"
	}

	stages[2].Path = path
	stages[2].Files = listTopFiles(path)

	return stages
}

func getCurrentStageInRepo(state FlowState) string {
	switch state {
	case FlowStateExecuting:
		return "execution"
	case FlowStateIsolated:
		return "review"
	default:
		return "complete"
	}
}

func buildDirectStages(state FlowState, path string) []FlowStage {
	stages := []FlowStage{
		{Name: "dispatch", Status: "completed"},
		{Name: "execution", Status: "active", Path: path, Files: listTopFiles(path)},
		{Name: "complete", Status: "pending"},
	}

	switch state {
	case FlowStateComplete, FlowStateMerged:
		stages[1].Status = "completed"
		stages[2].Status = "completed"
	case FlowStateFailed:
		stages[1].Status = "failed"
		stages[2].Status = "failed"
	}

	return stages
}

func scanInputAny(discovered map[string]*Flow) {
	dir := filepath.Join(slopspacesRoot, "input", "any")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		flowPath := filepath.Join(dir, e.Name())
		flowID := e.Name()
		info, _ := e.Info()

		// Check what type of input this is
		var flowType FlowType = FlowTypeDirect
		var instruction, targetRepo string

		if data, err := os.ReadFile(filepath.Join(flowPath, "DISPATCH.json")); err == nil {
			var m map[string]interface{}
			if json.Unmarshal(data, &m) == nil {
				if t, ok := m["type"].(string); ok {
					flowType = FlowType(t)
				}
				if inst, ok := m["instruction"].(string); ok {
					instruction = truncate(inst, 200)
				}
				if repo, ok := m["target_repo"].(string); ok {
					targetRepo = repo
				}
			}
		} else if data, err := os.ReadFile(filepath.Join(flowPath, "INSTRUCTION.json")); err == nil {
			flowType = FlowTypeDirect
			var m map[string]interface{}
			if json.Unmarshal(data, &m) == nil {
				if inst, ok := m["instruction"].(string); ok {
					instruction = truncate(inst, 200)
				}
			}
		}

		if existing, ok := discovered[flowID]; ok {
			// Update existing flow
			existing.Paths = append(existing.Paths, flowPath)
			if existing.Instruction == "" {
				existing.Instruction = instruction
			}
			continue
		}

		flow := &Flow{
			ID:           flowID,
			State:        FlowStateExecuting,
			Type:         flowType,
			DispatchPath: flowPath,
			Instruction:  instruction,
			TargetRepo:   targetRepo,
			Paths:        []string{flowPath},
			CurrentStage: "execution",
			Stages: []FlowStage{
				{Name: "dispatch", Status: "completed"},
				{Name: "execution", Status: "active", Path: flowPath, Files: listTopFiles(flowPath)},
			},
		}

		if info != nil {
			flow.Created = info.ModTime()
			flow.Modified = info.ModTime()
		}

		discovered[flowID] = flow
	}
}

func scanOutput(discovered map[string]*Flow) {
	// Scan both output and output/content
	for _, subdir := range []string{"", "content"} {
		dir := filepath.Join(slopspacesRoot, "output", subdir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}

			flowPath := filepath.Join(dir, e.Name())
			flowID := e.Name()
			info, _ := e.Info()

			if existing, ok := discovered[flowID]; ok {
				// Mark as complete
				existing.State = FlowStateComplete
				existing.Paths = append(existing.Paths, flowPath)

				// Update stages
				for i := range existing.Stages {
					if existing.Stages[i].Name == "complete" {
						existing.Stages[i].Status = "completed"
						existing.Stages[i].Path = flowPath
						existing.Stages[i].Files = listTopFiles(flowPath)
					} else if existing.Stages[i].Status == "active" {
						existing.Stages[i].Status = "completed"
					}
				}
				existing.CurrentStage = "complete"
				continue
			}

			flow := &Flow{
				ID:           flowID,
				State:        FlowStateComplete,
				Type:         FlowTypeDirect,
				Paths:        []string{flowPath},
				CurrentStage: "complete",
				Stages: []FlowStage{
					{Name: "execution", Status: "completed"},
					{Name: "complete", Status: "completed", Path: flowPath, Files: listTopFiles(flowPath)},
				},
			}

			if info != nil {
				flow.Created = info.ModTime()
				flow.Modified = info.ModTime()
			}

			discovered[flowID] = flow
		}
	}
}

func scanWorking(discovered map[string]*Flow) {
	dir := filepath.Join(slopspacesRoot, "working")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		flowPath := filepath.Join(dir, e.Name())
		flowID := e.Name()
		info, _ := e.Info()

		if existing, ok := discovered[flowID]; ok {
			existing.Paths = append(existing.Paths, flowPath)
			continue
		}

		flow := &Flow{
			ID:           flowID,
			State:        FlowStateExecuting,
			Type:         FlowTypeUnknown,
			Paths:        []string{flowPath},
			CurrentStage: "execution",
			Stages: []FlowStage{
				{Name: "execution", Status: "active", Path: flowPath, Files: listTopFiles(flowPath)},
			},
		}

		if info != nil {
			flow.Created = info.ModTime()
			flow.Modified = info.ModTime()
		}

		discovered[flowID] = flow
	}
}

func mergeFlow(existing, new *Flow) {
	// Update state if new is more advanced
	if stateOrder(new.State) > stateOrder(existing.State) {
		existing.State = new.State
	}

	// Update type if unknown
	if existing.Type == FlowTypeUnknown && new.Type != FlowTypeUnknown {
		existing.Type = new.Type
	}

	// Merge paths
	pathSet := make(map[string]bool)
	for _, p := range existing.Paths {
		pathSet[p] = true
	}
	for _, p := range new.Paths {
		if !pathSet[p] {
			existing.Paths = append(existing.Paths, p)
		}
	}

	// Update PR info
	if existing.IsolationPRUrl == "" && new.IsolationPRUrl != "" {
		existing.IsolationPRUrl = new.IsolationPRUrl
	}
	if new.IsolationPRState != "" {
		existing.IsolationPRState = new.IsolationPRState
	}
	if existing.ReintegrationPRUrl == "" && new.ReintegrationPRUrl != "" {
		existing.ReintegrationPRUrl = new.ReintegrationPRUrl
	}
	if new.ReintegrationPRState != "" {
		existing.ReintegrationPRState = new.ReintegrationPRState
	}

	// Merge stages (prefer new if more complete)
	if len(new.Stages) > len(existing.Stages) {
		existing.Stages = new.Stages
	} else {
		// Update stage statuses
		for i := range existing.Stages {
			for _, ns := range new.Stages {
				if existing.Stages[i].Name == ns.Name {
					if stageStatusOrder(ns.Status) > stageStatusOrder(existing.Stages[i].Status) {
						existing.Stages[i].Status = ns.Status
					}
					if ns.PRUrl != "" {
						existing.Stages[i].PRUrl = ns.PRUrl
					}
					if ns.PRState != "" {
						existing.Stages[i].PRState = ns.PRState
					}
					if ns.Path != "" && len(ns.Files) > 0 {
						existing.Stages[i].Path = ns.Path
						existing.Stages[i].Files = ns.Files
					}
				}
			}
		}
	}

	// Update current stage
	if new.CurrentStage != "" {
		existing.CurrentStage = new.CurrentStage
	}

	// Update instruction
	if existing.Instruction == "" && new.Instruction != "" {
		existing.Instruction = new.Instruction
	}

	// Update target repo
	if existing.TargetRepo == "" && new.TargetRepo != "" {
		existing.TargetRepo = new.TargetRepo
	}

	// Update modified time
	if new.Modified.After(existing.Modified) {
		existing.Modified = new.Modified
	}
}

func stateOrder(s FlowState) int {
	order := map[FlowState]int{
		FlowStatePending:       0,
		FlowStateProcessing:    1,
		FlowStateProposed:      2,
		FlowStateApproved:      3,
		FlowStateExecuting:     4,
		FlowStateIsolated:      5,
		FlowStateRevising:      6,
		FlowStateReintegrating: 7,
		FlowStateMerged:        10,
		FlowStateComplete:      10,
		FlowStateClosed:        9,
		FlowStateFailed:        9,
	}
	return order[s]
}

func stageStatusOrder(s string) int {
	order := map[string]int{
		"pending":   0,
		"skipped":   1,
		"active":    2,
		"completed": 3,
		"failed":    4,
	}
	return order[s]
}

// ── GitHub PR Polling ──────────────────────────────────────────────────────────

func pollPRStatus() {
	lastPRPollMu.Lock()
	if time.Since(lastPRPoll) < githubPollInterval {
		lastPRPollMu.Unlock()
		return
	}
	lastPRPoll = time.Now()
	lastPRPollMu.Unlock()

	// Collect all PR URLs from flows
	flowCacheMu.RLock()
	prURLs := make(map[string]bool)
	for _, flow := range flowCache {
		if flow.IsolationPRUrl != "" {
			prURLs[flow.IsolationPRUrl] = true
		}
		if flow.ReintegrationPRUrl != "" {
			prURLs[flow.ReintegrationPRUrl] = true
		}
	}
	flowCacheMu.RUnlock()

	// Poll each PR
	for url := range prURLs {
		go fetchPRInfo(url)
	}
}

func fetchPRInfo(prURL string) {
	// Extract owner/repo/number from URL
	// Format: https://github.com/owner/repo/pull/123
	parts := strings.Split(prURL, "/")
	if len(parts) < 7 {
		return
	}
	owner := parts[len(parts)-4]
	repo := parts[len(parts)-3]
	number := parts[len(parts)-1]

	// Use gh CLI to get PR info
	cmd := exec.Command("gh", "pr", "view", number, "--repo", fmt.Sprintf("%s/%s", owner, repo), "--json", "state,title,updatedAt,comments")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to fetch PR %s: %v", prURL, err)
		return
	}

	var prData struct {
		State     string `json:"state"`
		Title     string `json:"title"`
		UpdatedAt string `json:"updatedAt"`
		Comments  []struct {
			Body      string `json:"body"`
			CreatedAt string `json:"createdAt"`
			Author    struct {
				Login string `json:"login"`
			} `json:"author"`
		} `json:"comments"`
	}

	if err := json.Unmarshal(output, &prData); err != nil {
		return
	}

	prCacheMu.Lock()
	prCache[prURL] = &PRInfo{
		URL:   prURL,
		State: strings.ToLower(prData.State),
		Title: prData.Title,
	}
	prCacheMu.Unlock()

	// Update flows with this PR
	updateFlowsWithPR(prURL, strings.ToLower(prData.State))
}

func updateFlowsWithPR(prURL, state string) {
	flowCacheMu.Lock()
	defer flowCacheMu.Unlock()

	for _, flow := range flowCache {
		updated := false

		if flow.IsolationPRUrl == prURL && flow.IsolationPRState != state {
			flow.IsolationPRState = state
			updated = true

			// Update flow state based on PR state
			switch state {
			case "merged":
				if flow.Type == FlowTypeRepoIsolation {
					flow.State = FlowStateReintegrating
				} else {
					flow.State = FlowStateMerged
				}
			case "closed":
				flow.State = FlowStateClosed
			}
		}

		if flow.ReintegrationPRUrl == prURL && flow.ReintegrationPRState != state {
			flow.ReintegrationPRState = state
			updated = true

			switch state {
			case "merged":
				flow.State = FlowStateMerged
			case "closed":
				flow.State = FlowStateClosed
			}
		}

		if updated {
			broadcastFlowUpdate(flow.ID, flow.State, flow.CurrentStage)
		}
	}
}

func startPRPoller() {
	ticker := time.NewTicker(githubPollInterval)
	go func() {
		for range ticker.C {
			pollPRStatus()
		}
	}()
}

// ── File Watcher ──────────────────────────────────────────────────────────────

func startWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("watcher init error: %v", err)
		return
	}

	watchDirs := []string{
		slopspacesRoot,
		filepath.Join(slopspacesRoot, "heuristic", "pending"),
		filepath.Join(slopspacesRoot, "heuristic", "processed"),
		filepath.Join(slopspacesRoot, "input", "any"),
		filepath.Join(slopspacesRoot, "output"),
		filepath.Join(slopspacesRoot, "output", "content"),
		filepath.Join(slopspacesRoot, "working"),
		filepath.Join(slopspacesRoot, "dispatcher", "live", "flows"),
		filepath.Join(slopspacesRoot, "dispatcher", "live", "flows", "repo-isolation"),
		filepath.Join(slopspacesRoot, "dispatcher", "live", "flows", "in-repo"),
		filepath.Join(slopspacesRoot, "dispatcher", "live", "flows", "direct"),
	}

	addDir := func(path string) {
		if err := watcher.Add(path); err != nil {
			// Ignore errors for non-existent dirs
			if !os.IsNotExist(err) {
				log.Printf("watch add %s: %v", path, err)
			}
		}
	}

	for _, d := range watchDirs {
		addDir(d)
		// Watch existing subdirectories
		if entries, err := os.ReadDir(d); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					addDir(filepath.Join(d, e.Name()))
				}
			}
		}
	}

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
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						addDir(event.Name)
					}
				case event.Has(fsnotify.Remove):
					op = "remove"
				case event.Has(fsnotify.Rename):
					op = "rename"
				}

				rel, _ := filepath.Rel(slopspacesRoot, event.Name)
				broadcastFileEvent(rel, op)

				// Trigger flow refresh on significant events
				if op == "create" || op == "remove" {
					go func() {
						discoverFlows()
						// Broadcast full flow list update
						hub.broadcast(WSEvent{
							Type: "flows_updated",
							Time: time.Now().UTC().Format(time.RFC3339),
						})
					}()
				}

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

func handleFlows(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")

	flows := discoverFlows()
	json.NewEncoder(w).Encode(flows)
}

func handleFlow(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	id := strings.TrimPrefix(r.URL.Path, "/api/flows/")

	flowCacheMu.RLock()
	flow, ok := flowCache[id]
	flowCacheMu.RUnlock()

	if !ok {
		// Try to find it
		flows := discoverFlows()
		for _, f := range flows {
			if f.ID == id {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(f)
				return
			}
		}
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flow)
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

func handlePollPR(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	// Force a PR poll
	go pollPRStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "polling"})
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// ── Utilities ──────────────────────────────────────────────────────────────────

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

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	// Remove markdown headers
	lines := strings.Split(s, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	s = strings.Join(cleaned, " ")
	if len(s) > maxLen {
		return s[:maxLen] + "…"
	}
	return s
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", handleWS)
	mux.HandleFunc("/api/flows", handleFlows)
	mux.HandleFunc("/api/flows/", handleFlow)
	mux.HandleFunc("/api/file", handleFileContent)
	mux.HandleFunc("/api/poll-pr", handlePollPR)

	sub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// Start background services
	go startWatcher()
	go startPRPoller()

	// Initial flow discovery
	discoverFlows()

	log.Printf("Agent Viewer listening on http://localhost%s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}
