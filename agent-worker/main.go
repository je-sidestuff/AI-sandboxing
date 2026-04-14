package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/je-sidestuff/AI-sandboxing/pkg/agentaudit"
	"github.com/je-sidestuff/AI-sandboxing/pkg/agentselect"
	"github.com/je-sidestuff/AI-sandboxing/pkg/filestory"
)

// Default paths
const (
	defaultInputDir   = "/workspaces/slopspaces/input/"
	defaultOutputDir  = "/workspaces/slopspaces/output/"
	defaultRecordsDir = "/workspaces/slopspaces/agent-records/"
	checkInterval     = 10 * time.Second
	defaultAgent      = "claude"

	// Agent workspace - the restricted environment where the AI runs
	// In host mode: files are copied in/out; AI runs as restricted user
	// In container mode (future): directories are mounted; AI runs in container
	agentWorkspaceRoot = "/agent/agent-worker"
	defaultAgentUser   = "agent-worker"

	// Default read-space: a cached copy of AI-sandboxing repo (minus .git)
	// This provides agents with visibility into the codebase
	defaultReadSpacePath   = "/workspaces/slopspaces/read-spaces/default"
	defaultReadSpaceSource = "/workspaces/workspace/sandbox/AI-sandboxing"
)

// Work unit type constants
const (
	WorkUnitTypeInstruction = "instruction"
	WorkUnitTypeReport      = "report"
)

// Report type constants
const (
	ReportTypeCustom  = "custom"
	ReportTypeDaily   = "daily"
	ReportTypeWeekly  = "weekly"
	ReportTypeMonthly = "monthly"
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

// Instruction represents the JSON structure for work instructions
type Instruction struct {
	Instruction        string `json:"instruction"`
	Mode               string `json:"mode"`
	Agent              string `json:"agent,omitempty"`
	Model              string `json:"model,omitempty"`
	Capability         string `json:"capability,omitempty"`
	SelectionRationale string `json:"selection_rationale,omitempty"`
	Timestamp          string `json:"timestamp,omitempty"`
}

// Report represents the JSON structure for report work units
type Report struct {
	Type               string `json:"type"`
	Content            string `json:"content,omitempty"`
	Agent              string `json:"agent,omitempty"`
	Model              string `json:"model,omitempty"`
	Capability         string `json:"capability,omitempty"`
	SelectionRationale string `json:"selection_rationale,omitempty"`
	Timestamp          string `json:"timestamp,omitempty"`
}

// WorkUnit represents a discovered work unit with its type
type WorkUnit struct {
	Path string
	Type string // "instruction" or "report"
}

// WorkerRecord holds metadata for processed work units
type WorkerRecord struct {
	WorkerID           string `json:"worker_id"`
	WorkUnit           string `json:"work_unit"`
	StartTime          string `json:"start_time"`
	EndTime            string `json:"end_time"`
	DurationMs         int64  `json:"duration_ms"`
	Agent              string `json:"agent"`
	Model              string `json:"model,omitempty"`
	Capability         string `json:"capability,omitempty"`
	SelectionRationale string `json:"selection_rationale,omitempty"`
	Mode               string `json:"mode,omitempty"`
	ReportType         string `json:"report_type,omitempty"`
	ExitCode           int    `json:"exit_code"`
	InputPath          string `json:"input_path"`
	OutputPath         string `json:"output_path"`
}

// AgentWorkspace represents the prepared workspace for an agent invocation
// The workspace is located at /agent/agent-worker/ and contains:
//   - read/default/  : Copy of AI-sandboxing repo (excluding .git) - the default read-space
//   - read/workunit/ : Copy of work unit content (the instruction folder)
//   - write/primary/ : Agent's working directory where it can create/modify files
type AgentWorkspace struct {
	RootPath     string // /agent/agent-worker
	ReadDefault  string // /agent/agent-worker/read/default (AI-sandboxing repo)
	ReadWorkunit string // /agent/agent-worker/read/workunit (work unit content)
	WritePrimary string // /agent/agent-worker/write/primary
}

// AgentWorker manages the work processing loop
type AgentWorker struct {
	workerID           string
	inputDir           string
	outputDir          string
	recordsDir         string
	currentAgent       string
	lastActivity       time.Time
	backoffIndex       int
	nextBackoffLog     time.Time
	instructionEnabled bool
	reportEnabled      bool
	agentUser          string            // OS user to run agent as (for host mode isolation)
	agentUID           int               // UID of agent user (0 if not found/not using)
	agentGID           int               // GID of agent user (0 if not found/not using)
	fileStory          *filestory.Logger // File operation logger (nil if FILE_STORY_PATH not set)
}

// NewAgentWorker creates a new worker instance
func NewAgentWorker() *AgentWorker {
	inputDir := os.Getenv("INPUT_DIR")
	if inputDir == "" {
		inputDir = defaultInputDir
	}

	outputDir := os.Getenv("OUTPUT_DIR")
	if outputDir == "" {
		outputDir = defaultOutputDir
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

	// By default, both instruction and report modes are enabled
	// Set WORKER_INSTRUCTION_ENABLED=false to disable instruction processing
	// Set WORKER_REPORT_ENABLED=false to disable report processing
	instructionEnabled := true
	if os.Getenv("WORKER_INSTRUCTION_ENABLED") == "false" {
		instructionEnabled = false
	}

	reportEnabled := true
	if os.Getenv("WORKER_REPORT_ENABLED") == "false" {
		reportEnabled = false
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
	workerID := uuid.New().String()[:8]

	// Initialize file story logger (uses FILE_STORY_PATH env var)
	fileStoryLogger := filestory.NewLogger("agent-worker")

	return &AgentWorker{
		workerID:           workerID,
		inputDir:           inputDir,
		outputDir:          outputDir,
		recordsDir:         recordsDir,
		currentAgent:       currentAgent,
		lastActivity:       now,
		backoffIndex:       0,
		nextBackoffLog:     now.Add(backoffLevels[0]),
		instructionEnabled: instructionEnabled,
		reportEnabled:      reportEnabled,
		agentUser:          agentUser,
		agentUID:           agentUID,
		agentGID:           agentGID,
		fileStory:          fileStoryLogger,
	}
}

// ensureDirectories creates necessary directories
func (w *AgentWorker) ensureDirectories() error {
	dirs := []string{
		filepath.Join(w.inputDir, "any"),
		filepath.Join(w.outputDir, "content"),
		filepath.Join(w.outputDir, "records"),
		filepath.Join(w.recordsDir, "worker"),
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
func (w *AgentWorker) ensureDefaultReadSpace() error {
	// Check if default read-space directory exists
	if _, err := os.Stat(defaultReadSpacePath); os.IsNotExist(err) {
		log.Printf("[%s] Creating default read-space at %s", w.workerID, defaultReadSpacePath)

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

		log.Printf("[%s] Default read-space created successfully", w.workerID)
	}

	return nil
}

// prepareWorkspace creates the agent workspace at /agent/agent-worker/
// This clears any previous content and sets up fresh read/write spaces.
// In host mode, the agent-worker binary (running as a privileged user) prepares this
// workspace, then runs the AI as a restricted user that only has access to this directory.
//
// The workspace contains:
//   - read/default/  : Copy of AI-sandboxing repo (the default read-space)
//   - read/workunit/ : Copy of work unit content (the instruction folder)
//   - write/primary/ : Agent's working directory
func (w *AgentWorker) prepareWorkspace(sourcePath string) (*AgentWorkspace, error) {
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
	// - The AI may have created files owned by the agent-worker user
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
func (w *AgentWorker) cleanupWorkspace() {
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

// buildWorkspacePreamble creates a context preamble that explains the workspace structure
// to the agent. This helps agents understand their filesystem boundaries and prevents
// them from requesting broad access like "/*".
func buildWorkspacePreamble(workspace *AgentWorkspace) string {
	return fmt.Sprintf(`## Workspace Environment

You are running in a sandboxed workspace. Here's what you need to know:

**YOUR WORKING DIRECTORY (where you MUST create/modify all files):**
%s

**Reference materials (READ-ONLY - do NOT try to write here):**
- %s - This is a READ-ONLY copy of the codebase for reference
- %s - This contains your work unit instructions

**CRITICAL RULES:**
1. ALL file operations (create, edit, write) MUST happen in %s
2. The read/default/ directory contains a COPY of the codebase - do NOT try to modify it
3. If you see paths like "ignored-scratch/" or "agent-scribe/" in the codebase reference, those are READ-ONLY copies - create your own directories in the write space instead
4. Do NOT request access to paths outside this workspace (like /* or /root)
5. When you need to create test files, logs, or any output - use your working directory

**Root workspace:** %s

---

`, workspace.WritePrimary, workspace.ReadDefault, workspace.ReadWorkunit, workspace.WritePrimary, workspace.RootPath)
}

// prepareAgentConfig writes agent-specific configuration files to the workspace.
// Different agents have different ways to configure allowed directories:
// - Claude Code: uses --add-dir flag (handled in invoke-agent.sh)
// - Copilot: uses --add-dir flag (handled in invoke-agent.sh)
// - OpenCode: uses .opencode.json config file (location may vary)
// - Others: may not support external directory configuration
func (w *AgentWorker) prepareAgentConfig(workspace *AgentWorkspace, agent string) error {
	switch agent {
	case "opencode":
		// OpenCode uses a .opencode.json config file to specify allowed directories.
		// We write the config to multiple locations since opencode might look in:
		// 1. Current working directory (workspace.WritePrimary)
		// 2. User's home directory ($HOME or /agent/agent-worker which is the agent's home)
		//
		// IMPORTANT: We are very restrictive about write access:
		// - ONLY the write/primary directory should be writable
		// - read/default/ and read/workunit/ are for reading reference materials
		// - The agent must NOT try to write to paths that look like they came from the codebase copy
		//
		// The read/default/ directory contains a COPY of the AI-sandboxing repo. When
		// the agent sees paths like "ignored-scratch/" or "agent-scribe/" there, those are
		// NOT actual working directories - they're read-only reference copies. The agent
		// should create its own directories in write/primary/ instead.
		opencodeConfig := map[string]interface{}{
			"lsp": map[string]interface{}{
				"disabled": true,
			},
			// Context paths for file reading - agent can READ from these
			"contextPaths": []string{
				workspace.WritePrimary, // /agent/agent-worker/write/primary (working dir)
				workspace.ReadDefault,  // /agent/agent-worker/read/default (codebase reference - READ ONLY)
				workspace.ReadWorkunit, // /agent/agent-worker/read/workunit (work unit - READ ONLY)
			},
			// Allowed directories for external access - ONLY the write space for modifications
			"allowedDirectories": []string{
				workspace.WritePrimary, // ONLY allow writes to write/primary
			},
			// External directories that the agent can access (read)
			"externalDirectories": []string{
				workspace.RootPath, // /agent/agent-worker (for reading)
			},
			// Permissions for file operations - be restrictive
			"permissions": map[string]interface{}{
				// Only auto-approve the write space for external_directory permission
				"external_directory": []string{
					workspace.WritePrimary,                            // /agent/agent-worker/write/primary
					workspace.WritePrimary + "/*",                     // subdirs of write space
					filepath.Join(workspace.RootPath, "write") + "/*", // anything under write/
				},
			},
			// Auto-approve permissions ONLY for write paths
			"autoApprove": map[string]interface{}{
				"external_directory": []string{
					workspace.WritePrimary + "/*",                     // Only auto-approve write/primary/*
					filepath.Join(workspace.RootPath, "write") + "/*", // and write/*
				},
			},
		}

		configData, err := json.MarshalIndent(opencodeConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal opencode config: %w", err)
		}

		// Write to working directory
		configPath := filepath.Join(workspace.WritePrimary, ".opencode.json")
		if err := os.WriteFile(configPath, configData, 0644); err != nil {
			return fmt.Errorf("failed to write opencode config: %w", err)
		}

		// Also write to the agent workspace root (which serves as the agent's "home")
		// This covers cases where opencode looks in $HOME or the process's home dir
		homeConfigPath := filepath.Join(workspace.RootPath, ".opencode.json")
		if err := os.WriteFile(homeConfigPath, configData, 0644); err != nil {
			log.Printf("[%s] Warning: failed to write opencode config to home: %v", w.workerID, err)
		}

		// Set ownership if running as root
		if os.Getuid() == 0 && w.agentUID != 0 {
			os.Chown(configPath, w.agentUID, w.agentGID)
			os.Chown(homeConfigPath, w.agentUID, w.agentGID)
		}

		log.Printf("[%s] Created opencode config for workspace: %s", w.workerID, workspace.RootPath)
	}

	return nil
}

// copyWriteSpaceOutput copies files from write/primary back to the work unit folder
// This extracts the agent's output after execution
func (w *AgentWorker) copyWriteSpaceOutput(workspace *AgentWorkspace, destPath string) error {
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

// checkForWorkUnits scans the input directory for work packages
func (w *AgentWorker) checkForWorkUnits() ([]WorkUnit, error) {
	anyDir := filepath.Join(w.inputDir, "any")
	entries, err := os.ReadDir(anyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Log directory scan (uses abbreviation if >8 entries)
	if len(entries) > 0 {
		w.fileStory.LogTree(filestory.OpListDir, anyDir)
	}

	var workUnits []WorkUnit
	for _, entry := range entries {
		if entry.IsDir() {
			folderPath := filepath.Join(anyDir, entry.Name())

			// Skip if already being processed
			processingMD := filepath.Join(folderPath, "PROCESSING.md")
			if _, err := os.Stat(processingMD); err == nil {
				continue
			}

			// Check for instruction files
			if w.instructionEnabled {
				instructionJSON := filepath.Join(folderPath, "INSTRUCTION.json")
				instructionMD := filepath.Join(folderPath, "INSTRUCTION.md")
				_, jsonExists := os.Stat(instructionJSON)
				_, mdExists := os.Stat(instructionMD)

				if jsonExists == nil || mdExists == nil {
					workUnits = append(workUnits, WorkUnit{Path: folderPath, Type: WorkUnitTypeInstruction})
					continue
				}
			}

			// Check for report files
			if w.reportEnabled {
				reportJSON := filepath.Join(folderPath, "REPORT.json")
				reportMD := filepath.Join(folderPath, "REPORT.md")
				_, jsonExists := os.Stat(reportJSON)
				_, mdExists := os.Stat(reportMD)

				if jsonExists == nil || mdExists == nil {
					workUnits = append(workUnits, WorkUnit{Path: folderPath, Type: WorkUnitTypeReport})
					continue
				}
			}
		}
	}

	return workUnits, nil
}

// handleInstructionFiles processes the instruction files according to the spec
func (w *AgentWorker) handleInstructionFiles(folderPath string) (*Instruction, error) {
	instructionJSON := filepath.Join(folderPath, "INSTRUCTION.json")
	instructionMD := filepath.Join(folderPath, "INSTRUCTION.md")

	_, jsonExists := os.Stat(instructionJSON)
	_, mdExists := os.Stat(instructionMD)

	// If INSTRUCTION.json exists
	if jsonExists == nil {
		// Delete INSTRUCTION.md if it exists (to show it was ignored)
		if mdExists == nil {
			if err := os.Remove(instructionMD); err != nil {
				log.Printf("Warning: failed to remove INSTRUCTION.md: %v", err)
			}
		}

		// Read and parse the JSON
		data, err := os.ReadFile(instructionJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to read INSTRUCTION.json: %w", err)
		}
		w.fileStory.LogFile(filestory.OpRead, instructionJSON)

		var inst Instruction
		if err := json.Unmarshal(data, &inst); err != nil {
			return nil, fmt.Errorf("failed to parse INSTRUCTION.json: %w", err)
		}

		// Validate mode
		if inst.Mode != "prompt" && inst.Mode != "execute" {
			inst.Mode = "prompt" // Default to prompt mode
		}

		return &inst, nil
	}

	// If only INSTRUCTION.md exists, convert it to INSTRUCTION.json
	if mdExists == nil {
		// Read the markdown content
		mdContent, err := os.ReadFile(instructionMD)
		if err != nil {
			return nil, fmt.Errorf("failed to read INSTRUCTION.md: %w", err)
		}
		w.fileStory.LogFile(filestory.OpRead, instructionMD)

		// Create the instruction struct
		inst := Instruction{
			Instruction: string(mdContent),
			Mode:        "prompt", // Default mode for converted files
			Timestamp:   time.Now().Format(time.RFC3339),
		}

		// Write the JSON file
		jsonData, err := json.MarshalIndent(inst, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal INSTRUCTION.json: %w", err)
		}

		if err := os.WriteFile(instructionJSON, jsonData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write INSTRUCTION.json: %w", err)
		}

		// Remove the original INSTRUCTION.md (it's been converted)
		if err := os.Remove(instructionMD); err != nil {
			log.Printf("Warning: failed to remove INSTRUCTION.md after conversion: %v", err)
		}

		return &inst, nil
	}

	return nil, fmt.Errorf("no instruction file found")
}

// handleReportFiles processes the report files according to the spec
// REPORT.json takes precedence over REPORT.md
// REPORT.md is converted to REPORT.json with type "custom"
func (w *AgentWorker) handleReportFiles(folderPath string) (*Report, error) {
	reportJSON := filepath.Join(folderPath, "REPORT.json")
	reportMD := filepath.Join(folderPath, "REPORT.md")

	_, jsonExists := os.Stat(reportJSON)
	_, mdExists := os.Stat(reportMD)

	// If REPORT.json exists
	if jsonExists == nil {
		// Delete REPORT.md if it exists (to show it was ignored)
		if mdExists == nil {
			if err := os.Remove(reportMD); err != nil {
				log.Printf("Warning: failed to remove REPORT.md: %v", err)
			}
		}

		// Read and parse the JSON
		data, err := os.ReadFile(reportJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to read REPORT.json: %w", err)
		}
		w.fileStory.LogFile(filestory.OpRead, reportJSON)

		var report Report
		if err := json.Unmarshal(data, &report); err != nil {
			return nil, fmt.Errorf("failed to parse REPORT.json: %w", err)
		}

		// Validate type
		if report.Type != ReportTypeCustom && report.Type != ReportTypeDaily &&
			report.Type != ReportTypeWeekly && report.Type != ReportTypeMonthly {
			return nil, fmt.Errorf("invalid report type: %s (must be custom, daily, weekly, or monthly)", report.Type)
		}

		return &report, nil
	}

	// If only REPORT.md exists, convert it to REPORT.json with type "custom"
	if mdExists == nil {
		// Read the markdown content
		mdContent, err := os.ReadFile(reportMD)
		if err != nil {
			return nil, fmt.Errorf("failed to read REPORT.md: %w", err)
		}
		w.fileStory.LogFile(filestory.OpRead, reportMD)

		// Create the report struct with type "custom"
		report := Report{
			Type:      ReportTypeCustom,
			Content:   string(mdContent),
			Timestamp: time.Now().Format(time.RFC3339),
		}

		// Write the JSON file
		jsonData, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal REPORT.json: %w", err)
		}

		if err := os.WriteFile(reportJSON, jsonData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write REPORT.json: %w", err)
		}

		// Remove the original REPORT.md (it's been converted)
		if err := os.Remove(reportMD); err != nil {
			log.Printf("Warning: failed to remove REPORT.md after conversion: %v", err)
		}

		return &report, nil
	}

	return nil, fmt.Errorf("no report file found")
}

// processWorkUnit handles a single work package
func (w *AgentWorker) processWorkUnit(folderPath string) error {
	folderName := filepath.Base(folderPath)
	log.Printf("[%s] Processing work unit: %s", w.workerID, folderName)

	startTime := time.Now()

	// Handle instruction files
	inst, err := w.handleInstructionFiles(folderPath)
	if err != nil {
		return fmt.Errorf("failed to handle instruction files: %w", err)
	}

	// Create PROCESSING.md to mark we're working on it
	processingMD := filepath.Join(folderPath, "PROCESSING.md")
	processingContent := fmt.Sprintf("# Processing\n\nWorker ID: %s\nStarted: %s\nAgent: %s\nMode: %s\n",
		w.workerID, startTime.Format(time.RFC3339), w.getAgent(inst), inst.Mode)
	if err := os.WriteFile(processingMD, []byte(processingContent), 0644); err != nil {
		return fmt.Errorf("failed to create PROCESSING.md: %w", err)
	}

	// Determine which agent to use
	agent := w.getAgent(inst)

	// Execute the agent
	exitCode := w.executeAgent(folderPath, inst, agent)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Format timestamp for directory and file naming
	timestamp := endTime.Format("2006-01-02_15-04-05")

	// Create content and records directories
	contentPath := filepath.Join(w.outputDir, "content", folderName)
	recordsPath := filepath.Join(w.outputDir, "records", fmt.Sprintf("%s-%s", folderName, timestamp))

	if err := os.MkdirAll(contentPath, 0755); err != nil {
		return fmt.Errorf("failed to create content directory: %w", err)
	}
	if err := os.MkdirAll(recordsPath, 0755); err != nil {
		return fmt.Errorf("failed to create records directory: %w", err)
	}

	// Generate PROCESSED.md content
	processedContent := fmt.Sprintf("# Processed\n\nWorker ID: %s\nStarted: %s\nCompleted: %s\nDuration: %s\nAgent: %s\nMode: %s\nExit Code: %d\n",
		w.workerID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
		duration.Round(time.Millisecond).String(), agent, inst.Mode, exitCode)

	// Move work content to content directory, excluding metadata files
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return fmt.Errorf("failed to read work folder: %w", err)
	}

	metadataFiles := map[string]bool{
		"PROCESSING.md":    true,
		"INSTRUCTION.json": true,
		"INSTRUCTION.md":   true,
	}

	for _, entry := range entries {
		srcPath := filepath.Join(folderPath, entry.Name())
		if metadataFiles[entry.Name()] {
			// Move metadata files to records directory
			destPath := filepath.Join(recordsPath, entry.Name())
			if err := os.Rename(srcPath, destPath); err != nil {
				log.Printf("Warning: failed to move metadata file %s: %v", entry.Name(), err)
			}
		} else {
			// Move content files to content directory
			destPath := filepath.Join(contentPath, entry.Name())
			if err := os.Rename(srcPath, destPath); err != nil {
				log.Printf("Warning: failed to move content file %s: %v", entry.Name(), err)
			}
		}
	}

	// Write PROCESSED.md to records directory (with original name)
	processedMDRecords := filepath.Join(recordsPath, "PROCESSED.md")
	if err := os.WriteFile(processedMDRecords, []byte(processedContent), 0644); err != nil {
		log.Printf("Warning: failed to create PROCESSED.md in records: %v", err)
	}

	// Write PROCESSED.md to content directory (with timestamp suffix)
	processedMDContent := filepath.Join(contentPath, fmt.Sprintf("PROCESSED-%s.md", timestamp))
	if err := os.WriteFile(processedMDContent, []byte(processedContent), 0644); err != nil {
		log.Printf("Warning: failed to create PROCESSED-%s.md in content: %v", timestamp, err)
	}

	// Remove the now-empty input folder
	if err := os.Remove(folderPath); err != nil {
		log.Printf("Warning: failed to remove input folder: %v", err)
	}

	// Write worker record
	if err := w.writeWorkerRecord(folderName, inst, agent, startTime, endTime, exitCode, folderPath, contentPath); err != nil {
		log.Printf("Warning: failed to write worker record: %v", err)
	}

	log.Printf("[%s] Completed work unit: %s (exit: %d, duration: %s)", w.workerID, folderName, exitCode, duration.Round(time.Millisecond))
	log.Printf("[%s] Content: %s", w.workerID, contentPath)
	log.Printf("[%s] Records: %s", w.workerID, recordsPath)
	return nil
}

// processReportWorkUnit handles a report work package
func (w *AgentWorker) processReportWorkUnit(folderPath string) error {
	folderName := filepath.Base(folderPath)
	log.Printf("[%s] Processing report work unit: %s", w.workerID, folderName)

	startTime := time.Now()

	// Handle report files
	report, err := w.handleReportFiles(folderPath)
	if err != nil {
		return fmt.Errorf("failed to handle report files: %w", err)
	}

	// Determine which agent to use
	agent := w.getAgentFromReport(report)

	// Create PROCESSING.md to mark we're working on it
	processingMD := filepath.Join(folderPath, "PROCESSING.md")
	processingContent := fmt.Sprintf("# Processing\n\nWorker ID: %s\nStarted: %s\nAgent: %s\nReport Type: %s\n",
		w.workerID, startTime.Format(time.RFC3339), agent, report.Type)
	if err := os.WriteFile(processingMD, []byte(processingContent), 0644); err != nil {
		return fmt.Errorf("failed to create PROCESSING.md: %w", err)
	}

	// Execute the report generation
	exitCode := w.executeReportAgent(folderPath, report, agent)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Format timestamp for directory and file naming
	timestamp := endTime.Format("2006-01-02_15-04-05")

	// Create content and records directories
	contentPath := filepath.Join(w.outputDir, "content", folderName)
	recordsPath := filepath.Join(w.outputDir, "records", fmt.Sprintf("%s-%s", folderName, timestamp))

	if err := os.MkdirAll(contentPath, 0755); err != nil {
		return fmt.Errorf("failed to create content directory: %w", err)
	}
	if err := os.MkdirAll(recordsPath, 0755); err != nil {
		return fmt.Errorf("failed to create records directory: %w", err)
	}

	// Generate PROCESSED.md content
	processedContent := fmt.Sprintf("# Processed\n\nWorker ID: %s\nStarted: %s\nCompleted: %s\nDuration: %s\nAgent: %s\nReport Type: %s\nExit Code: %d\n",
		w.workerID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
		duration.Round(time.Millisecond).String(), agent, report.Type, exitCode)

	// Move work content to content directory, excluding metadata files
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return fmt.Errorf("failed to read work folder: %w", err)
	}

	metadataFiles := map[string]bool{
		"PROCESSING.md": true,
		"REPORT.json":   true,
		"REPORT.md":     true,
	}

	for _, entry := range entries {
		srcPath := filepath.Join(folderPath, entry.Name())
		if metadataFiles[entry.Name()] {
			// Move metadata files to records directory
			destPath := filepath.Join(recordsPath, entry.Name())
			if err := os.Rename(srcPath, destPath); err != nil {
				log.Printf("Warning: failed to move metadata file %s: %v", entry.Name(), err)
			}
		} else {
			// Move content files to content directory
			destPath := filepath.Join(contentPath, entry.Name())
			if err := os.Rename(srcPath, destPath); err != nil {
				log.Printf("Warning: failed to move content file %s: %v", entry.Name(), err)
			}
		}
	}

	// Write PROCESSED.md to records directory (with original name)
	processedMDRecords := filepath.Join(recordsPath, "PROCESSED.md")
	if err := os.WriteFile(processedMDRecords, []byte(processedContent), 0644); err != nil {
		log.Printf("Warning: failed to create PROCESSED.md in records: %v", err)
	}

	// Write PROCESSED.md to content directory (with timestamp suffix)
	processedMDContent := filepath.Join(contentPath, fmt.Sprintf("PROCESSED-%s.md", timestamp))
	if err := os.WriteFile(processedMDContent, []byte(processedContent), 0644); err != nil {
		log.Printf("Warning: failed to create PROCESSED-%s.md in content: %v", timestamp, err)
	}

	// Remove the now-empty input folder
	if err := os.Remove(folderPath); err != nil {
		log.Printf("Warning: failed to remove input folder: %v", err)
	}

	// Write worker record
	if err := w.writeReportWorkerRecord(folderName, report, agent, startTime, endTime, exitCode, folderPath, contentPath); err != nil {
		log.Printf("Warning: failed to write worker record: %v", err)
	}

	log.Printf("[%s] Completed report work unit: %s (exit: %d, duration: %s)", w.workerID, folderName, exitCode, duration.Round(time.Millisecond))
	log.Printf("[%s] Content: %s", w.workerID, contentPath)
	log.Printf("[%s] Records: %s", w.workerID, recordsPath)
	return nil
}

// getAgent returns the agent to use, considering instruction override
func (w *AgentWorker) getAgent(inst *Instruction) string {
	sel, err := agentselect.Select(inst.Instruction, inst.Agent, inst.Model, inst.Capability)
	if err != nil {
		log.Printf("[%s] Selection error: %v, using default agent", w.workerID, err)
		return w.currentAgent
	}
	return sel.Agent
}

// getAgentFromReport returns the agent to use, considering report override
func (w *AgentWorker) getAgentFromReport(report *Report) string {
	taskDesc := report.Content
	if taskDesc == "" {
		taskDesc = report.Type
	}
	sel, err := agentselect.Select(taskDesc, report.Agent, report.Model, report.Capability)
	if err != nil {
		log.Printf("[%s] Selection error: %v, using default agent", w.workerID, err)
		return w.currentAgent
	}
	return sel.Agent
}

// executeAgent runs the agent against the work unit
// It prepares the isolated workspace at /agent/agent-worker/, runs the AI as a
// restricted user (in host mode), and copies output back to the work unit folder.
func (w *AgentWorker) executeAgent(folderPath string, inst *Instruction, agent string) int {
	// Find invoke-agent.sh
	invokeScript := findInvokeScript()
	if invokeScript == "" {
		log.Printf("Error: invoke-agent.sh not found")
		return 1
	}

	// Prepare the workspace - copy work unit to /agent/agent-worker/read/default/
	workspace, err := w.prepareWorkspace(folderPath)
	if err != nil {
		log.Printf("Error: failed to prepare workspace: %v", err)
		return 1
	}
	defer func() {
		// Copy output from write space back to work unit folder
		if copyErr := w.copyWriteSpaceOutput(workspace, folderPath); copyErr != nil {
			log.Printf("Warning: failed to copy write space output: %v", copyErr)
		}
		// Clean up workspace
		w.cleanupWorkspace()
	}()

	// Write agent-specific configuration files (e.g., .opencode.json for opencode)
	if err := w.prepareAgentConfig(workspace, agent); err != nil {
		log.Printf("Warning: failed to prepare agent config: %v", err)
		// Continue anyway - some agents may work without config
	}

	// Determine mode flag
	modeFlag := "-p"
	if inst.Mode == "execute" {
		modeFlag = "-e"
	}

	// Build the full instruction with workspace context preamble
	// This helps the agent understand its filesystem boundaries
	fullInstruction := buildWorkspacePreamble(workspace) + inst.Instruction

	if auditErr := agentaudit.Capture(agentaudit.Input{
		AgentType:          "agent-worker",
		ID:                 w.workerID,
		Agent:              agent,
		Model:              inst.Model,
		Capability:         inst.Capability,
		SelectionRationale: inst.SelectionRationale,
		Prompt:             fullInstruction,
		FSPaths:            []string{workspace.RootPath, folderPath},
	}); auditErr != nil {
		log.Printf("[%s] Warning: agent audit capture failed: %v", w.workerID, auditErr)
	}

	// Write instruction to the write space so the agent can see it
	promptFile := filepath.Join(workspace.WritePrimary, ".prompt.tmp")
	if err := os.WriteFile(promptFile, []byte(fullInstruction), 0644); err != nil {
		log.Printf("Error: failed to write prompt file: %v", err)
		return 1
	}
	// Set ownership of prompt file if running as root
	if os.Getuid() == 0 && w.agentUID != 0 {
		os.Chown(promptFile, w.agentUID, w.agentGID)
	}

	cmdArgs := []string{modeFlag, "-a", agent}
	if inst.Model != "" {
		cmdArgs = append(cmdArgs, "-m", inst.Model)
	}
	if inst.Capability != "" {
		cmdArgs = append(cmdArgs, "-c", inst.Capability)
	}
	cmdArgs = append(cmdArgs, "-f", promptFile)

	// Create command
	cmd := exec.Command(invokeScript, cmdArgs...)

	// Agent runs from write/primary/ - this is its working directory
	cmd.Dir = workspace.WritePrimary

	// Set AGENT_FULL_AUTO for execute mode since approval happened via PR workflow
	fullAutoEnv := "0"
	if inst.Mode == "execute" {
		fullAutoEnv = "1"
	}

	// Grant access to the entire agent workspace root (/agent/agent-worker/)
	// The agent sees this as its world:
	//   /agent/agent-worker/
	//   ├── read/           (read-only via OS permissions)
	//   │   ├── default/    (AI-sandboxing codebase)
	//   │   └── workunit/   (instruction work unit files)
	//   └── write/          (writable via OS permissions)
	//       └── primary/    (working directory)
	//
	// OS-level user permissions enforce that read/ is read-only and write/ is writable.
	//
	// Note: We do NOT override HOME here. Claude and other agents need access to their
	// credentials in ~/.claude/ or similar. The working directory (cmd.Dir) is set
	// to the write space, but the agent can still access its normal config files.
	// Tools like opencode that need local config files should have them placed in
	// the workspace read space if required.
	cmd.Env = append(os.Environ(),
		"AGENT_NAME="+agent,
		"AGENT_PRESET="+agent, // backwards compatibility
		"AGENT_RECORDS_PATH="+w.recordsDir,
		"AGENT_FULL_AUTO="+fullAutoEnv,
		"AGENT_ADD_DIRS="+workspace.RootPath,
	)

	// In host mode, run as restricted user if configured and we're root
	if os.Getuid() == 0 && w.agentUID != 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(w.agentUID),
				Gid: uint32(w.agentGID),
			},
		}
		log.Printf("[%s] Running agent as user %s (uid=%d)", w.workerID, w.agentUser, w.agentUID)
	} else if w.agentUser != "" && w.agentUID == 0 {
		// User configured but not found - warn but continue as current user
		log.Printf("[%s] Warning: agent user '%s' not found, running as current user", w.workerID, w.agentUser)
	}

	// For daemon/background operation, stdin should not be connected to os.Stdin
	// as this can cause hangs when there's no TTY. In execute mode with AGENT_FULL_AUTO,
	// the agent is pre-approved and doesn't need interactive input.
	// In prompt mode, it's explicitly non-interactive.
	// Leaving cmd.Stdin as nil (defaults to /dev/null) prevents hanging.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("[%s] Invoking agent %s with mode %s", w.workerID, agent, inst.Mode)
	log.Printf("[%s] Workspace: %s", w.workerID, workspace.RootPath)

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		log.Printf("Error running agent: %v", err)
		return 1
	}

	return 0
}

// buildReportContextPrompt creates the contextual preamble for report generation
func (w *AgentWorker) buildReportContextPrompt() string {
	return `You are generating a report based on agent work records and history.

## Context Available to You

You have access to agent records that document previous work sessions. These records can be found in the records directory and include:
- Worker records: JSON files documenting completed work units, including timestamps, agents used, and exit codes
- Session transcripts: Previous agent conversations and their outputs
- Output artifacts: Files and documents created during previous work sessions

## How to Use This Context

1. **Explore the records directory** to understand what work has been done
2. **Read relevant session files** to understand the details of past work
3. **Synthesize information** from multiple sources to create a comprehensive report
4. **Focus on the specific request** outlined below

## Your Task

`
}

// executeReportAgent runs the agent for a report work unit
// It prepares the isolated workspace at /agent/agent-worker/, runs the AI as a
// restricted user (in host mode), and copies output back to the work unit folder.
func (w *AgentWorker) executeReportAgent(folderPath string, report *Report, agent string) int {
	// Find invoke-agent.sh
	invokeScript := findInvokeScript()
	if invokeScript == "" {
		log.Printf("Error: invoke-agent.sh not found")
		return 1
	}

	// Prepare the workspace - copy work unit to /agent/agent-worker/read/default/
	workspace, err := w.prepareWorkspace(folderPath)
	if err != nil {
		log.Printf("Error: failed to prepare workspace: %v", err)
		return 1
	}
	defer func() {
		// Copy output from write space back to work unit folder
		if copyErr := w.copyWriteSpaceOutput(workspace, folderPath); copyErr != nil {
			log.Printf("Warning: failed to copy write space output: %v", copyErr)
		}
		// Clean up workspace
		w.cleanupWorkspace()
	}()

	// Write agent-specific configuration files (e.g., .opencode.json for opencode)
	if err := w.prepareAgentConfig(workspace, agent); err != nil {
		log.Printf("Warning: failed to prepare agent config: %v", err)
		// Continue anyway - some agents may work without config
	}

	// Build the instruction based on report type
	var instruction string
	contextPrompt := w.buildReportContextPrompt()

	switch report.Type {
	case ReportTypeCustom:
		// Custom reports use the content field as the primary instruction
		if report.Content == "" {
			log.Printf("Error: custom report has no content")
			return 1
		}
		instruction = contextPrompt + report.Content
	case ReportTypeDaily:
		instruction = contextPrompt + "Generate a daily report summarizing today's activities, progress, and any blockers. Look through the recent records and session outputs to compile this summary."
	case ReportTypeWeekly:
		instruction = contextPrompt + "Generate a weekly report summarizing this week's activities, accomplishments, progress toward goals, and upcoming priorities. Analyze the records from the past week to compile this summary."
	case ReportTypeMonthly:
		instruction = contextPrompt + "Generate a monthly report summarizing this month's activities, key accomplishments, metrics, challenges encountered, and strategic outlook. Review the records from the past month to create this comprehensive summary."
	default:
		log.Printf("Error: unsupported report type: %s", report.Type)
		return 1
	}

	// Reports always use execute mode to allow file creation
	modeFlag := "-e"

	// Add workspace context preamble to help agent understand its boundaries
	fullInstruction := buildWorkspacePreamble(workspace) + instruction

	if auditErr := agentaudit.Capture(agentaudit.Input{
		AgentType:          "agent-worker",
		ID:                 w.workerID,
		Agent:              agent,
		Model:              report.Model,
		Capability:         report.Capability,
		SelectionRationale: report.SelectionRationale,
		Prompt:             fullInstruction,
		FSPaths:            []string{workspace.RootPath, folderPath},
	}); auditErr != nil {
		log.Printf("[%s] Warning: agent audit capture failed: %v", w.workerID, auditErr)
	}

	// Write instruction to the write space so the agent can see it
	promptFile := filepath.Join(workspace.WritePrimary, ".prompt.tmp")
	if err := os.WriteFile(promptFile, []byte(fullInstruction), 0644); err != nil {
		log.Printf("Error: failed to write prompt file: %v", err)
		return 1
	}
	// Set ownership of prompt file if running as root
	if os.Getuid() == 0 && w.agentUID != 0 {
		os.Chown(promptFile, w.agentUID, w.agentGID)
	}

	// Build command arguments using -f to read prompt from file
	cmdArgs := []string{modeFlag, "-a", agent}
	if report.Model != "" {
		cmdArgs = append(cmdArgs, "-m", report.Model)
	}
	if report.Capability != "" {
		cmdArgs = append(cmdArgs, "-c", report.Capability)
	}
	cmdArgs = append(cmdArgs, "-f", promptFile)

	cmd := exec.Command(invokeScript, cmdArgs...)

	// Agent runs from write/primary/ - this is its working directory
	cmd.Dir = workspace.WritePrimary

	// Grant access to the entire agent workspace root (/agent/agent-worker/)
	// The agent sees this as its world:
	//   /agent/agent-worker/
	//   ├── read/           (read-only via OS permissions)
	//   │   ├── default/    (AI-sandboxing codebase)
	//   │   └── workunit/   (report work unit files)
	//   └── write/          (writable via OS permissions)
	//       └── primary/    (working directory)
	//
	// OS-level user permissions enforce that read/ is read-only and write/ is writable.
	//
	// Note: We do NOT override HOME here. Claude and other agents need access to their
	// credentials in ~/.claude/ or similar. The working directory (cmd.Dir) is set
	// to the write space, but the agent can still access its normal config files.
	cmd.Env = append(os.Environ(),
		"AGENT_NAME="+agent,
		"AGENT_PRESET="+agent, // backwards compatibility
		"AGENT_RECORDS_PATH="+w.recordsDir,
		"REPORT_TYPE="+report.Type,
		"AGENT_ADD_DIRS="+workspace.RootPath,
	)

	// In host mode, run as restricted user if configured and we're root
	if os.Getuid() == 0 && w.agentUID != 0 {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(w.agentUID),
				Gid: uint32(w.agentGID),
			},
		}
		log.Printf("[%s] Running agent as user %s (uid=%d)", w.workerID, w.agentUser, w.agentUID)
	} else if w.agentUser != "" && w.agentUID == 0 {
		// User configured but not found - warn but continue as current user
		log.Printf("[%s] Warning: agent user '%s' not found, running as current user", w.workerID, w.agentUser)
	}

	// Reports use execute mode with AGENT_FULL_AUTO - no interactive input needed.
	// Leaving cmd.Stdin as nil (defaults to /dev/null) prevents hanging in daemon context.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("[%s] Invoking agent %s for %s report", w.workerID, agent, report.Type)
	log.Printf("[%s] Workspace: %s", w.workerID, workspace.RootPath)

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		log.Printf("Error running agent: %v", err)
		return 1
	}

	return 0
}

// writeWorkerRecord writes a record of the processed work unit
func (w *AgentWorker) writeWorkerRecord(workUnit string, inst *Instruction, agent string, startTime, endTime time.Time, exitCode int, inputPath, outputPath string) error {
	record := WorkerRecord{
		WorkerID:   w.workerID,
		WorkUnit:   workUnit,
		StartTime:  startTime.Format(time.RFC3339),
		EndTime:    endTime.Format(time.RFC3339),
		DurationMs: endTime.Sub(startTime).Milliseconds(),
		Agent:      agent,
		Mode:       inst.Mode,
		ExitCode:   exitCode,
		InputPath:  inputPath,
		OutputPath: outputPath,
	}

	recordData, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// Create filename with worker ID prefix
	recordFilename := fmt.Sprintf("%s_%s_%d.json", w.workerID, workUnit, time.Now().Unix())
	recordPath := filepath.Join(w.recordsDir, "worker", recordFilename)

	if err := os.WriteFile(recordPath, recordData, 0644); err != nil {
		return fmt.Errorf("failed to write record: %w", err)
	}

	return nil
}

// writeReportWorkerRecord writes a record of the processed report work unit
func (w *AgentWorker) writeReportWorkerRecord(workUnit string, report *Report, agent string, startTime, endTime time.Time, exitCode int, inputPath, outputPath string) error {
	record := WorkerRecord{
		WorkerID:   w.workerID,
		WorkUnit:   workUnit,
		StartTime:  startTime.Format(time.RFC3339),
		EndTime:    endTime.Format(time.RFC3339),
		DurationMs: endTime.Sub(startTime).Milliseconds(),
		Agent:      agent,
		ReportType: report.Type,
		ExitCode:   exitCode,
		InputPath:  inputPath,
		OutputPath: outputPath,
	}

	recordData, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// Create filename with worker ID prefix
	recordFilename := fmt.Sprintf("%s_%s_%d.json", w.workerID, workUnit, time.Now().Unix())
	recordPath := filepath.Join(w.recordsDir, "worker", recordFilename)

	if err := os.WriteFile(recordPath, recordData, 0644); err != nil {
		return fmt.Errorf("failed to write record: %w", err)
	}

	return nil
}

// run is the main processing loop
func (w *AgentWorker) run() {
	log.Printf("[%s] Agent worker started", w.workerID)
	log.Printf("[%s] Input: %s", w.workerID, filepath.Join(w.inputDir, "any"))
	log.Printf("[%s] Output content: %s", w.workerID, filepath.Join(w.outputDir, "content"))
	log.Printf("[%s] Output records: %s", w.workerID, filepath.Join(w.outputDir, "records"))
	log.Printf("[%s] Worker records: %s", w.workerID, filepath.Join(w.recordsDir, "worker"))
	log.Printf("[%s] Default agent: %s", w.workerID, w.currentAgent)
	if w.fileStory.Enabled() {
		log.Printf("[%s] File story logging: %s", w.workerID, os.Getenv("FILE_STORY_PATH"))
	}

	// Log enabled modes
	var enabledModes []string
	if w.instructionEnabled {
		enabledModes = append(enabledModes, "INSTRUCTION")
	}
	if w.reportEnabled {
		enabledModes = append(enabledModes, "REPORT")
	}
	if len(enabledModes) == 0 {
		log.Printf("[%s] WARNING: No work unit modes enabled! Set WORKER_INSTRUCTION_ENABLED=true or WORKER_REPORT_ENABLED=true", w.workerID)
	} else {
		log.Printf("[%s] Enabled modes: %v", w.workerID, enabledModes)
	}

	for {
		workUnits, err := w.checkForWorkUnits()
		if err != nil {
			log.Printf("[%s] Error checking for work units: %v", w.workerID, err)
		}

		if len(workUnits) > 0 {
			// Reset backoff on activity
			w.lastActivity = time.Now()
			w.backoffIndex = 0
			w.nextBackoffLog = w.lastActivity.Add(backoffLevels[0])

			for _, workUnit := range workUnits {
				var processErr error
				switch workUnit.Type {
				case WorkUnitTypeInstruction:
					processErr = w.processWorkUnit(workUnit.Path)
				case WorkUnitTypeReport:
					processErr = w.processReportWorkUnit(workUnit.Path)
				default:
					log.Printf("[%s] Unknown work unit type: %s", w.workerID, workUnit.Type)
					continue
				}
				if processErr != nil {
					log.Printf("[%s] Error processing work unit: %v", w.workerID, processErr)
				}
			}
		} else {
			// No activity - check if we should log with backoff
			now := time.Now()
			if now.After(w.nextBackoffLog) {
				timeSinceActivity := now.Sub(w.lastActivity)
				log.Printf("[%s] No new activity detected for %s", w.workerID, timeSinceActivity.Round(time.Second))

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

func main() {
	worker := NewAgentWorker()

	if err := worker.ensureDirectories(); err != nil {
		log.Fatalf("Failed to ensure directories: %v", err)
	}

	// Ensure the default read-space exists (copy of AI-sandboxing repo)
	if err := worker.ensureDefaultReadSpace(); err != nil {
		log.Fatalf("Failed to ensure default read-space: %v", err)
	}

	worker.run()
}
