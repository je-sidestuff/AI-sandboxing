package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Default paths
const (
	defaultInputDir   = "/workspaces/slopspaces/input/"
	defaultOutputDir  = "/workspaces/slopspaces/output/"
	defaultRecordsDir = "/workspaces/slopspaces/agent-records/"
	pollInterval      = 500 * time.Millisecond // Fast polling for responsive dispatch
	defaultTimeout    = 30 * time.Minute       // Default timeout for dispatch operations
)

// Work unit type constants
const (
	WorkUnitTypeInstruction = "instruction"
	WorkUnitTypeReport      = "report"
)

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
	Timestamp string `json:"timestamp,omitempty"` // When the report was created
	Date      string `json:"date,omitempty"`      // For dated reports
}

// DispatchResult represents the result of a dispatched work unit
type DispatchResult struct {
	WorkUnitID   string            `json:"work_unit_id"`
	OutputPath   string            `json:"output_path"`
	Success      bool              `json:"success"`
	ExitCode     int               `json:"exit_code"`
	StartTime    time.Time         `json:"start_time"`
	EndTime      time.Time         `json:"end_time"`
	Duration     time.Duration     `json:"duration"`
	ProcessedMD  string            `json:"processed_md,omitempty"`
	OutputFiles  []string          `json:"output_files,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// DispatchRecord records a dispatch operation for persistence
type DispatchRecord struct {
	DispatcherID string `json:"dispatcher_id"`
	WorkUnitID   string `json:"work_unit_id"`
	WorkUnitType string `json:"work_unit_type"`
	DispatchTime string `json:"dispatch_time"`
	CompleteTime string `json:"complete_time,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
	InputPath    string `json:"input_path"`
	OutputPath   string `json:"output_path,omitempty"`
	Success      bool   `json:"success"`
	ExitCode     int    `json:"exit_code,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Dispatcher manages dispatching work units and collecting results
type Dispatcher struct {
	dispatcherID string
	inputDir     string
	outputDir    string
	recordsDir   string
}

// NewDispatcher creates a new dispatcher instance
func NewDispatcher() *Dispatcher {
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

	dispatcherID := uuid.New().String()[:8]

	return &Dispatcher{
		dispatcherID: dispatcherID,
		inputDir:     inputDir,
		outputDir:    outputDir,
		recordsDir:   recordsDir,
	}
}

// ensureDirectories creates necessary directories
func (d *Dispatcher) ensureDirectories() error {
	dirs := []string{
		filepath.Join(d.inputDir, "any"),
		d.outputDir,
		filepath.Join(d.recordsDir, "dispatch"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// generateWorkUnitID creates a unique work unit identifier
func generateWorkUnitID(prefix string) string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	shortID := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%s_%s", prefix, timestamp, shortID)
}

// DispatchInstruction creates an instruction work unit and waits for completion
func (d *Dispatcher) DispatchInstruction(instruction string, mode string, agent string, timeout time.Duration) (*DispatchResult, error) {
	if mode == "" {
		mode = "prompt"
	}
	if mode != "prompt" && mode != "execute" {
		return nil, fmt.Errorf("invalid mode: %s (must be 'prompt' or 'execute')", mode)
	}

	workUnitID := generateWorkUnitID("dispatch-inst")
	startTime := time.Now()

	// Create the instruction struct
	inst := Instruction{
		Instruction: instruction,
		Mode:        mode,
		Agent:       agent,
		Timestamp:   startTime.Format(time.RFC3339),
	}

	// Create the work unit
	_, err := d.createWorkUnit(workUnitID, WorkUnitTypeInstruction, &inst, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create work unit: %w", err)
	}

	log.Printf("[%s] Dispatched instruction work unit: %s", d.dispatcherID, workUnitID)

	// Wait for completion
	result, err := d.waitForCompletion(workUnitID, startTime, timeout)
	if err != nil {
		// Record the failed dispatch
		d.writeDispatchRecord(workUnitID, WorkUnitTypeInstruction, startTime, time.Now(), "", false, 0, err.Error())
		return nil, err
	}

	// Record successful dispatch
	d.writeDispatchRecord(workUnitID, WorkUnitTypeInstruction, startTime, result.EndTime, result.OutputPath, result.Success, result.ExitCode, "")

	return result, nil
}

// DispatchReport creates a report work unit and waits for completion
func (d *Dispatcher) DispatchReport(reportType string, content string, agent string, timeout time.Duration) (*DispatchResult, error) {
	validTypes := map[string]bool{"custom": true, "daily": true, "weekly": true, "monthly": true}
	if !validTypes[reportType] {
		return nil, fmt.Errorf("invalid report type: %s (must be custom, daily, weekly, or monthly)", reportType)
	}

	workUnitID := generateWorkUnitID("dispatch-report")
	startTime := time.Now()

	// Create the report struct
	report := Report{
		Type:      reportType,
		Content:   content,
		Agent:     agent,
		Timestamp: startTime.Format(time.RFC3339),
	}

	// For dated reports, set today's date
	if reportType == "daily" || reportType == "weekly" || reportType == "monthly" {
		report.Date = time.Now().Format("2006-01-02")
	}

	// Create the work unit
	_, err := d.createWorkUnit(workUnitID, WorkUnitTypeReport, nil, &report)
	if err != nil {
		return nil, fmt.Errorf("failed to create work unit: %w", err)
	}

	log.Printf("[%s] Dispatched report work unit: %s (type: %s)", d.dispatcherID, workUnitID, reportType)

	// Wait for completion
	result, err := d.waitForCompletion(workUnitID, startTime, timeout)
	if err != nil {
		d.writeDispatchRecord(workUnitID, WorkUnitTypeReport, startTime, time.Now(), "", false, 0, err.Error())
		return nil, err
	}

	d.writeDispatchRecord(workUnitID, WorkUnitTypeReport, startTime, result.EndTime, result.OutputPath, result.Success, result.ExitCode, "")

	return result, nil
}

// createWorkUnit creates the work unit folder and files in INPUT_DIR
func (d *Dispatcher) createWorkUnit(workUnitID string, workUnitType string, inst *Instruction, report *Report) (string, error) {
	workUnitPath := filepath.Join(d.inputDir, "any", workUnitID)

	if err := os.MkdirAll(workUnitPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create work unit directory: %w", err)
	}

	switch workUnitType {
	case WorkUnitTypeInstruction:
		if inst == nil {
			return "", fmt.Errorf("instruction is nil for instruction work unit")
		}
		instData, err := json.MarshalIndent(inst, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal instruction: %w", err)
		}
		instPath := filepath.Join(workUnitPath, "INSTRUCTION.json")
		if err := os.WriteFile(instPath, instData, 0644); err != nil {
			return "", fmt.Errorf("failed to write INSTRUCTION.json: %w", err)
		}

	case WorkUnitTypeReport:
		if report == nil {
			return "", fmt.Errorf("report is nil for report work unit")
		}
		reportData, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal report: %w", err)
		}
		reportPath := filepath.Join(workUnitPath, "REPORT.json")
		if err := os.WriteFile(reportPath, reportData, 0644); err != nil {
			return "", fmt.Errorf("failed to write REPORT.json: %w", err)
		}

	default:
		return "", fmt.Errorf("unknown work unit type: %s", workUnitType)
	}

	return workUnitPath, nil
}

// waitForCompletion polls OUTPUT_DIR for the completed work unit
func (d *Dispatcher) waitForCompletion(workUnitID string, startTime time.Time, timeout time.Duration) (*DispatchResult, error) {
	deadline := time.Now().Add(timeout)
	outputPath := filepath.Join(d.outputDir, workUnitID)

	for time.Now().Before(deadline) {
		// Check if work unit has appeared in output directory
		if _, err := os.Stat(outputPath); err == nil {
			// Work unit folder exists in output - check for PROCESSED.md
			processedPath := filepath.Join(outputPath, "PROCESSED.md")
			if _, err := os.Stat(processedPath); err == nil {
				// Processing complete
				return d.collectResult(workUnitID, outputPath, startTime)
			}
		}

		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("timeout waiting for work unit %s to complete after %s", workUnitID, timeout)
}

// collectResult gathers the result from a completed work unit
func (d *Dispatcher) collectResult(workUnitID string, outputPath string, startTime time.Time) (*DispatchResult, error) {
	endTime := time.Now()

	result := &DispatchResult{
		WorkUnitID: workUnitID,
		OutputPath: outputPath,
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime),
		Success:    true, // Assume success, will update based on PROCESSED.md
	}

	// Read PROCESSED.md for exit code and details
	processedPath := filepath.Join(outputPath, "PROCESSED.md")
	processedContent, err := os.ReadFile(processedPath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read PROCESSED.md: %v", err)
	} else {
		result.ProcessedMD = string(processedContent)

		// Parse exit code from PROCESSED.md
		lines := strings.Split(string(processedContent), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Exit Code:") {
				var exitCode int
				fmt.Sscanf(line, "Exit Code: %d", &exitCode)
				result.ExitCode = exitCode
				result.Success = (exitCode == 0)
				break
			}
		}
	}

	// List output files
	entries, err := os.ReadDir(outputPath)
	if err == nil {
		for _, entry := range entries {
			result.OutputFiles = append(result.OutputFiles, entry.Name())
		}
	}

	return result, nil
}

// writeDispatchRecord writes a record of the dispatch operation
func (d *Dispatcher) writeDispatchRecord(workUnitID string, workUnitType string, startTime, endTime time.Time, outputPath string, success bool, exitCode int, errMsg string) {
	record := DispatchRecord{
		DispatcherID: d.dispatcherID,
		WorkUnitID:   workUnitID,
		WorkUnitType: workUnitType,
		DispatchTime: startTime.Format(time.RFC3339),
		CompleteTime: endTime.Format(time.RFC3339),
		DurationMs:   endTime.Sub(startTime).Milliseconds(),
		InputPath:    filepath.Join(d.inputDir, "any", workUnitID),
		OutputPath:   outputPath,
		Success:      success,
		ExitCode:     exitCode,
		Error:        errMsg,
	}

	recordData, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		log.Printf("[%s] Warning: failed to marshal dispatch record: %v", d.dispatcherID, err)
		return
	}

	recordFilename := fmt.Sprintf("%s_%s_%d.json", d.dispatcherID, workUnitID, time.Now().Unix())
	recordPath := filepath.Join(d.recordsDir, "dispatch", recordFilename)

	if err := os.WriteFile(recordPath, recordData, 0644); err != nil {
		log.Printf("[%s] Warning: failed to write dispatch record: %v", d.dispatcherID, err)
	}
}

// DispatchInstructionAsync creates an instruction work unit without waiting for completion
// Returns the work unit ID that can be used to check status later
func (d *Dispatcher) DispatchInstructionAsync(instruction string, mode string, agent string) (string, error) {
	if mode == "" {
		mode = "prompt"
	}
	if mode != "prompt" && mode != "execute" {
		return "", fmt.Errorf("invalid mode: %s (must be 'prompt' or 'execute')", mode)
	}

	workUnitID := generateWorkUnitID("dispatch-inst")
	startTime := time.Now()

	inst := Instruction{
		Instruction: instruction,
		Mode:        mode,
		Agent:       agent,
		Timestamp:   startTime.Format(time.RFC3339),
	}

	_, err := d.createWorkUnit(workUnitID, WorkUnitTypeInstruction, &inst, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create work unit: %w", err)
	}

	log.Printf("[%s] Dispatched instruction work unit (async): %s", d.dispatcherID, workUnitID)
	return workUnitID, nil
}

// DispatchReportAsync creates a report work unit without waiting for completion
func (d *Dispatcher) DispatchReportAsync(reportType string, content string, agent string) (string, error) {
	validTypes := map[string]bool{"custom": true, "daily": true, "weekly": true, "monthly": true}
	if !validTypes[reportType] {
		return "", fmt.Errorf("invalid report type: %s", reportType)
	}

	workUnitID := generateWorkUnitID("dispatch-report")
	startTime := time.Now()

	report := Report{
		Type:      reportType,
		Content:   content,
		Agent:     agent,
		Timestamp: startTime.Format(time.RFC3339),
	}

	if reportType == "daily" || reportType == "weekly" || reportType == "monthly" {
		report.Date = time.Now().Format("2006-01-02")
	}

	_, err := d.createWorkUnit(workUnitID, WorkUnitTypeReport, nil, &report)
	if err != nil {
		return "", fmt.Errorf("failed to create work unit: %w", err)
	}

	log.Printf("[%s] Dispatched report work unit (async): %s", d.dispatcherID, workUnitID)
	return workUnitID, nil
}

// CheckStatus checks if a work unit has completed and returns its result if available
func (d *Dispatcher) CheckStatus(workUnitID string) (*DispatchResult, bool, error) {
	outputPath := filepath.Join(d.outputDir, workUnitID)

	// Check if work unit exists in output
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		// Still in input or being processed
		inputPath := filepath.Join(d.inputDir, "any", workUnitID)
		if _, err := os.Stat(inputPath); err == nil {
			// Still in input, check if being processed
			processingPath := filepath.Join(inputPath, "PROCESSING.md")
			if _, err := os.Stat(processingPath); err == nil {
				return nil, false, nil // In progress
			}
			return nil, false, nil // Pending
		}
		return nil, false, fmt.Errorf("work unit %s not found", workUnitID)
	}

	// Check for PROCESSED.md
	processedPath := filepath.Join(outputPath, "PROCESSED.md")
	if _, err := os.Stat(processedPath); os.IsNotExist(err) {
		return nil, false, nil // Still being processed (moved but not done)
	}

	// Collect the result
	result, err := d.collectResult(workUnitID, outputPath, time.Time{}) // StartTime unknown for async
	if err != nil {
		return nil, true, err
	}

	return result, true, nil
}

// WaitForCompletion waits for an async dispatch to complete
func (d *Dispatcher) WaitForCompletion(workUnitID string, timeout time.Duration) (*DispatchResult, error) {
	startTime := time.Now()
	return d.waitForCompletion(workUnitID, startTime, timeout)
}

func main() {
	// CLI flags
	instructionFlag := flag.String("i", "", "Instruction to dispatch")
	reportFlag := flag.String("r", "", "Report type to dispatch (custom, daily, weekly, monthly)")
	contentFlag := flag.String("c", "", "Content for custom reports")
	modeFlag := flag.String("m", "prompt", "Mode for instructions (prompt or execute)")
	agentFlag := flag.String("a", "", "Agent to use (optional)")
	timeoutFlag := flag.Duration("t", defaultTimeout, "Timeout for dispatch operation")
	asyncFlag := flag.Bool("async", false, "Dispatch asynchronously (don't wait for completion)")
	checkFlag := flag.String("check", "", "Check status of a work unit by ID")
	waitFlag := flag.String("wait", "", "Wait for a work unit to complete by ID")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "agent-dispatch: Dispatch work units to agent workers\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  agent-dispatch -i \"instruction\" [-m mode] [-a agent] [-t timeout] [--async]\n")
		fmt.Fprintf(os.Stderr, "  agent-dispatch -r type [-c content] [-a agent] [-t timeout] [--async]\n")
		fmt.Fprintf(os.Stderr, "  agent-dispatch --check <work-unit-id>\n")
		fmt.Fprintf(os.Stderr, "  agent-dispatch --wait <work-unit-id> [-t timeout]\n\n")
		fmt.Fprintf(os.Stderr, "Environment Variables:\n")
		fmt.Fprintf(os.Stderr, "  INPUT_DIR    Input directory (default: %s)\n", defaultInputDir)
		fmt.Fprintf(os.Stderr, "  OUTPUT_DIR   Output directory (default: %s)\n", defaultOutputDir)
		fmt.Fprintf(os.Stderr, "  RECORDS_DIR  Records directory (default: %s)\n\n", defaultRecordsDir)
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	dispatcher := NewDispatcher()

	if err := dispatcher.ensureDirectories(); err != nil {
		log.Fatalf("Failed to ensure directories: %v", err)
	}

	// Handle check status
	if *checkFlag != "" {
		result, completed, err := dispatcher.CheckStatus(*checkFlag)
		if err != nil {
			log.Fatalf("Error checking status: %v", err)
		}
		if !completed {
			fmt.Printf("Work unit %s is still pending or in progress\n", *checkFlag)
			os.Exit(1)
		}
		printResult(result)
		return
	}

	// Handle wait for completion
	if *waitFlag != "" {
		log.Printf("[%s] Waiting for work unit: %s", dispatcher.dispatcherID, *waitFlag)
		result, err := dispatcher.WaitForCompletion(*waitFlag, *timeoutFlag)
		if err != nil {
			log.Fatalf("Error waiting for completion: %v", err)
		}
		printResult(result)
		return
	}

	// Handle instruction dispatch
	if *instructionFlag != "" {
		if *asyncFlag {
			workUnitID, err := dispatcher.DispatchInstructionAsync(*instructionFlag, *modeFlag, *agentFlag)
			if err != nil {
				log.Fatalf("Failed to dispatch instruction: %v", err)
			}
			fmt.Printf("Dispatched work unit: %s\n", workUnitID)
			fmt.Printf("Check status with: agent-dispatch --check %s\n", workUnitID)
			fmt.Printf("Wait for completion with: agent-dispatch --wait %s\n", workUnitID)
			return
		}

		log.Printf("[%s] Dispatching instruction (mode: %s, timeout: %s)", dispatcher.dispatcherID, *modeFlag, *timeoutFlag)
		result, err := dispatcher.DispatchInstruction(*instructionFlag, *modeFlag, *agentFlag, *timeoutFlag)
		if err != nil {
			log.Fatalf("Dispatch failed: %v", err)
		}
		printResult(result)
		return
	}

	// Handle report dispatch
	if *reportFlag != "" {
		if *reportFlag == "custom" && *contentFlag == "" {
			log.Fatalf("Custom reports require -c content flag")
		}

		if *asyncFlag {
			workUnitID, err := dispatcher.DispatchReportAsync(*reportFlag, *contentFlag, *agentFlag)
			if err != nil {
				log.Fatalf("Failed to dispatch report: %v", err)
			}
			fmt.Printf("Dispatched work unit: %s\n", workUnitID)
			fmt.Printf("Check status with: agent-dispatch --check %s\n", workUnitID)
			fmt.Printf("Wait for completion with: agent-dispatch --wait %s\n", workUnitID)
			return
		}

		log.Printf("[%s] Dispatching report (type: %s, timeout: %s)", dispatcher.dispatcherID, *reportFlag, *timeoutFlag)
		result, err := dispatcher.DispatchReport(*reportFlag, *contentFlag, *agentFlag, *timeoutFlag)
		if err != nil {
			log.Fatalf("Dispatch failed: %v", err)
		}
		printResult(result)
		return
	}

	// No action specified
	flag.Usage()
	os.Exit(1)
}

// printResult outputs the dispatch result
func printResult(result *DispatchResult) {
	fmt.Println("\n=== Dispatch Result ===")
	fmt.Printf("Work Unit ID: %s\n", result.WorkUnitID)
	fmt.Printf("Output Path:  %s\n", result.OutputPath)
	fmt.Printf("Success:      %v\n", result.Success)
	fmt.Printf("Exit Code:    %d\n", result.ExitCode)
	fmt.Printf("Duration:     %s\n", result.Duration.Round(time.Millisecond))

	if len(result.OutputFiles) > 0 {
		fmt.Printf("Output Files: %v\n", result.OutputFiles)
	}

	if result.Error != "" {
		fmt.Printf("Error:        %s\n", result.Error)
	}

	if !result.Success {
		os.Exit(1)
	}
}
