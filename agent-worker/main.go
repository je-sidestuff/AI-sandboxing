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

// Report represents the JSON structure for report work units
type Report struct {
	Type      string `json:"type"`                // "custom", "daily", "weekly", "monthly"
	Content   string `json:"content,omitempty"`   // For custom type: the markdown content
	Agent     string `json:"agent,omitempty"`     // Optional agent override
	Timestamp string `json:"timestamp,omitempty"` // When the report was created/converted
}

// WorkUnit represents a discovered work unit with its type
type WorkUnit struct {
	Path string
	Type string // "instruction" or "report"
}

// WorkerRecord holds metadata for processed work units
type WorkerRecord struct {
	WorkerID   string `json:"worker_id"`
	WorkUnit   string `json:"work_unit"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	DurationMs int64  `json:"duration_ms"`
	Agent      string `json:"agent"`
	Mode       string `json:"mode,omitempty"`        // For instruction work units
	ReportType string `json:"report_type,omitempty"` // For report work units: daily, weekly, monthly
	ExitCode   int    `json:"exit_code"`
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
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

	now := time.Now()
	workerID := uuid.New().String()[:8]

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
func (w *AgentWorker) checkForWorkUnits() ([]WorkUnit, error) {
	anyDir := filepath.Join(w.inputDir, "any")
	entries, err := os.ReadDir(anyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
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

	// Move folder to output directory
	destPath := filepath.Join(w.outputDir, folderName)
	if err := os.Rename(folderPath, destPath); err != nil {
		return fmt.Errorf("failed to move folder to output: %w", err)
	}

	// Create PROCESSED.md in the destination
	processedMD := filepath.Join(destPath, "PROCESSED.md")
	processedContent := fmt.Sprintf("# Processed\n\nWorker ID: %s\nStarted: %s\nCompleted: %s\nDuration: %s\nAgent: %s\nReport Type: %s\nExit Code: %d\n",
		w.workerID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
		duration.Round(time.Millisecond).String(), agent, report.Type, exitCode)
	if err := os.WriteFile(processedMD, []byte(processedContent), 0644); err != nil {
		log.Printf("Warning: failed to create PROCESSED.md: %v", err)
	}

	// Write worker record
	if err := w.writeReportWorkerRecord(folderName, report, agent, startTime, endTime, exitCode, folderPath, destPath); err != nil {
		log.Printf("Warning: failed to write worker record: %v", err)
	}

	log.Printf("[%s] Completed report work unit: %s (exit: %d, duration: %s)", w.workerID, folderName, exitCode, duration.Round(time.Millisecond))
	return nil
}

// getAgent returns the agent to use, considering instruction override
func (w *AgentWorker) getAgent(inst *Instruction) string {
	if inst.Agent != "" && isValidAgent(inst.Agent) {
		return inst.Agent
	}
	return w.currentAgent
}

// getAgentFromReport returns the agent to use, considering report override
func (w *AgentWorker) getAgentFromReport(report *Report) string {
	if report.Agent != "" && isValidAgent(report.Agent) {
		return report.Agent
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

	// Write instruction to a temp file to avoid shell argument parsing issues
	// (multi-line prompts with lines starting with "-" get misinterpreted as options)
	promptFile := filepath.Join(folderPath, ".prompt.tmp")
	if err := os.WriteFile(promptFile, []byte(inst.Instruction), 0644); err != nil {
		log.Printf("Error: failed to write prompt file: %v", err)
		return 1
	}
	defer os.Remove(promptFile)

	// Build command arguments using -f to read prompt from file
	cmdArgs := []string{modeFlag, "-a", agent, "-f", promptFile}

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
func (w *AgentWorker) executeReportAgent(folderPath string, report *Report, agent string) int {
	// Find invoke-agent.sh
	invokeScript := findInvokeScript()
	if invokeScript == "" {
		log.Printf("Error: invoke-agent.sh not found")
		return 1
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

	// Write instruction to a temp file to avoid shell argument parsing issues
	// (multi-line prompts with lines starting with "-" get misinterpreted as options)
	promptFile := filepath.Join(folderPath, ".prompt.tmp")
	if err := os.WriteFile(promptFile, []byte(instruction), 0644); err != nil {
		log.Printf("Error: failed to write prompt file: %v", err)
		return 1
	}
	defer os.Remove(promptFile)

	// Build command arguments using -f to read prompt from file
	cmdArgs := []string{modeFlag, "-a", agent, "-f", promptFile}

	// Create command
	cmd := exec.Command(invokeScript, cmdArgs...)
	cmd.Dir = folderPath
	cmd.Env = append(os.Environ(),
		"AGENT_PRESET="+agent,
		"AGENT_RECORDS_PATH="+w.recordsDir,
		"REPORT_TYPE="+report.Type,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("[%s] Invoking agent %s for %s report", w.workerID, agent, report.Type)

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
	log.Printf("[%s] Output: %s", w.workerID, w.outputDir)
	log.Printf("[%s] Records: %s", w.workerID, filepath.Join(w.recordsDir, "worker"))
	log.Printf("[%s] Default agent: %s", w.workerID, w.currentAgent)

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

	worker.run()
}
