package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/je-sidestuff/AI-sandboxing/pkg/filestory"
)

// Default paths
const (
	defaultHeuristicDir = "/workspaces/slopspaces/heuristic"
	defaultRequestDir   = "/workspaces/slopspaces/input/any" // Output to agent-dispatch input dir
	defaultRecordsDir   = "/workspaces/slopspaces/agent-records/"
	checkInterval       = 10 * time.Second
	defaultAgent        = "claude"

	// Agent workspace - the restricted environment where the AI runs
	// In host mode: files are copied in/out; AI runs as restricted user
	// In container mode (future): directories are mounted; AI runs in container
	agentWorkspaceRoot = "/agent/heuristic-request"
	defaultAgentUser   = "heuristic-request"

	// Default read-space: a cached copy of AI-sandboxing repo (minus .git)
	// This provides agents with visibility into the codebase
	defaultReadSpacePath   = "/workspaces/slopspaces/read-spaces/default"
	defaultReadSpaceSource = "/workspaces/workspace/sandbox/AI-sandboxing"
)

// Available agents (must match invoke-agent.sh agent definitions)
var availableAgents = []string{"copilot", "gemini", "claude", "opencode", "codex", "grok"}

// Exponential backoff levels for logging inactivity
var backoffLevels = []time.Duration{
	30 * time.Second,
	5 * time.Minute,
	1 * time.Hour,
	24 * time.Hour,
}

// HeuristicUnit represents a discovered heuristic input
type HeuristicUnit struct {
	Path       string
	ID         string
	Content    string
	FolderPath string
	Agent      string // Optional agent override
	Model      string // Optional model override (for agents that support it)
}

// HeuristicData is the format for HEURISTIC.json
type HeuristicData struct {
	Message string `json:"message"`
	Agent   string `json:"agent,omitempty"` // Optional agent override
	Model   string `json:"model,omitempty"` // Optional model override (for agents that support it)
}

// ExtractedFile represents a file extracted from agent output
type ExtractedFile struct {
	Filename string
	Content  string
}

// AgentWorkspace represents the prepared workspace for an agent invocation
// The workspace is located at /agent/heuristic-request/ and contains:
//   - read/default/  : Copy of AI-sandboxing repo (excluding .git) - the default read-space
//   - read/workunit/ : Copy of work unit content (the heuristic folder)
//   - write/primary/ : Agent's working directory where it can create/modify files
type AgentWorkspace struct {
	RootPath     string // /agent/heuristic-request
	ReadDefault  string // /agent/heuristic-request/read/default (AI-sandboxing repo)
	ReadWorkunit string // /agent/heuristic-request/read/workunit (work unit content)
	WritePrimary string // /agent/heuristic-request/write/primary
}

// HeuristicWatcher manages the heuristic watch loop
type HeuristicWatcher struct {
	watcherID      string
	heuristicDir   string
	requestDir     string
	recordsDir     string
	currentAgent   string
	lastActivity   time.Time
	backoffIndex   int
	nextBackoffLog time.Time
	agentUser      string // OS user to run agent as (for host mode isolation)
	agentUID       int    // UID of agent user (0 if not found/not using)
	agentGID       int    // GID of agent user (0 if not found/not using)
	fileStory      *filestory.Logger // File operation logger (nil if FILE_STORY_PATH not set)
}

// NewHeuristicWatcher creates a new watcher instance
func NewHeuristicWatcher() *HeuristicWatcher {
	heuristicDir := os.Getenv("HEURISTIC_DIR")
	if heuristicDir == "" {
		heuristicDir = defaultHeuristicDir
	}

	requestDir := os.Getenv("REQUEST_DIR")
	if requestDir == "" {
		requestDir = defaultRequestDir
	}

	recordsDir := os.Getenv("RECORDS_DIR")
	if recordsDir == "" {
		recordsDir = defaultRecordsDir
	}

	// Support both AGENT_NAME (new) and AGENT_PRESET (deprecated) for backwards compatibility
	currentAgent := os.Getenv("AGENT_NAME")
	if currentAgent == "" {
		currentAgent = os.Getenv("AGENT_PRESET")
	}
	if currentAgent == "" {
		currentAgent = defaultAgent
	}

	// Agent user for host mode isolation
	// Set AGENT_USER to override the default restricted user
	// Set AGENT_USER="" to disable user isolation (run as current user)
	agentUser := os.Getenv("AGENT_USER")
	if agentUser == "" && os.Getenv("AGENT_USER") == "" {
		// Not explicitly set, use default
		agentUser = defaultAgentUser
	}

	var agentUID, agentGID int
	if agentUser != "" {
		if u, err := user.Lookup(agentUser); err == nil {
			agentUID, _ = strconv.Atoi(u.Uid)
			agentGID, _ = strconv.Atoi(u.Gid)
		}
		// If user not found, UID/GID remain 0 - handled at runtime
	}

	now := time.Now()
	watcherID := uuid.New().String()[:8]

	// Initialize file story logger (uses FILE_STORY_PATH env var)
	fileStoryLogger := filestory.NewLogger("heuristic-request")

	return &HeuristicWatcher{
		watcherID:      watcherID,
		heuristicDir:   heuristicDir,
		requestDir:     requestDir,
		recordsDir:     recordsDir,
		currentAgent:   currentAgent,
		lastActivity:   now,
		backoffIndex:   0,
		nextBackoffLog: now.Add(backoffLevels[0]),
		agentUser:      agentUser,
		agentUID:       agentUID,
		agentGID:       agentGID,
		fileStory:      fileStoryLogger,
	}
}

// ensureDirectories creates necessary directories
func (w *HeuristicWatcher) ensureDirectories() error {
	dirs := []string{
		filepath.Join(w.heuristicDir, "pending"),
		filepath.Join(w.heuristicDir, "processed"),
		w.requestDir,
		filepath.Join(w.recordsDir, "heuristic"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// ensureDefaultReadSpace ensures the default read-space exists in slopspaces.
// The default read-space is a copy of the AI-sandboxing repository with .git removed.
// This is created once and reused across invocations (refreshed if source is newer).
func (w *HeuristicWatcher) ensureDefaultReadSpace() error {
	// Check if default read-space directory exists
	if _, err := os.Stat(defaultReadSpacePath); os.IsNotExist(err) {
		log.Printf("[%s] Creating default read-space at %s", w.watcherID, defaultReadSpacePath)

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(defaultReadSpacePath), 0755); err != nil {
			return fmt.Errorf("failed to create read-spaces directory: %w", err)
		}

		// Copy AI-sandboxing repo to default read-space (excluding .git)
		if err := os.MkdirAll(defaultReadSpacePath, 0755); err != nil {
			return fmt.Errorf("failed to create default read-space: %w", err)
		}

		if err := copyDirContents(defaultReadSpaceSource, defaultReadSpacePath, true); err != nil {
			return fmt.Errorf("failed to copy AI-sandboxing to default read-space: %w", err)
		}

		log.Printf("[%s] Default read-space created successfully", w.watcherID)
	}

	return nil
}

// prepareWorkspace creates the agent workspace at /agent/heuristic-request/
// This clears any previous content and sets up fresh read/write spaces.
// In host mode, the heuristic-request binary (running as a privileged user) prepares this
// workspace, then runs the AI as a restricted user that only has access to this directory.
//
// The workspace contains:
//   - read/default/  : Copy of AI-sandboxing repo (the default read-space)
//   - read/workunit/ : Copy of work unit content (the heuristic folder)
//   - write/primary/ : Agent's working directory
func (w *HeuristicWatcher) prepareWorkspace(sourcePath string) (*AgentWorkspace, error) {
	workspace := &AgentWorkspace{
		RootPath:     agentWorkspaceRoot,
		ReadDefault:  filepath.Join(agentWorkspaceRoot, "read", "default"),
		ReadWorkunit: filepath.Join(agentWorkspaceRoot, "read", "workunit"),
		WritePrimary: filepath.Join(agentWorkspaceRoot, "write", "primary"),
	}

	// Clean up any existing workspace content
	// We clean contents rather than removing directories themselves, because:
	// - The parent read/ and write/ directories have special permissions (1777)
	// - This allows any user to clean up workspace files from previous runs
	// - The AI may have created files owned by the heuristic-request user
	readDir := filepath.Join(agentWorkspaceRoot, "read")
	writeDir := filepath.Join(agentWorkspaceRoot, "write")

	if err := cleanDirContents(readDir); err != nil {
		return nil, fmt.Errorf("failed to clean read space: %w", err)
	}
	if err := cleanDirContents(writeDir); err != nil {
		return nil, fmt.Errorf("failed to clean write space: %w", err)
	}

	// Create workspace directories
	if err := os.MkdirAll(workspace.ReadDefault, 0755); err != nil {
		return nil, fmt.Errorf("failed to create read/default: %w", err)
	}
	if err := os.MkdirAll(workspace.ReadWorkunit, 0755); err != nil {
		return nil, fmt.Errorf("failed to create read/workunit: %w", err)
	}
	if err := os.MkdirAll(workspace.WritePrimary, 0755); err != nil {
		return nil, fmt.Errorf("failed to create write/primary: %w", err)
	}

	// Copy the default read-space (AI-sandboxing repo) from slopspaces cache
	// This provides the agent with visibility into the codebase
	if err := copyDirContents(defaultReadSpacePath, workspace.ReadDefault, true); err != nil {
		return nil, fmt.Errorf("failed to copy default read-space: %w", err)
	}
	w.fileStory.LogTree(filestory.OpCopyIn, workspace.ReadDefault)

	// Copy work unit content to read/workunit (excluding .git)
	if err := copyDirContents(sourcePath, workspace.ReadWorkunit, true); err != nil {
		return nil, fmt.Errorf("failed to copy work unit to read space: %w", err)
	}
	w.fileStory.LogTree(filestory.OpCopyIn, workspace.ReadWorkunit)

	// Set ownership of write space to agent user (if running as root)
	// The read space stays root-owned so the agent can read but not write.
	// The write space is chowned to the agent user so they can write.
	if os.Getuid() == 0 && w.agentUID != 0 {
		// Only chown the write space to the agent user
		if err := chownRecursive(workspace.WritePrimary, w.agentUID, w.agentGID); err != nil {
			return nil, fmt.Errorf("failed to set write space ownership: %w", err)
		}
	}

	return workspace, nil
}

// cleanupWorkspace removes the workspace content after execution
func (w *HeuristicWatcher) cleanupWorkspace() {
	// Clean up workspace content but keep the read/ and write/ directories
	// (they have special permissions set up by 'make setup-dirs')
	cleanDirContents(filepath.Join(agentWorkspaceRoot, "read"))
	cleanDirContents(filepath.Join(agentWorkspaceRoot, "write"))
}

// cleanDirContents removes all contents of a directory but keeps the directory itself
// This is used instead of os.RemoveAll because the parent directories have special
// permissions (1777) that we want to preserve across runs
func cleanDirContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, nothing to clean
		}
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %s: %w", path, err)
		}
	}

	return nil
}

// copyDirContents copies directory contents from src to dst
// If excludeGit is true, .git directories are skipped
func copyDirContents(src, dst string, excludeGit bool) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Skip .git directories if requested
		if excludeGit && entry.Name() == ".git" {
			continue
		}

		if entry.IsDir() {
			// Create directory and copy contents recursively
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyDirContents(srcPath, dstPath, excludeGit); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// chownRecursive changes ownership of a directory tree
func chownRecursive(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(name, uid, gid)
	})
}

// copyWriteSpaceOutput copies files from write/primary back to the destination folder
// This extracts the agent's output after execution
func (w *HeuristicWatcher) copyWriteSpaceOutput(workspace *AgentWorkspace, destPath string) error {
	// Check if write space exists and has content
	entries, err := os.ReadDir(workspace.WritePrimary)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No output to copy
		}
		return err
	}

	// Log the agent's output before copying
	w.fileStory.LogTree(filestory.OpCopyOut, workspace.WritePrimary)

	for _, entry := range entries {
		srcPath := filepath.Join(workspace.WritePrimary, entry.Name())
		dstPath := filepath.Join(destPath, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyDirContents(srcPath, dstPath, false); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// findInvokeScript locates the invoke-agent.sh script
func findInvokeScript() string {
	// Check same directory as executable
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "invoke-agent.sh")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Check current working directory
	cwd, err := os.Getwd()
	if err == nil {
		candidate := filepath.Join(cwd, "invoke-agent.sh")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Check ambiguous-agent directory relative to current directory
	if err == nil {
		candidate := filepath.Join(cwd, "ambiguous-agent", "invoke-agent.sh")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Check parent directory's ambiguous-agent
	if err == nil {
		candidate := filepath.Join(filepath.Dir(cwd), "ambiguous-agent", "invoke-agent.sh")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Check PATH
	path, err := exec.LookPath("invoke-agent.sh")
	if err == nil {
		return path
	}

	return ""
}

// isValidAgent checks if the agent name is in the available agents list
func isValidAgent(name string) bool {
	for _, a := range availableAgents {
		if a == name {
			return true
		}
	}
	return false
}

// checkForHeuristicUnits scans the heuristic directory for HEURISTIC.md files
func (w *HeuristicWatcher) checkForHeuristicUnits() ([]HeuristicUnit, error) {
	pendingDir := filepath.Join(w.heuristicDir, "pending")
	entries, err := os.ReadDir(pendingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Log directory scan (uses abbreviation if >8 entries)
	if len(entries) > 0 {
		w.fileStory.LogTree(filestory.OpListDir, pendingDir)
	}

	var units []HeuristicUnit
	for _, entry := range entries {
		if entry.IsDir() {
			folderPath := filepath.Join(pendingDir, entry.Name())

			// Skip if already being processed
			processingMD := filepath.Join(folderPath, "PROCESSING.md")
			if _, err := os.Stat(processingMD); err == nil {
				continue
			}

			// Check for HEURISTIC.json first
			heuristicJSON := filepath.Join(folderPath, "HEURISTIC.json")
			if _, err := os.Stat(heuristicJSON); err == nil {
				content, err := os.ReadFile(heuristicJSON)
				if err != nil {
					log.Printf("[%s] Warning: failed to read HEURISTIC.json in %s: %v", w.watcherID, entry.Name(), err)
					continue
				}
				w.fileStory.LogFile(filestory.OpRead, heuristicJSON)

				var data HeuristicData
				if err := json.Unmarshal(content, &data); err != nil {
					log.Printf("[%s] Warning: failed to parse HEURISTIC.json in %s: %v", w.watcherID, entry.Name(), err)
					continue
				}

				units = append(units, HeuristicUnit{
					Path:       heuristicJSON,
					ID:         entry.Name(),
					Content:    data.Message,
					Agent:      data.Agent,
					Model:      data.Model,
					FolderPath: folderPath,
				})
				continue // Skip to next folder
			}

			// Fallback to HEURISTIC.md
			heuristicMD := filepath.Join(folderPath, "HEURISTIC.md")
			if _, err := os.Stat(heuristicMD); err == nil {
				content, err := os.ReadFile(heuristicMD)
				if err != nil {
					log.Printf("[%s] Warning: failed to read HEURISTIC.md in %s: %v", w.watcherID, entry.Name(), err)
					continue
				}
				w.fileStory.LogFile(filestory.OpRead, heuristicMD)

				units = append(units, HeuristicUnit{
					Path:       heuristicMD,
					ID:         entry.Name(),
					Content:    string(content),
					FolderPath: folderPath,
					Agent:      "", // No agent override from MD
					Model:      "", // No model override from MD
				})
			}
		}
	}

	return units, nil
}

// buildHeuristicPrompt creates the prompt for the agent
func (w *HeuristicWatcher) buildHeuristicPrompt(heuristicContent string) string {
	return fmt.Sprintf(`You are a heuristic processor. Based on the following heuristic input, decide what action should be taken.

## Heuristic Input

%s

## Your Task

Analyze the heuristic input and determine the appropriate execution pattern. All dispatches go through an approval flow automatically before execution.

## Dispatch Types (Execution Patterns)

### repo-isolation
For tasks that modify a target repository. Work is done in an isolated clone, then submitted as a PR back to the target. Use when:
- Adding features to a specific repo (e.g., "add email handling to agent-events")
- Making changes to external repositories
- Tasks that should be reviewed via PR before merging

%sjson DISPATCH.json
{
  "type": "repo-isolation",
  "target_repo": "owner/repo-name",
  "instruction": "What changes to make",
  "mode": "execute"
}
%s

### in-repo
For tasks that work directly on a branch in the target repo (less isolation). Use when:
- Quick fixes that don't need full isolation overhead
- Working on the current repository

%sjson DISPATCH.json
{
  "type": "in-repo",
  "target_repo": "owner/repo-name",
  "instruction": "What changes to make",
  "mode": "execute"
}
%s

### direct
For simple local tasks that don't modify external repos. Use when:
- Local tasks, reports, or analysis
- Tasks that don't create PRs

%sjson DISPATCH.json
{
  "type": "direct",
  "instruction": "The task to perform",
  "mode": "execute"
}
%s

### sequence-to-new-repo
For multi-step tasks that should be executed as a series of timed steps in a NEW repository. Creates a fresh repo and executes steps sequentially with configurable delays between them. Use when:
- Writing multi-chapter documentation or tutorials
- Implementing features in phases
- Tasks that naturally decompose into ordered steps
- Creating content that builds on previous steps

%sjson DISPATCH.json
{
  "type": "sequence-to-new-repo",
  "sequence_repo_name": "descriptive-repo-name",
  "instruction": "Initial setup instruction (creates repo structure)",
  "sequence_commands": [
    "Step 1: First action to perform",
    "Step 2: Second action (builds on step 1)",
    "Step 3: Third action (builds on previous steps)"
  ],
  "sequence_minutes_between": 20,
  "mode": "execute"
}
%s

**Guidelines for sequence-to-new-repo:**
- **REQUIRED: sequence_repo_name** - Always specify a descriptive repo name. Do NOT embed the repo name in the instruction text; that causes duplicate repo creation.
- Use when the request mentions: "chapters", "phases", "series", "step-by-step", "tutorial", "guide", "multi-part", "staged"
- First instruction creates the repo structure (README, folder layout) - do NOT include "create repo X" in the instruction since the dispatcher handles repo creation
- Each sequence_command is a discrete, self-contained step
- Steps should be logically ordered - later steps may reference earlier work
- 20 minutes between steps is the default; adjust based on complexity (5-60 minutes typical)
- Keep steps focused: 3-10 steps is typical, max 80

## Alternate Output: INSTRUCTION (RARELY USED)

IMPORTANT: Only use INSTRUCTION.json for trivial read-only queries that need NO approval and NO file modifications.
Examples: "what time is it?", "explain what X means", "list available agents"

For ANY task that involves:
- Creating, modifying, or deleting files
- Making changes to code or repositories
- Bug fixes, features, or refactoring
- Analysis that produces output files

→ USE DISPATCH.json instead (with type "direct" for local tasks)

%sjson INSTRUCTION.json
{
  "instruction": "Your trivial read-only query here",
  "mode": "prompt"
}
%s

## Guidelines

- **DISPATCH is the default** - almost all tasks should use DISPATCH.json
- **repo-isolation**: Default choice for modifying repos. Phrases like "add a feature to X", "implement Y in Z repo"
- **in-repo**: Use for quick fixes where isolation overhead isn't warranted
- **direct**: Use for local tasks, reports, analysis, bug fixes - anything that creates or modifies files locally
- **sequence-to-new-repo**: Use for multi-part content creation (tutorials, documentation, phased implementations)
- **INSTRUCTION**: ONLY for trivial read-only queries with no file operations
- Infer repo owner when not specified: "agent-events" → "je-sidestuff/agent-events"
- All dispatches go through approval automatically - you don't need to specify approval

**Sequence detection keywords:** chapters, phases, series, step-by-step, tutorial, guide, multi-part, staged, phased

Output exactly ONE file wrapped in triple backticks with the filename.
`, heuristicContent, "```", "```", "```", "```", "```", "```", "```", "```", "```", "```")
}

// extractFilesFromOutput parses the agent output to extract files from code blocks
func extractFilesFromOutput(output string) []ExtractedFile {
	var files []ExtractedFile

	// Pattern to match code blocks with filename: ```<lang> <filename>\n...\n```
	// Supports variations like:
	// ```markdown DISPATCH.md
	// ```json INSTRUCTION.json
	pattern := regexp.MustCompile("(?s)```(?:markdown|json|md)?\\s*(DISPATCH\\.(?:md|json)|INSTRUCTION\\.(?:md|json))\\s*\n(.*?)```")

	matches := pattern.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			filename := strings.TrimSpace(match[1])
			content := match[2]

			// Only accept valid filenames
			if filename == "DISPATCH.md" || filename == "DISPATCH.json" ||
				filename == "INSTRUCTION.md" || filename == "INSTRUCTION.json" {
				files = append(files, ExtractedFile{
					Filename: filename,
					Content:  content,
				})
			}
		}
	}

	return files
}

// processHeuristicUnit handles a single heuristic input
func (w *HeuristicWatcher) processHeuristicUnit(unit HeuristicUnit) error {
	log.Printf("[%s] Processing heuristic unit: %s", w.watcherID, unit.ID)

	startTime := time.Now()

	// Create PROCESSING.md to mark we're working on it
	processingMD := filepath.Join(unit.FolderPath, "PROCESSING.md")
	processingContent := fmt.Sprintf("# Processing\n\nWatcher ID: %s\nStarted: %s\nAgent: %s\n",
		w.watcherID, startTime.Format(time.RFC3339), w.currentAgent)
	if err := os.WriteFile(processingMD, []byte(processingContent), 0644); err != nil {
		return fmt.Errorf("failed to create PROCESSING.md: %w", err)
	}

	// Build the prompt
	prompt := w.buildHeuristicPrompt(unit.Content)

	// Execute the agent in prompt-only mode
	output, exitCode, err := w.executeAgent(unit.FolderPath, prompt, unit.Agent, unit.Model)
	if err != nil {
		w.markHeuristicFailed(unit, err, startTime)
		return err
	}

	// Extract files from output
	extractedFiles := extractFilesFromOutput(output)
	log.Printf("[%s] Extracted %d files from agent output (exit code: %d)", w.watcherID, len(extractedFiles), exitCode)
	if len(extractedFiles) == 0 {
		err := fmt.Errorf("no valid files extracted from agent output (exit code: %d, output length: %d bytes)", exitCode, len(output))
		w.markHeuristicFailed(unit, err, startTime)
		return err
	}

	// Create request folder in REQUEST_DIR
	requestID := fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02_15-04-05"), unit.ID)
	requestFolder := filepath.Join(w.requestDir, requestID)
	if err := os.MkdirAll(requestFolder, 0755); err != nil {
		w.markHeuristicFailed(unit, err, startTime)
		return fmt.Errorf("failed to create request folder: %w", err)
	}

	// Write extracted files to request folder
	for _, file := range extractedFiles {
		filePath := filepath.Join(requestFolder, file.Filename)
		if err := os.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
			log.Printf("[%s] Warning: failed to write %s: %v", w.watcherID, file.Filename, err)
		} else {
			log.Printf("[%s] Extracted %s to %s", w.watcherID, file.Filename, requestFolder)
			// Log the extracted file to file story
			w.fileStory.LogFile(filestory.OpCreate, filePath)
		}
	}

	// Copy original heuristic for reference
	heuristicCopy := filepath.Join(requestFolder, "HEURISTIC_SOURCE.md")
	if err := os.WriteFile(heuristicCopy, []byte(unit.Content), 0644); err != nil {
		log.Printf("[%s] Warning: failed to copy heuristic source: %v", w.watcherID, err)
	} else {
		w.fileStory.LogFile(filestory.OpCreate, heuristicCopy)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Move to processed directory
	processedDir := filepath.Join(w.heuristicDir, "processed")
	destPath := filepath.Join(processedDir, unit.ID)
	if err := os.Rename(unit.FolderPath, destPath); err != nil {
		log.Printf("[%s] Warning: failed to move to processed: %v", w.watcherID, err)
	} else {
		// Create PROCESSED.md in the destination
		processedMD := filepath.Join(destPath, "PROCESSED.md")
		processedContent := fmt.Sprintf("# Processed\n\nWatcher ID: %s\nStarted: %s\nCompleted: %s\nDuration: %s\nAgent: %s\nExit Code: %d\nRequest ID: %s\nExtracted Files: %d\n",
			w.watcherID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
			duration.Round(time.Millisecond).String(), w.currentAgent, exitCode, requestID, len(extractedFiles))
		if err := os.WriteFile(processedMD, []byte(processedContent), 0644); err != nil {
			log.Printf("Warning: failed to create PROCESSED.md: %v", err)
		}
	}

	// Write record
	w.writeHeuristicRecord(unit, startTime, endTime, exitCode, requestID, len(extractedFiles), nil)

	log.Printf("[%s] Completed heuristic unit: %s (exit: %d, duration: %s, files: %d)",
		w.watcherID, unit.ID, exitCode, duration.Round(time.Millisecond), len(extractedFiles))
	return nil
}

// executeAgent runs the agent in prompt-only mode and captures output
// It prepares the isolated workspace at /agent/heuristic-request/, runs the AI as a
// restricted user (in host mode), and copies output back to the work unit folder.
// agentOverride: if non-empty and valid, use this agent instead of the default
// modelOverride: if non-empty, pass this model to the agent (for agents that support it)
func (w *HeuristicWatcher) executeAgent(folderPath, prompt, agentOverride, modelOverride string) (string, int, error) {
	// Find invoke-agent.sh
	invokeScript := findInvokeScript()
	if invokeScript == "" {
		return "", 1, fmt.Errorf("invoke-agent.sh not found")
	}

	agentToUse := w.currentAgent
	if agentOverride != "" && isValidAgent(agentOverride) {
		agentToUse = agentOverride
	}

	// Prepare the workspace - copy work unit to /agent/heuristic-request/read/default/
	workspace, err := w.prepareWorkspace(folderPath)
	if err != nil {
		return "", 1, fmt.Errorf("failed to prepare workspace: %w", err)
	}
	defer func() {
		// Copy output from write space back to work unit folder
		if copyErr := w.copyWriteSpaceOutput(workspace, folderPath); copyErr != nil {
			log.Printf("[%s] Warning: failed to copy write space output: %v", w.watcherID, copyErr)
		}
		// Clean up workspace
		w.cleanupWorkspace()
	}()

	// Write prompt to the write space so the agent can see it
	promptFile := filepath.Join(workspace.WritePrimary, ".prompt.tmp")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return "", 1, fmt.Errorf("failed to write prompt file: %w", err)
	}
	// Set ownership of prompt file if running as root
	if os.Getuid() == 0 && w.agentUID != 0 {
		os.Chown(promptFile, w.agentUID, w.agentGID)
	}

	// Build command arguments - ALWAYS use prompt mode (-p)
	cmdArgs := []string{"-p", "-a", agentToUse}
	// Add model flag if model override is specified
	if modelOverride != "" {
		cmdArgs = append(cmdArgs, "-m", modelOverride)
	}
	cmdArgs = append(cmdArgs, "-f", promptFile)

	// Create command
	cmd := exec.Command(invokeScript, cmdArgs...)

	// Agent runs from write/primary/ - this is its working directory
	cmd.Dir = workspace.WritePrimary

	// Grant access to the entire agent workspace root (/agent/heuristic-request/)
	// The agent sees this as its world:
	//   /agent/heuristic-request/
	//   ├── read/           (read-only via OS permissions)
	//   │   ├── default/    (AI-sandboxing codebase)
	//   │   └── workunit/   (heuristic work unit files)
	//   └── write/          (writable via OS permissions)
	//       └── primary/    (working directory)
	//
	// OS-level user permissions enforce that read/ is read-only and write/ is writable.
	//
	// Note: We do NOT override HOME here. Claude and other agents need access to their
	// credentials in ~/.claude/ or similar. The working directory (cmd.Dir) is set
	// to the write space, but the agent can still access its normal config files.
	env := append(os.Environ(),
		"AGENT_NAME="+agentToUse,
		"AGENT_PRESET="+agentToUse, // backwards compatibility
		"AGENT_RECORDS_PATH="+w.recordsDir,
		"AGENT_ADD_DIRS="+workspace.RootPath,
	)
	if modelOverride != "" {
		env = append(env, "AGENT_MODEL="+modelOverride)
	}
	cmd.Env = env

	// In host mode, run as restricted user if configured and we're root
	if os.Getuid() == 0 && w.agentUID != 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(w.agentUID),
				Gid: uint32(w.agentGID),
			},
		}
		log.Printf("[%s] Running agent as user %s (uid=%d)", w.watcherID, w.agentUser, w.agentUID)
	} else if w.agentUser != "" && w.agentUID == 0 {
		// User configured but not found - warn but continue as current user
		log.Printf("[%s] Warning: agent user '%s' not found, running as current user", w.watcherID, w.agentUser)
	}

	// Capture output - write to the workspace write space
	outputFile := filepath.Join(workspace.WritePrimary, "agent_output.txt")
	outFile, err := os.Create(outputFile)
	if err != nil {
		return "", 1, fmt.Errorf("failed to create output file: %w", err)
	}
	// Set ownership of output file if running as root
	if os.Getuid() == 0 && w.agentUID != 0 {
		os.Chown(outputFile, w.agentUID, w.agentGID)
	}
	defer outFile.Close()

	// Capture output to file - stdin should NOT be connected since we're using
	// prompt-only mode (-p) which is non-interactive. Connecting os.Stdin can
	// cause hangs when running as a daemon or in environments without a TTY.
	cmd.Stdout = outFile
	cmd.Stderr = outFile
	// cmd.Stdin is intentionally left nil (defaults to /dev/null) for non-interactive mode

	log.Printf("[%s] Invoking agent %s in prompt-only mode", w.watcherID, agentToUse)
	log.Printf("[%s] Workspace: %s", w.watcherID, workspace.RootPath)

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			log.Printf("[%s] Agent exited with code %d", w.watcherID, exitCode)
		} else {
			return "", 1, fmt.Errorf("failed to run agent: %w", err)
		}
	}

	// Read the captured output
	output, err := os.ReadFile(outputFile)
	if err != nil {
		return "", exitCode, fmt.Errorf("failed to read output file: %w", err)
	}

	// Log output info for debugging
	outputLen := len(output)
	log.Printf("[%s] Agent output: %d bytes", w.watcherID, outputLen)
	if outputLen > 0 && outputLen < 500 {
		// Log short outputs in full for debugging
		log.Printf("[%s] Output content: %s", w.watcherID, string(output))
	} else if outputLen >= 500 {
		// Log first 200 chars of longer outputs
		log.Printf("[%s] Output preview: %s...", w.watcherID, string(output[:200]))
	}

	return string(output), exitCode, nil
}

// markHeuristicFailed marks a heuristic unit as failed
func (w *HeuristicWatcher) markHeuristicFailed(unit HeuristicUnit, processErr error, startTime time.Time) {
	endTime := time.Now()

	// Create FAILED.md
	failedMD := filepath.Join(unit.FolderPath, "FAILED.md")
	failedContent := fmt.Sprintf("# Failed\n\nWatcher ID: %s\nStarted: %s\nFailed: %s\nError: %s\n",
		w.watcherID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339), processErr.Error())
	if err := os.WriteFile(failedMD, []byte(failedContent), 0644); err != nil {
		log.Printf("Warning: failed to create FAILED.md: %v", err)
	}

	// Remove PROCESSING.md
	os.Remove(filepath.Join(unit.FolderPath, "PROCESSING.md"))

	// Write record
	w.writeHeuristicRecord(unit, startTime, endTime, 1, "", 0, processErr)
}

// writeHeuristicRecord writes a record of the processed heuristic
func (w *HeuristicWatcher) writeHeuristicRecord(unit HeuristicUnit, startTime, endTime time.Time, exitCode int, requestID string, filesExtracted int, processErr error) {
	record := map[string]interface{}{
		"watcher_id":      w.watcherID,
		"heuristic_id":    unit.ID,
		"start_time":      startTime.Format(time.RFC3339),
		"end_time":        endTime.Format(time.RFC3339),
		"duration_ms":     endTime.Sub(startTime).Milliseconds(),
		"agent":           w.currentAgent,
		"exit_code":       exitCode,
		"request_id":      requestID,
		"files_extracted": filesExtracted,
		"success":         processErr == nil,
	}
	if processErr != nil {
		record["error"] = processErr.Error()
	}

	// Use a simple format since we don't have json package yet
	recordContent := fmt.Sprintf(`{
  "watcher_id": "%s",
  "heuristic_id": "%s",
  "start_time": "%s",
  "end_time": "%s",
  "duration_ms": %d,
  "agent": "%s",
  "exit_code": %d,
  "request_id": "%s",
  "files_extracted": %d,
  "success": %v`,
		record["watcher_id"], record["heuristic_id"],
		record["start_time"], record["end_time"],
		record["duration_ms"], record["agent"],
		record["exit_code"], record["request_id"],
		record["files_extracted"], record["success"])

	if processErr != nil {
		recordContent += fmt.Sprintf(",\n  \"error\": %q", processErr.Error())
	}
	recordContent += "\n}"

	recordFilename := fmt.Sprintf("%s_%s_%d.json", w.watcherID, unit.ID, time.Now().Unix())
	recordPath := filepath.Join(w.recordsDir, "heuristic", recordFilename)

	if err := os.WriteFile(recordPath, []byte(recordContent), 0644); err != nil {
		log.Printf("[%s] Warning: failed to write heuristic record: %v", w.watcherID, err)
	}
}

// run is the main watch loop
func (w *HeuristicWatcher) run() {
	log.Printf("[%s] Heuristic request watcher started", w.watcherID)
	log.Printf("[%s] Watching: %s", w.watcherID, filepath.Join(w.heuristicDir, "pending"))
	log.Printf("[%s] Requests: %s", w.watcherID, w.requestDir)
	log.Printf("[%s] Records: %s", w.watcherID, filepath.Join(w.recordsDir, "heuristic"))
	log.Printf("[%s] Default agent: %s", w.watcherID, w.currentAgent)
	if w.fileStory.Enabled() {
		log.Printf("[%s] File story logging: %s", w.watcherID, os.Getenv("FILE_STORY_PATH"))
	}

	for {
		units, err := w.checkForHeuristicUnits()
		if err != nil {
			log.Printf("[%s] Error checking for heuristic units: %v", w.watcherID, err)
		}

		if len(units) > 0 {
			// Reset backoff on activity
			w.lastActivity = time.Now()
			w.backoffIndex = 0
			w.nextBackoffLog = w.lastActivity.Add(backoffLevels[0])

			for _, unit := range units {
				if err := w.processHeuristicUnit(unit); err != nil {
					log.Printf("[%s] Error processing heuristic unit %s: %v", w.watcherID, unit.ID, err)
				}
			}
		} else {
			// No activity - check if we should log with backoff
			now := time.Now()
			if now.After(w.nextBackoffLog) {
				timeSinceActivity := now.Sub(w.lastActivity)
				log.Printf("[%s] No new heuristic activity for %s", w.watcherID, timeSinceActivity.Round(time.Second))

				// Advance to next backoff level if not at max
				if w.backoffIndex < len(backoffLevels)-1 {
					w.backoffIndex++
				}
				w.nextBackoffLog = now.Add(backoffLevels[w.backoffIndex])
			}
		}

		time.Sleep(checkInterval)
	}
}

// processOnce checks for units and processes them once (for testing)
func (w *HeuristicWatcher) processOnce() error {
	units, err := w.checkForHeuristicUnits()
	if err != nil {
		return err
	}

	if len(units) == 0 {
		log.Printf("[%s] No heuristic units found", w.watcherID)
		return nil
	}

	for _, unit := range units {
		if err := w.processHeuristicUnit(unit); err != nil {
			log.Printf("[%s] Error processing heuristic unit %s: %v", w.watcherID, unit.ID, err)
		}
	}

	return nil
}

func main() {
	watchFlag := flag.Bool("watch", false, "Start the heuristic watch loop (default behavior)")
	onceFlag := flag.Bool("once", false, "Process pending heuristics once and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "heuristic-request: Watch for HEURISTIC.md files and process them via prompt-only agents\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  heuristic-request [--watch|--once]\n\n")
		fmt.Fprintf(os.Stderr, "Environment Variables:\n")
		fmt.Fprintf(os.Stderr, "  HEURISTIC_DIR  Directory to watch for HEURISTIC.md files (default: %s)\n", defaultHeuristicDir)
		fmt.Fprintf(os.Stderr, "  REQUEST_DIR    Directory to place extracted requests for agent-dispatch (default: %s)\n", defaultRequestDir)
		fmt.Fprintf(os.Stderr, "  RECORDS_DIR    Records directory (default: %s)\n", defaultRecordsDir)
		fmt.Fprintf(os.Stderr, "  AGENT_PRESET   Agent to use (default: %s)\n\n", defaultAgent)
		fmt.Fprintf(os.Stderr, "How it works:\n")
		fmt.Fprintf(os.Stderr, "  1. Place HEURISTIC.md in a folder under HEURISTIC_DIR/pending/\n")
		fmt.Fprintf(os.Stderr, "  2. The watcher invokes an agent in prompt-only mode (-p)\n")
		fmt.Fprintf(os.Stderr, "  3. The agent analyzes the heuristic and outputs DISPATCH.md/json or INSTRUCTION.md/json\n")
		fmt.Fprintf(os.Stderr, "  4. The output is extracted and placed in REQUEST_DIR\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	watcher := NewHeuristicWatcher()

	if err := watcher.ensureDirectories(); err != nil {
		log.Fatalf("Failed to ensure directories: %v", err)
	}

	// Ensure the default read-space exists (copy of AI-sandboxing repo)
	if err := watcher.ensureDefaultReadSpace(); err != nil {
		log.Fatalf("Failed to ensure default read-space: %v", err)
	}

	if *onceFlag {
		if err := watcher.processOnce(); err != nil {
			log.Fatalf("Failed to process heuristics: %v", err)
		}
		return
	}

	// Default to watch mode
	_ = *watchFlag
	watcher.run()
}
