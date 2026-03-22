package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Default paths
const (
	defaultHeuristicDir = "/workspaces/slopspaces/heuristic"
	defaultRequestDir   = "/workspaces/slopspaces/requests"
	defaultRecordsDir   = "/workspaces/slopspaces/agent-records/"
	checkInterval       = 10 * time.Second
	defaultAgent        = "claude"
)

// Available agent presets (must match invoke-agent.sh presets)
var availableAgents = []string{"copilot", "gemini", "claude", "opencode", "codex"}

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
}

// ExtractedFile represents a file extracted from agent output
type ExtractedFile struct {
	Filename string
	Content  string
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

	currentAgent := os.Getenv("AGENT_PRESET")
	if currentAgent == "" {
		currentAgent = defaultAgent
	}

	now := time.Now()
	watcherID := uuid.New().String()[:8]

	return &HeuristicWatcher{
		watcherID:      watcherID,
		heuristicDir:   heuristicDir,
		requestDir:     requestDir,
		recordsDir:     recordsDir,
		currentAgent:   currentAgent,
		lastActivity:   now,
		backoffIndex:   0,
		nextBackoffLog: now.Add(backoffLevels[0]),
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

	var units []HeuristicUnit
	for _, entry := range entries {
		if entry.IsDir() {
			folderPath := filepath.Join(pendingDir, entry.Name())

			// Skip if already being processed
			processingMD := filepath.Join(folderPath, "PROCESSING.md")
			if _, err := os.Stat(processingMD); err == nil {
				continue
			}

			// Check for HEURISTIC.md
			heuristicMD := filepath.Join(folderPath, "HEURISTIC.md")
			if _, err := os.Stat(heuristicMD); err == nil {
				content, err := os.ReadFile(heuristicMD)
				if err != nil {
					log.Printf("[%s] Warning: failed to read HEURISTIC.md in %s: %v", w.watcherID, entry.Name(), err)
					continue
				}

				units = append(units, HeuristicUnit{
					Path:       heuristicMD,
					ID:         entry.Name(),
					Content:    string(content),
					FolderPath: folderPath,
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

Analyze the heuristic input and determine whether to create a DISPATCH or INSTRUCTION request.

- **DISPATCH**: Use when the task requires dispatch-level orchestration (e.g., creating branches, PRs, multi-step workflows)
- **INSTRUCTION**: Use when the task is a direct instruction for an agent to execute

## Output Format

You MUST output exactly ONE of the following, wrapped in triple backticks with the filename:

### Option 1: DISPATCH.md
%smarkdown DISPATCH.md
# Dispatch Instructions

[Your dispatch instructions here in markdown format]
%s

### Option 2: DISPATCH.json
%sjson DISPATCH.json
{
  "type": "direct",
  "instruction": "Your instruction here",
  "mode": "execute"
}
%s

### Option 3: INSTRUCTION.md
%smarkdown INSTRUCTION.md
# Instructions

[Your instructions here in markdown format]
%s

### Option 4: INSTRUCTION.json
%sjson INSTRUCTION.json
{
  "instruction": "Your instruction here",
  "mode": "prompt"
}
%s

Choose the most appropriate format based on the heuristic input. Only output one file.
`, heuristicContent, "```", "```", "```", "```", "```", "```", "```", "```")
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
	output, exitCode, err := w.executeAgent(unit.FolderPath, prompt)
	if err != nil {
		w.markHeuristicFailed(unit, err, startTime)
		return err
	}

	// Extract files from output
	extractedFiles := extractFilesFromOutput(output)
	if len(extractedFiles) == 0 {
		err := fmt.Errorf("no valid files extracted from agent output")
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
		}
	}

	// Copy original heuristic for reference
	heuristicCopy := filepath.Join(requestFolder, "HEURISTIC_SOURCE.md")
	if err := os.WriteFile(heuristicCopy, []byte(unit.Content), 0644); err != nil {
		log.Printf("[%s] Warning: failed to copy heuristic source: %v", w.watcherID, err)
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
func (w *HeuristicWatcher) executeAgent(folderPath, prompt string) (string, int, error) {
	// Find invoke-agent.sh
	invokeScript := findInvokeScript()
	if invokeScript == "" {
		return "", 1, fmt.Errorf("invoke-agent.sh not found")
	}

	// Write prompt to a temp file
	promptFile := filepath.Join(folderPath, ".prompt.tmp")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return "", 1, fmt.Errorf("failed to write prompt file: %w", err)
	}
	defer os.Remove(promptFile)

	// Build command arguments - ALWAYS use prompt mode (-p)
	cmdArgs := []string{"-p", "-a", w.currentAgent, "-f", promptFile}

	// Create command
	cmd := exec.Command(invokeScript, cmdArgs...)
	cmd.Dir = folderPath
	cmd.Env = append(os.Environ(),
		"AGENT_PRESET="+w.currentAgent,
		"AGENT_RECORDS_PATH="+w.recordsDir,
	)

	// Capture output
	outputFile := filepath.Join(folderPath, "agent_output.txt")
	outFile, err := os.Create(outputFile)
	if err != nil {
		return "", 1, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Use a multi-writer to capture output and also display it
	cmd.Stdout = outFile
	cmd.Stderr = outFile
	cmd.Stdin = os.Stdin

	log.Printf("[%s] Invoking agent %s in prompt-only mode", w.watcherID, w.currentAgent)

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", 1, fmt.Errorf("failed to run agent: %w", err)
		}
	}

	// Read the captured output
	output, err := os.ReadFile(outputFile)
	if err != nil {
		return "", exitCode, fmt.Errorf("failed to read output file: %w", err)
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
	errStr := ""
	if processErr != nil {
		errStr = processErr.Error()
	}

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
		"error":           errStr,
	}
	_ = record

	// Use a simple format since we don't have json package yet
	recordJSON := fmt.Sprintf(
		`{"watcher_id":"%s","heuristic_id":"%s","start_time":"%s","end_time":"%s","duration_ms":%d,"agent":"%s","exit_code":%d,"request_id":"%s","files_extracted":%d,"success":%v,"error":"%s"}`,
		w.watcherID, unit.ID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
		endTime.Sub(startTime).Milliseconds(), w.currentAgent, exitCode, requestID, filesExtracted, processErr == nil, errStr,
	)

	recordFilename := fmt.Sprintf("%s_%s_%d.json", w.watcherID, unit.ID, time.Now().Unix())
	recordPath := filepath.Join(w.recordsDir, "heuristic", recordFilename)

	if err := os.WriteFile(recordPath, []byte(recordJSON), 0644); err != nil {
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
		fmt.Fprintf(os.Stderr, "  REQUEST_DIR    Directory to place extracted requests (default: %s)\n", defaultRequestDir)
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

