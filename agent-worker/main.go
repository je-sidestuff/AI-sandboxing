package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Default paths
const (
	defaultInputDir   = "/workspaces/slopspaces/input/"
	defaultOutputDir  = "/workspaces/slopspaces/output/"
	defaultRecordsDir = "/workspaces/slopspaces/agent-records/"
	checkInterval     = 10 * time.Second
	defaultAgent      = "claude"
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

// Instruction represents the JSON structure for work instructions
type Instruction struct {
	Instruction string `json:"instruction"`
	Mode        string `json:"mode"` // "prompt" (-p) or "execute" (-e)
	Agent       string `json:"agent,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

// WorkerRecord holds metadata for processed work units
type WorkerRecord struct {
	WorkerID     string `json:"worker_id"`
	WorkUnit     string `json:"work_unit"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	DurationMs   int64  `json:"duration_ms"`
	Agent        string `json:"agent"`
	Mode         string `json:"mode"`
	ExitCode     int    `json:"exit_code"`
	InputPath    string `json:"input_path"`
	OutputPath   string `json:"output_path"`
}

// AgentWorker manages the work processing loop
type AgentWorker struct {
	workerID       string
	inputDir       string
	outputDir      string
	recordsDir     string
	currentAgent   string
	lastActivity   time.Time
	backoffIndex   int
	nextBackoffLog time.Time
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

	currentAgent := os.Getenv("AGENT_PRESET")
	if currentAgent == "" {
		currentAgent = defaultAgent
	}

	now := time.Now()
	workerID := uuid.New().String()[:8]

	return &AgentWorker{
		workerID:       workerID,
		inputDir:       inputDir,
		outputDir:      outputDir,
		recordsDir:     recordsDir,
		currentAgent:   currentAgent,
		lastActivity:   now,
		backoffIndex:   0,
		nextBackoffLog: now.Add(backoffLevels[0]),
	}
}

// ensureDirectories creates necessary directories
func (w *AgentWorker) ensureDirectories() error {
	dirs := []string{
		filepath.Join(w.inputDir, "any"),
		w.outputDir,
		filepath.Join(w.recordsDir, "worker"),
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

// checkForWorkUnits scans the input directory for work packages
func (w *AgentWorker) checkForWorkUnits() ([]string, error) {
	anyDir := filepath.Join(w.inputDir, "any")
	entries, err := os.ReadDir(anyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var workUnits []string
	for _, entry := range entries {
		if entry.IsDir() {
			folderPath := filepath.Join(anyDir, entry.Name())

			// Check if this is a work unit for us
			instructionJSON := filepath.Join(folderPath, "INSTRUCTION.json")
			instructionMD := filepath.Join(folderPath, "INSTRUCTION.md")
			processingMD := filepath.Join(folderPath, "PROCESSING.md")

			// Skip if already being processed
			if _, err := os.Stat(processingMD); err == nil {
				continue
			}

			_, jsonExists := os.Stat(instructionJSON)
			_, mdExists := os.Stat(instructionMD)

			if jsonExists == nil || mdExists == nil {
				workUnits = append(workUnits, folderPath)
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

	// Move folder to output directory
	destPath := filepath.Join(w.outputDir, folderName)
	if err := os.Rename(folderPath, destPath); err != nil {
		return fmt.Errorf("failed to move folder to output: %w", err)
	}

	// Create PROCESSED.md in the destination
	processedMD := filepath.Join(destPath, "PROCESSED.md")
	processedContent := fmt.Sprintf("# Processed\n\nWorker ID: %s\nStarted: %s\nCompleted: %s\nDuration: %s\nAgent: %s\nMode: %s\nExit Code: %d\n",
		w.workerID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
		duration.Round(time.Millisecond).String(), agent, inst.Mode, exitCode)
	if err := os.WriteFile(processedMD, []byte(processedContent), 0644); err != nil {
		log.Printf("Warning: failed to create PROCESSED.md: %v", err)
	}

	// Write worker record
	if err := w.writeWorkerRecord(folderName, inst, agent, startTime, endTime, exitCode, folderPath, destPath); err != nil {
		log.Printf("Warning: failed to write worker record: %v", err)
	}

	log.Printf("[%s] Completed work unit: %s (exit: %d, duration: %s)", w.workerID, folderName, exitCode, duration.Round(time.Millisecond))
	return nil
}

// getAgent returns the agent to use, considering instruction override
func (w *AgentWorker) getAgent(inst *Instruction) string {
	if inst.Agent != "" && isValidAgent(inst.Agent) {
		return inst.Agent
	}
	return w.currentAgent
}

// executeAgent runs the agent against the work unit
func (w *AgentWorker) executeAgent(folderPath string, inst *Instruction, agent string) int {
	// Find invoke-agent.sh
	invokeScript := findInvokeScript()
	if invokeScript == "" {
		log.Printf("Error: invoke-agent.sh not found")
		return 1
	}

	// Determine mode flag
	modeFlag := "-p"
	if inst.Mode == "execute" {
		modeFlag = "-e"
	}

	// Build command arguments
	cmdArgs := []string{modeFlag, "-a", agent, inst.Instruction}

	// Create command
	cmd := exec.Command(invokeScript, cmdArgs...)
	cmd.Dir = folderPath
	cmd.Env = append(os.Environ(),
		"AGENT_PRESET="+agent,
		"AGENT_RECORDS_PATH="+w.recordsDir,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("[%s] Invoking agent %s with mode %s", w.workerID, agent, inst.Mode)

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

// run is the main processing loop
func (w *AgentWorker) run() {
	log.Printf("[%s] Agent worker started", w.workerID)
	log.Printf("[%s] Input: %s", w.workerID, filepath.Join(w.inputDir, "any"))
	log.Printf("[%s] Output: %s", w.workerID, w.outputDir)
	log.Printf("[%s] Records: %s", w.workerID, filepath.Join(w.recordsDir, "worker"))
	log.Printf("[%s] Default agent: %s", w.workerID, w.currentAgent)

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
				if err := w.processWorkUnit(workUnit); err != nil {
					log.Printf("[%s] Error processing work unit: %v", w.workerID, err)
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

	worker.run()
}
