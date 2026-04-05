// Package filestory provides file operation logging with tree-like visualization.
// When FILE_STORY_PATH is set, agents log all file operations (create, modify, read)
// with timestamps, actors, checksums, and tree structure.
package filestory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Operation types for file actions
const (
	OpCreate  = "create"
	OpModify  = "modify"
	OpRead    = "read"
	OpDelete  = "delete"
	OpListDir = "listdir"
	OpCopyIn  = "copy-in"  // Copy into workspace
	OpCopyOut = "copy-out" // Copy out of workspace
)

// maxDirEntries is the threshold for abbreviating directory listings
const maxDirEntries = 8

// Logger handles file story logging to the configured path
type Logger struct {
	mu       sync.Mutex
	path     string
	actor    string
	enabled  bool
	file     *os.File
}

// NewLogger creates a new file story logger.
// If FILE_STORY_PATH is not set or empty, logging is disabled (no-op).
func NewLogger(actor string) *Logger {
	path := os.Getenv("FILE_STORY_PATH")
	return &Logger{
		path:    path,
		actor:   actor,
		enabled: path != "",
	}
}

// NewLoggerWithPath creates a logger with an explicit path (for testing)
func NewLoggerWithPath(actor, path string) *Logger {
	return &Logger{
		path:    path,
		actor:   actor,
		enabled: path != "",
	}
}

// Enabled returns true if file story logging is active
func (l *Logger) Enabled() bool {
	return l.enabled
}

// Close closes the underlying file if open
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// ensureOpen opens the log file for appending if not already open
func (l *Logger) ensureOpen() error {
	if l.file != nil {
		return nil
	}

	// Ensure parent directory exists
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file story log: %w", err)
	}
	l.file = f
	return nil
}

// LogFile logs a single file operation with its checksum
func (l *Logger) LogFile(op, path string) error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureOpen(); err != nil {
		return err
	}

	checksum := computeChecksum(path)
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Format: [timestamp] actor/function: path (checksum)
	line := fmt.Sprintf("[%s] %s/%s: %s (%s)\n",
		timestamp, l.actor, op, path, checksum)

	_, err := l.file.WriteString(line)
	return err
}

// LogFileWithChecksum logs a file operation with a pre-computed checksum
func (l *Logger) LogFileWithChecksum(op, path, checksum string) error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureOpen(); err != nil {
		return err
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s/%s: %s (%s)\n",
		timestamp, l.actor, op, path, checksum)

	_, err := l.file.WriteString(line)
	return err
}

// LogTree logs a directory tree with checksums for all files.
// If a directory contains more than 8 entries, it's abbreviated.
func (l *Logger) LogTree(op, rootPath string) error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureOpen(); err != nil {
		return err
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Write header
	header := fmt.Sprintf("[%s] %s/%s: %s\n", timestamp, l.actor, op, rootPath)
	if _, err := l.file.WriteString(header); err != nil {
		return err
	}

	// Build and write tree
	tree, err := buildTree(rootPath, "")
	if err != nil {
		// Log the error but don't fail
		errLine := fmt.Sprintf("  (error building tree: %v)\n", err)
		l.file.WriteString(errLine)
		return nil
	}

	_, err = l.file.WriteString(tree)
	return err
}

// LogTreeDiff logs changes between two directory states (before/after)
func (l *Logger) LogTreeDiff(op, rootPath string, beforeChecksums map[string]string) error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureOpen(); err != nil {
		return err
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Collect current state
	afterChecksums := make(map[string]string)
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(rootPath, path)
		afterChecksums[relPath] = computeChecksum(path)
		return nil
	})

	// Find differences
	var created, modified, deleted []string

	for path, afterSum := range afterChecksums {
		beforeSum, existed := beforeChecksums[path]
		if !existed {
			created = append(created, path)
		} else if beforeSum != afterSum {
			modified = append(modified, path)
		}
	}

	for path := range beforeChecksums {
		if _, exists := afterChecksums[path]; !exists {
			deleted = append(deleted, path)
		}
	}

	// Sort for consistent output
	sort.Strings(created)
	sort.Strings(modified)
	sort.Strings(deleted)

	// Write header
	header := fmt.Sprintf("[%s] %s/%s: %s (diff)\n", timestamp, l.actor, op, rootPath)
	if _, err := l.file.WriteString(header); err != nil {
		return err
	}

	// Write changes
	if len(created) > 0 {
		l.file.WriteString("  + created:\n")
		for _, p := range created {
			l.file.WriteString(fmt.Sprintf("    %s (%s)\n", p, afterChecksums[p]))
		}
	}

	if len(modified) > 0 {
		l.file.WriteString("  ~ modified:\n")
		for _, p := range modified {
			l.file.WriteString(fmt.Sprintf("    %s (%s -> %s)\n", p, beforeChecksums[p][:8], afterChecksums[p][:8]))
		}
	}

	if len(deleted) > 0 {
		l.file.WriteString("  - deleted:\n")
		for _, p := range deleted {
			l.file.WriteString(fmt.Sprintf("    %s\n", p))
		}
	}

	if len(created) == 0 && len(modified) == 0 && len(deleted) == 0 {
		l.file.WriteString("  (no changes)\n")
	}

	return nil
}

// SnapshotChecksums captures the current state of a directory for later diffing
func SnapshotChecksums(rootPath string) map[string]string {
	checksums := make(map[string]string)
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(rootPath, path)
		checksums[relPath] = computeChecksum(path)
		return nil
	})
	return checksums
}

// buildTree constructs a tree-like string representation of a directory
func buildTree(rootPath, prefix string) (string, error) {
	var sb strings.Builder

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return "", err
	}

	// Separate files and directories
	var files, dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}

	// Check if we need to abbreviate
	totalEntries := len(files) + len(dirs)
	abbreviated := totalEntries > maxDirEntries

	if abbreviated {
		// Show summary instead of full listing
		sb.WriteString(fmt.Sprintf("%s  [%d files, %d dirs - abbreviated]\n", prefix, len(files), len(dirs)))

		// Show first few files with checksums
		showCount := 3
		if len(files) < showCount {
			showCount = len(files)
		}
		for i := 0; i < showCount; i++ {
			filePath := filepath.Join(rootPath, files[i].Name())
			checksum := computeChecksum(filePath)
			sb.WriteString(fmt.Sprintf("%s  ├── %s (%s)\n", prefix, files[i].Name(), checksum))
		}
		if len(files) > showCount {
			sb.WriteString(fmt.Sprintf("%s  ├── ... and %d more files\n", prefix, len(files)-showCount))
		}

		// Show first few directories
		showDirs := 2
		if len(dirs) < showDirs {
			showDirs = len(dirs)
		}
		for i := 0; i < showDirs; i++ {
			sb.WriteString(fmt.Sprintf("%s  ├── %s/\n", prefix, dirs[i].Name()))
		}
		if len(dirs) > showDirs {
			sb.WriteString(fmt.Sprintf("%s  └── ... and %d more directories\n", prefix, len(dirs)-showDirs))
		}

		return sb.String(), nil
	}

	// Full listing
	allEntries := append(files, dirs...)
	for i, entry := range allEntries {
		isLast := i == len(allEntries)-1
		connector := "├──"
		if isLast {
			connector = "└──"
		}

		entryPath := filepath.Join(rootPath, entry.Name())

		if entry.IsDir() {
			sb.WriteString(fmt.Sprintf("%s  %s %s/\n", prefix, connector, entry.Name()))

			// Recurse into directory
			newPrefix := prefix + "  │"
			if isLast {
				newPrefix = prefix + "   "
			}
			subtree, err := buildTree(entryPath, newPrefix)
			if err == nil {
				sb.WriteString(subtree)
			}
		} else {
			checksum := computeChecksum(entryPath)
			sb.WriteString(fmt.Sprintf("%s  %s %s (%s)\n", prefix, connector, entry.Name(), checksum))
		}
	}

	return sb.String(), nil
}

// computeChecksum calculates SHA256 of a file, returning first 8 hex chars
func computeChecksum(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "????????"
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "????????"
	}

	return hex.EncodeToString(h.Sum(nil))[:8]
}

// ComputeChecksum is exported for use by callers who need checksums
func ComputeChecksum(path string) string {
	return computeChecksum(path)
}
