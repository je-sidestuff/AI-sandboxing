package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"agent-dispatch/prpoller"

	"github.com/google/uuid"
)

// Default paths
const (
	defaultInputDir        = "/workspaces/slopspaces/input/"
	defaultOutputDir       = "/workspaces/slopspaces/output/"
	defaultRecordsDir      = "/workspaces/slopspaces/agent-records/"
	defaultDispatcherLive  = "/workspaces/slopspaces/dispatcher/live"
	pollInterval           = 500 * time.Millisecond // Fast polling for responsive dispatch
	checkInterval          = 10 * time.Second       // Watch mode check interval
	defaultTimeout         = 30 * time.Minute       // Default timeout for dispatch operations
	defaultTerraformBinary = "terraform"
)

// Work unit type constants
const (
	WorkUnitTypeInstruction = "instruction"
	WorkUnitTypeReport      = "report"
)

// Dispatch type constants
const (
	DispatchTypeDirect        = "direct"
	DispatchTypeInRepo        = "in-repo"
	DispatchTypeRepoIsolation = "repo-isolation"
	DispatchTypeApproval      = "approval"
)

// Conclusion state constants for isolation PR
const (
	ConclusionStateActive = "active"
	ConclusionStateClosed = "closed"
	ConclusionStateMerged = "merged"
)

// Reintegration conclusion state constants
const (
	ReintegrationStateNone   = "none"   // Reintegration PR not created yet
	ReintegrationStateActive = "active" // Reintegration PR is open
	ReintegrationStateClosed = "closed" // Reintegration PR was closed without merge
	ReintegrationStateMerged = "merged" // Reintegration PR was merged
)

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
	Timestamp string `json:"timestamp,omitempty"` // When the report was created
	Date      string `json:"date,omitempty"`      // For dated reports
}

// Dispatch represents the JSON structure for dispatch work units (watch mode)
type Dispatch struct {
	Type            string            `json:"type"`                      // "direct", "in-repo", "repo-isolation" (NOT "approval" - approval is automatic)
	Instruction     string            `json:"instruction"`               // The instruction to dispatch
	Mode            string            `json:"mode,omitempty"`            // "prompt" or "execute" (default: "execute")
	Agent           string            `json:"agent,omitempty"`           // Optional agent override
	TargetRepo      string            `json:"target_repo,omitempty"`     // For in-repo/repo-isolation: "owner/repo"
	PRTitle         string            `json:"pr_title,omitempty"`        // For in-repo/repo-isolation: optional PR title
	PRBody          string            `json:"pr_body,omitempty"`         // For in-repo/repo-isolation: optional PR body
	IsolationName   string            `json:"isolation_name,omitempty"`  // For repo-isolation: name of isolation repo to create
	ApprovalRepo    string            `json:"approval_repo,omitempty"`   // "owner/repo" of approval repository (default: sloppo)
	SourceContext   string            `json:"source_context,omitempty"`  // Description of request origin
	SkipApproval    bool              `json:"skip_approval,omitempty"`   // If true, skip the approval gate (use with caution!)
	Timestamp       string            `json:"timestamp,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"` // Additional metadata
}

// DispatchResult represents the result of a dispatched work unit
type DispatchResult struct {
	WorkUnitID  string        `json:"work_unit_id"`
	OutputPath  string        `json:"output_path"`
	Success     bool          `json:"success"`
	ExitCode    int           `json:"exit_code"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	ProcessedMD string        `json:"processed_md,omitempty"`
	OutputFiles []string      `json:"output_files,omitempty"`
	Error       string        `json:"error,omitempty"`
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

// DispatchUnit represents a discovered dispatch work unit (watch mode)
type DispatchUnit struct {
	Path     string
	ID       string
	Dispatch *Dispatch
}

// FlowRecord holds metadata for tracked terraform flows
type FlowRecord struct {
	DispatcherID       string    `json:"dispatcher_id"`
	FlowID             string    `json:"flow_id"`
	DispatchType       string    `json:"dispatch_type"`
	DispatchPath       string    `json:"dispatch_path"`
	TFConfigDir        string    `json:"tf_config_dir,omitempty"`
	StartTime          string    `json:"start_time"`
	EndTime            string    `json:"end_time,omitempty"`
	Status             string    `json:"status"` // "pending", "running", "monitoring", "completed", "failed"
	Error              string    `json:"error,omitempty"`
	PRUrl              string    `json:"pr_url,omitempty"`
	ConclusionState    string    `json:"conclusion_state,omitempty"`   // "active", "closed", "merged"
	NeedsMonitoring    bool      `json:"needs_monitoring,omitempty"`   // true if flow should be polled for conclusion state
	LastPollTime       string    `json:"last_poll_time,omitempty"`
	ReintegrationURL   string    `json:"reintegration_url,omitempty"`   // For repo-isolation: URL of reintegration PR
	PendingInstruction string    `json:"pending_instruction,omitempty"` // For approval (legacy): instruction to execute when approved
	PendingMode        string    `json:"pending_mode,omitempty"`        // For approval (legacy): mode for pending instruction
	PendingAgent       string    `json:"pending_agent,omitempty"`       // For approval (legacy): agent override for pending instruction
	PendingDispatch    *Dispatch `json:"pending_dispatch,omitempty"`    // For approval-gated flows: full dispatch to execute after approval
}

// Dispatcher manages dispatching work units and collecting results
type Dispatcher struct {
	dispatcherID    string
	inputDir        string
	outputDir       string
	recordsDir      string
	dispatcherLive  string
	terraformBinary string
	githubPAT       string
	githubOwner     string // For repo-isolation: the owner for isolation repos
	lastActivity    time.Time
	backoffIndex    int
	nextBackoffLog  time.Time
	prPoller        *prpoller.Poller // centralized PR comment poller

	// Change tracking for smart polling
	flowChanges      map[string]bool // flowID -> hasChanges; set by PR poller, cleared after terraform apply
	flowChangesMu    sync.Mutex
	lastPeriodicPoll time.Time         // last time we did a full periodic poll
	periodicInterval time.Duration     // how often to poll even without changes (default 5 min)
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

	dispatcherLive := os.Getenv("DISPATCHER_LIVE")
	if dispatcherLive == "" {
		dispatcherLive = defaultDispatcherLive
	}

	terraformBinary := os.Getenv("TERRAFORM_BINARY")
	if terraformBinary == "" {
		terraformBinary = defaultTerraformBinary
	}

	githubPAT := os.Getenv("GITHUB_PAT")
	if githubPAT == "" {
		githubPAT = os.Getenv("GH_TOKEN")
	}

	githubOwner := os.Getenv("GITHUB_OWNER")
	if githubOwner == "" {
		githubOwner = "je-sidestuff" // Default owner for isolation repos
	}

	now := time.Now()
	dispatcherID := uuid.New().String()[:8]

	d := &Dispatcher{
		dispatcherID:     dispatcherID,
		inputDir:         inputDir,
		outputDir:        outputDir,
		recordsDir:       recordsDir,
		dispatcherLive:   dispatcherLive,
		terraformBinary:  terraformBinary,
		githubPAT:        githubPAT,
		githubOwner:      githubOwner,
		lastActivity:     now,
		backoffIndex:     0,
		nextBackoffLog:   now.Add(backoffLevels[0]),
		flowChanges:      make(map[string]bool),
		lastPeriodicPoll: now,
		periodicInterval: 1 * time.Minute, // Fallback poll every 1 minute even without detected changes
	}

	// Initialize the PR poller for monitoring active flows
	if githubPAT != "" {
		d.prPoller = prpoller.NewPoller(prpoller.Config{
			Interval: 30 * time.Second,
			Token:    githubPAT,
			OnChange: func(event prpoller.ChangeEvent) {
				log.Printf("[%s] PR activity detected on %s/%s#%d: %d new comment(s)",
					dispatcherID, event.PR.Owner, event.PR.Repo, event.PR.Number, len(event.NewComments))
				for _, c := range event.NewComments {
					log.Printf("[%s]   - @%s: %.80s", dispatcherID, c.Author, c.Body)
				}
				// Mark all flows for this PR as needing terraform apply
				d.markFlowChanged(event.PR.Owner, event.PR.Repo, event.PR.Number)
			},
		})
	}

	return d
}

// markFlowChanged marks flows associated with a PR as needing terraform apply
// This is called by the PR poller when new comments are detected
func (d *Dispatcher) markFlowChanged(owner, repo string, prNumber int) {
	d.flowChangesMu.Lock()
	defer d.flowChangesMu.Unlock()

	// Find flows that match this PR by scanning flow records
	flows, err := d.loadMonitoringFlows()
	if err != nil {
		return
	}

	prKey := fmt.Sprintf("%s/%s#%d", owner, repo, prNumber)
	for _, flow := range flows {
		// Check if this flow's PR URL matches
		if flow.PRUrl != "" && strings.Contains(flow.PRUrl, fmt.Sprintf("%s/%s/pull/%d", owner, repo, prNumber)) {
			d.flowChanges[flow.FlowID] = true
			log.Printf("[%s] Marked flow %s for terraform apply (PR activity on %s)", d.dispatcherID, flow.FlowID, prKey)
		}
	}
}

// hasFlowChanges checks if a flow has pending changes and clears the flag
func (d *Dispatcher) hasFlowChanges(flowID string) bool {
	d.flowChangesMu.Lock()
	defer d.flowChangesMu.Unlock()
	if d.flowChanges[flowID] {
		delete(d.flowChanges, flowID)
		return true
	}
	return false
}

// ensureDirectories creates necessary directories
func (d *Dispatcher) ensureDirectories() error {
	dirs := []string{
		filepath.Join(d.inputDir, "any"),
		filepath.Join(d.outputDir, "content"),
		filepath.Join(d.outputDir, "records"),
		filepath.Join(d.recordsDir, "dispatch"),
		filepath.Join(d.recordsDir, "dispatch-watch"),
		filepath.Join(d.dispatcherLive, "flows", "in-repo"),
		filepath.Join(d.dispatcherLive, "flows", "repo-isolation"),
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

// generateFlowID creates a unique flow identifier
func generateFlowID(dispatchType string) string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	shortID := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%s_%s", dispatchType, timestamp, shortID)
}

// =====================================================================
// Single-shot dispatch mode (--once)
// =====================================================================

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
		d.writeDispatchRecordOnce(workUnitID, WorkUnitTypeInstruction, startTime, time.Now(), "", false, 0, err.Error())
		return nil, err
	}

	// Record successful dispatch
	d.writeDispatchRecordOnce(workUnitID, WorkUnitTypeInstruction, startTime, result.EndTime, result.OutputPath, result.Success, result.ExitCode, "")

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
		d.writeDispatchRecordOnce(workUnitID, WorkUnitTypeReport, startTime, time.Now(), "", false, 0, err.Error())
		return nil, err
	}

	d.writeDispatchRecordOnce(workUnitID, WorkUnitTypeReport, startTime, result.EndTime, result.OutputPath, result.Success, result.ExitCode, "")

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
	// Content now goes to output/content/<work-name>
	contentPath := filepath.Join(d.outputDir, "content", workUnitID)

	for time.Now().Before(deadline) {
		// Check if work unit has appeared in content directory
		if _, err := os.Stat(contentPath); err == nil {
			// Work unit folder exists in content - check for PROCESSED-*.md files
			entries, err := os.ReadDir(contentPath)
			if err == nil {
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), "PROCESSED-") && strings.HasSuffix(entry.Name(), ".md") {
						// Processing complete
						return d.collectResult(workUnitID, contentPath, startTime)
					}
				}
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
		Success:    true, // Assume success, will update based on PROCESSED-*.md
	}

	// Find and read PROCESSED-*.md for exit code and details
	entries, err := os.ReadDir(outputPath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read output directory: %v", err)
		return result, nil
	}

	var processedContent []byte
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "PROCESSED-") && strings.HasSuffix(entry.Name(), ".md") {
			processedPath := filepath.Join(outputPath, entry.Name())
			processedContent, err = os.ReadFile(processedPath)
			if err != nil {
				result.Error = fmt.Sprintf("failed to read %s: %v", entry.Name(), err)
			} else {
				result.ProcessedMD = string(processedContent)

				// Parse exit code from PROCESSED-*.md
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
			break
		}
	}

	// List output files
	for _, entry := range entries {
		result.OutputFiles = append(result.OutputFiles, entry.Name())
	}

	return result, nil
}

// writeDispatchRecordOnce writes a record of the single-shot dispatch operation
func (d *Dispatcher) writeDispatchRecordOnce(workUnitID string, workUnitType string, startTime, endTime time.Time, outputPath string, success bool, exitCode int, errMsg string) {
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
	// Content now goes to output/content/<work-name>
	contentPath := filepath.Join(d.outputDir, "content", workUnitID)

	// Check if work unit exists in content directory
	if _, err := os.Stat(contentPath); os.IsNotExist(err) {
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

	// Check for PROCESSED-*.md files
	entries, err := os.ReadDir(contentPath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read content directory: %v", err)
	}

	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "PROCESSED-") && strings.HasSuffix(entry.Name(), ".md") {
			found = true
			break
		}
	}

	if !found {
		return nil, false, nil // Still being processed (moved but not done)
	}

	// Collect the result
	result, err := d.collectResult(workUnitID, contentPath, time.Time{}) // StartTime unknown for async
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

// =====================================================================
// Watch mode (default behavior)
// =====================================================================

// checkForDispatchUnits scans the input directory for DISPATCH.json/md files
func (d *Dispatcher) checkForDispatchUnits() ([]DispatchUnit, error) {
	anyDir := filepath.Join(d.inputDir, "any")
	entries, err := os.ReadDir(anyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var dispatchUnits []DispatchUnit
	for _, entry := range entries {
		if entry.IsDir() {
			folderPath := filepath.Join(anyDir, entry.Name())

			// Skip if already being dispatched
			dispatchingMD := filepath.Join(folderPath, "DISPATCHING.md")
			if _, err := os.Stat(dispatchingMD); err == nil {
				continue
			}

			// Check for DISPATCH.json or DISPATCH.md
			dispatchJSON := filepath.Join(folderPath, "DISPATCH.json")
			dispatchMD := filepath.Join(folderPath, "DISPATCH.md")

			_, jsonExists := os.Stat(dispatchJSON)
			_, mdExists := os.Stat(dispatchMD)

			if jsonExists == nil || mdExists == nil {
				dispatch, err := d.handleDispatchFiles(folderPath)
				if err != nil {
					log.Printf("[%s] Error handling dispatch files in %s: %v", d.dispatcherID, entry.Name(), err)
					continue
				}
				dispatchUnits = append(dispatchUnits, DispatchUnit{
					Path:     folderPath,
					ID:       entry.Name(),
					Dispatch: dispatch,
				})
			}
		}
	}

	return dispatchUnits, nil
}

// handleDispatchFiles processes DISPATCH.json/md files, converting .md to .json if needed
func (d *Dispatcher) handleDispatchFiles(folderPath string) (*Dispatch, error) {
	dispatchJSON := filepath.Join(folderPath, "DISPATCH.json")
	dispatchMD := filepath.Join(folderPath, "DISPATCH.md")

	_, jsonExists := os.Stat(dispatchJSON)
	_, mdExists := os.Stat(dispatchMD)

	// If DISPATCH.json exists, use it (takes precedence)
	if jsonExists == nil {
		// Delete DISPATCH.md if it exists (to show it was ignored)
		if mdExists == nil {
			if err := os.Remove(dispatchMD); err != nil {
				log.Printf("Warning: failed to remove DISPATCH.md: %v", err)
			}
		}

		// Read and parse the JSON
		data, err := os.ReadFile(dispatchJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to read DISPATCH.json: %w", err)
		}

		var dispatch Dispatch
		if err := json.Unmarshal(data, &dispatch); err != nil {
			return nil, fmt.Errorf("failed to parse DISPATCH.json: %w", err)
		}

		// Validate type
		if dispatch.Type != DispatchTypeDirect && dispatch.Type != DispatchTypeInRepo && dispatch.Type != DispatchTypeRepoIsolation && dispatch.Type != DispatchTypeApproval {
			return nil, fmt.Errorf("invalid dispatch type: %s (must be 'direct', 'in-repo', 'repo-isolation', or 'approval')", dispatch.Type)
		}

		// Default mode to execute
		if dispatch.Mode == "" {
			dispatch.Mode = "execute"
		}

		return &dispatch, nil
	}

	// If only DISPATCH.md exists, convert it to DISPATCH.json with type "direct"
	if mdExists == nil {
		// Read the markdown content
		mdContent, err := os.ReadFile(dispatchMD)
		if err != nil {
			return nil, fmt.Errorf("failed to read DISPATCH.md: %w", err)
		}

		// Create the dispatch struct with type "direct" (auto-transform behavior)
		dispatch := Dispatch{
			Type:        DispatchTypeDirect,
			Instruction: string(mdContent),
			Mode:        "execute", // Default to execute mode
			Timestamp:   time.Now().Format(time.RFC3339),
		}

		// Write the JSON file
		jsonData, err := json.MarshalIndent(dispatch, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal DISPATCH.json: %w", err)
		}

		if err := os.WriteFile(dispatchJSON, jsonData, 0644); err != nil {
			return nil, fmt.Errorf("failed to write DISPATCH.json: %w", err)
		}

		// Remove the original DISPATCH.md (it's been converted)
		if err := os.Remove(dispatchMD); err != nil {
			log.Printf("Warning: failed to remove DISPATCH.md after conversion: %v", err)
		}

		log.Printf("[%s] Converted DISPATCH.md to DISPATCH.json (type: direct)", d.dispatcherID)
		return &dispatch, nil
	}

	return nil, fmt.Errorf("no dispatch file found")
}

// processDispatchUnit handles a single dispatch work unit
func (d *Dispatcher) processDispatchUnit(unit DispatchUnit) error {
	log.Printf("[%s] Processing dispatch unit: %s (type: %s)", d.dispatcherID, unit.ID, unit.Dispatch.Type)

	startTime := time.Now()

	// Create DISPATCHING.md to mark we're working on it
	dispatchingMD := filepath.Join(unit.Path, "DISPATCHING.md")
	dispatchingContent := fmt.Sprintf("# Dispatching\n\nWatcher ID: %s\nStarted: %s\nType: %s\n",
		d.dispatcherID, startTime.Format(time.RFC3339), unit.Dispatch.Type)
	if err := os.WriteFile(dispatchingMD, []byte(dispatchingContent), 0644); err != nil {
		return fmt.Errorf("failed to create DISPATCHING.md: %w", err)
	}

	// Option B: ALL dispatches require approval by default unless SkipApproval is true
	// The only exception is type="approval" which is already an approval flow (to prevent infinite loops)
	if !unit.Dispatch.SkipApproval && unit.Dispatch.Type != DispatchTypeApproval {
		log.Printf("[%s] Dispatch requires approval gate (type: %s, skip_approval: %v)", d.dispatcherID, unit.Dispatch.Type, unit.Dispatch.SkipApproval)
		return d.createApprovalGatedDispatch(unit)
	}

	var err error
	switch unit.Dispatch.Type {
	case DispatchTypeDirect:
		err = d.processDirectDispatch(unit)
	case DispatchTypeInRepo:
		err = d.processInRepoDispatch(unit)
	case DispatchTypeRepoIsolation:
		err = d.processRepoIsolationDispatch(unit)
	case DispatchTypeApproval:
		// Legacy: explicit approval type (for backwards compatibility)
		err = d.processApprovalDispatch(unit)
	default:
		err = fmt.Errorf("unsupported dispatch type: %s", unit.Dispatch.Type)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	if err != nil {
		// Mark as failed
		d.markDispatchFailed(unit, err, startTime, endTime)
		return err
	}

	// Mark as completed and move to output
	d.markDispatchComplete(unit, startTime, endTime)

	log.Printf("[%s] Completed dispatch unit: %s (duration: %s)", d.dispatcherID, unit.ID, duration.Round(time.Millisecond))
	return nil
}

// processDirectDispatch handles direct dispatch (creates INSTRUCTION.json in-place)
func (d *Dispatcher) processDirectDispatch(unit DispatchUnit) error {
	log.Printf("[%s] Processing direct dispatch: %s", d.dispatcherID, unit.ID)

	// For direct dispatch, we transform the dispatch into an instruction
	// and let the agent-worker pick it up from the same location

	inst := Instruction{
		Instruction: unit.Dispatch.Instruction,
		Mode:        unit.Dispatch.Mode,
		Agent:       unit.Dispatch.Agent,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	instData, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal instruction: %w", err)
	}

	// Write INSTRUCTION.json to the same folder
	instPath := filepath.Join(unit.Path, "INSTRUCTION.json")
	if err := os.WriteFile(instPath, instData, 0644); err != nil {
		return fmt.Errorf("failed to write INSTRUCTION.json: %w", err)
	}

	// Remove DISPATCH.json and DISPATCHING.md so worker picks up the INSTRUCTION
	os.Remove(filepath.Join(unit.Path, "DISPATCH.json"))
	os.Remove(filepath.Join(unit.Path, "DISPATCHING.md"))

	log.Printf("[%s] Direct dispatch transformed to INSTRUCTION.json, ready for worker pickup", d.dispatcherID)
	return nil
}

// processInRepoDispatch handles in-repo dispatch with terraform lifecycle
func (d *Dispatcher) processInRepoDispatch(unit DispatchUnit) error {
	log.Printf("[%s] Processing in-repo dispatch: %s", d.dispatcherID, unit.ID)

	if d.githubPAT == "" {
		return fmt.Errorf("GITHUB_PAT or GH_TOKEN environment variable is required for in-repo dispatch")
	}

	targetRepo := unit.Dispatch.TargetRepo
	if targetRepo == "" {
		targetRepo = "je-sidestuff/AI-sandboxing" // Default
	}

	// Generate a unique flow ID
	flowID := generateFlowID(DispatchTypeInRepo)

	// Create the terraform config directory
	tfConfigDir := filepath.Join(d.dispatcherLive, "flows", "in-repo", flowID)
	if err := os.MkdirAll(tfConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create terraform config directory: %w", err)
	}

	// Write the flow record
	flowRecord := FlowRecord{
		DispatcherID: d.dispatcherID,
		FlowID:       flowID,
		DispatchType: DispatchTypeInRepo,
		DispatchPath: unit.Path,
		TFConfigDir:  tfConfigDir,
		StartTime:    time.Now().Format(time.RFC3339),
		Status:       "running",
	}
	d.writeFlowRecord(flowRecord)

	// Create the terraform configuration
	if err := d.createInRepoTerraformConfig(tfConfigDir, flowID, targetRepo, unit.Dispatch); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = err.Error()
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("failed to create terraform config: %w", err)
	}

	// Run terraform init
	log.Printf("[%s] Running terraform init in %s", d.dispatcherID, tfConfigDir)
	if err := d.runTerraform(tfConfigDir, "init"); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = fmt.Sprintf("terraform init failed: %v", err)
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Run terraform apply
	log.Printf("[%s] Running terraform apply in %s", d.dispatcherID, tfConfigDir)
	if err := d.runTerraform(tfConfigDir, "apply", "-auto-approve"); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = fmt.Sprintf("terraform apply failed: %v", err)
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	// Initial terraform apply complete - PR has been created and work dispatched
	// Now enter monitoring phase to watch for PR merge/close
	flowRecord.Status = "monitoring"
	flowRecord.NeedsMonitoring = true
	flowRecord.LastPollTime = time.Now().Format(time.RFC3339)

	// Get the initial conclusion state
	conclusionState, _ := d.getTerraformOutput(tfConfigDir, "conclusion_state")
	flowRecord.ConclusionState = conclusionState

	// Try to get PR URL from terraform output
	prURL, _ := d.getTerraformOutput(tfConfigDir, "pr_url")
	if prURL != "" {
		flowRecord.PRUrl = prURL
		log.Printf("[%s] In-repo dispatch created, PR URL: %s (entering monitoring)", d.dispatcherID, prURL)
	}

	d.writeFlowRecord(flowRecord)

	log.Printf("[%s] In-repo dispatch flow %s now monitoring for conclusion", d.dispatcherID, flowID)
	return nil
}

// processRepoIsolationDispatch handles repo-isolation dispatch with terraform lifecycle
// This creates a completely separate private repository for the AI to work in
func (d *Dispatcher) processRepoIsolationDispatch(unit DispatchUnit) error {
	log.Printf("[%s] Processing repo-isolation dispatch: %s", d.dispatcherID, unit.ID)

	if d.githubPAT == "" {
		return fmt.Errorf("GITHUB_PAT or GH_TOKEN environment variable is required for repo-isolation dispatch")
	}

	targetRepo := unit.Dispatch.TargetRepo
	if targetRepo == "" {
		targetRepo = "je-sidestuff/AI-sandboxing" // Default
	}

	// Generate isolation repo name if not specified
	isolationName := unit.Dispatch.IsolationName
	if isolationName == "" {
		// Generate a unique name based on flow ID
		timestamp := time.Now().Format("20060102-150405")
		shortID := uuid.New().String()[:8]
		isolationName = fmt.Sprintf("ai-isolation-%s-%s", timestamp, shortID)
	}

	// Generate a unique flow ID
	flowID := generateFlowID(DispatchTypeRepoIsolation)

	// Create the terraform config directory
	tfConfigDir := filepath.Join(d.dispatcherLive, "flows", "repo-isolation", flowID)
	if err := os.MkdirAll(tfConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create terraform config directory: %w", err)
	}

	// Write the flow record
	flowRecord := FlowRecord{
		DispatcherID: d.dispatcherID,
		FlowID:       flowID,
		DispatchType: DispatchTypeRepoIsolation,
		DispatchPath: unit.Path,
		TFConfigDir:  tfConfigDir,
		StartTime:    time.Now().Format(time.RFC3339),
		Status:       "running",
	}
	d.writeFlowRecord(flowRecord)

	// Create the terraform configuration
	if err := d.createRepoIsolationTerraformConfig(tfConfigDir, flowID, targetRepo, isolationName, unit.Dispatch); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = err.Error()
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("failed to create terraform config: %w", err)
	}

	// Run terraform init
	log.Printf("[%s] Running terraform init in %s", d.dispatcherID, tfConfigDir)
	if err := d.runTerraform(tfConfigDir, "init"); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = fmt.Sprintf("terraform init failed: %v", err)
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Run terraform apply
	log.Printf("[%s] Running terraform apply in %s", d.dispatcherID, tfConfigDir)
	if err := d.runTerraform(tfConfigDir, "apply", "-auto-approve"); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = fmt.Sprintf("terraform apply failed: %v", err)
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	// Initial terraform apply complete - PR has been created in isolation repo
	// Now enter monitoring phase to watch for PR merge/close
	flowRecord.Status = "monitoring"
	flowRecord.NeedsMonitoring = true
	flowRecord.LastPollTime = time.Now().Format(time.RFC3339)

	// Get the initial conclusion state
	conclusionState, _ := d.getTerraformOutput(tfConfigDir, "conclusion_state")
	flowRecord.ConclusionState = conclusionState

	// Try to get branch name from terraform output (repo-isolation outputs this)
	branchName, _ := d.getTerraformOutput(tfConfigDir, "branch_name")
	if branchName != "" {
		log.Printf("[%s] Repo-isolation dispatch created, isolation repo: %s, branch: %s", d.dispatcherID, isolationName, branchName)
	}

	// Get the PR URL and register for monitoring
	prURL, _ := d.getTerraformOutput(tfConfigDir, "pr_url")
	if prURL != "" {
		flowRecord.PRUrl = prURL
		log.Printf("[%s] Containment PR: %s (entering monitoring)", d.dispatcherID, prURL)

		// Register the PR for monitoring with the poller
		if d.prPoller != nil {
			repoFullName, _ := d.getTerraformOutput(tfConfigDir, "isolation_repo_full_name")
			prNumberStr, _ := d.getTerraformOutput(tfConfigDir, "pr_number")

			if repoFullName != "" && prNumberStr != "" {
				var prNumber int
				fmt.Sscanf(prNumberStr, "%d", &prNumber)

				parts := strings.SplitN(repoFullName, "/", 2)
				if len(parts) == 2 && prNumber > 0 {
					owner, repo := parts[0], parts[1]
					d.prPoller.Register(prpoller.PRRegistration{
						Owner:  owner,
						Repo:   repo,
						Number: prNumber,
						// NOTE: TerraformAction is intentionally NOT set here.
						// The pollAllMonitoringFlows loop already runs terraform apply every 10s,
						// so we don't need the PR poller to also trigger terraform - that causes
						// state lock conflicts. The PR poller is used only for logging/awareness.
						OnChange: func(event prpoller.ChangeEvent) {
							log.Printf("[%s] Detected %d new comment(s) on %s (will be processed by monitoring loop)",
								d.dispatcherID, len(event.NewComments), prURL)
						},
					})
					log.Printf("[%s] Registered PR %s/%s#%d for comment monitoring", d.dispatcherID, owner, repo, prNumber)
				}
			}
		}
	}

	d.writeFlowRecord(flowRecord)

	log.Printf("[%s] Repo-isolation dispatch flow %s now monitoring for conclusion", d.dispatcherID, flowID)
	return nil
}

// createApprovalGatedDispatch gates a dispatch through the approval flow
// This is the default behavior for all dispatches (Option B: ALL dispatches require approval)
// After approval PR merges, the dispatch will be re-emitted with SkipApproval=true
func (d *Dispatcher) createApprovalGatedDispatch(unit DispatchUnit) error {
	log.Printf("[%s] Creating approval-gated dispatch for: %s (type: %s)", d.dispatcherID, unit.ID, unit.Dispatch.Type)

	if d.githubPAT == "" {
		return fmt.Errorf("GITHUB_PAT or GH_TOKEN environment variable is required for approval-gated dispatch")
	}

	// Get approval repo (default to sloppo)
	approvalRepo := unit.Dispatch.ApprovalRepo
	if approvalRepo == "" {
		approvalRepo = os.Getenv("APPROVAL_REPO")
	}
	if approvalRepo == "" {
		approvalRepo = "je-sidestuff/sloppo"
	}

	// Parse owner/repo if full name provided, otherwise use githubOwner
	var approvalRepoName string
	var approvalOwner string
	if strings.Contains(approvalRepo, "/") {
		parts := strings.SplitN(approvalRepo, "/", 2)
		approvalOwner = parts[0]
		approvalRepoName = parts[1]
	} else {
		approvalOwner = d.githubOwner
		approvalRepoName = approvalRepo
	}

	// Get source context
	sourceContext := unit.Dispatch.SourceContext
	if sourceContext == "" {
		sourceContext = fmt.Sprintf("%s dispatch", unit.Dispatch.Type)
	}

	// Generate a unique flow ID
	flowID := generateFlowID(DispatchTypeApproval)

	// Create the terraform config directory
	tfConfigDir := filepath.Join(d.dispatcherLive, "flows", "approval", flowID)
	if err := os.MkdirAll(tfConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create terraform config directory: %w", err)
	}

	// Store the full pending dispatch for later execution
	pendingDispatch := *unit.Dispatch // Copy the dispatch
	pendingDispatch.SkipApproval = true // Pre-set for post-approval re-dispatch

	// Write the flow record with pending dispatch details
	flowRecord := FlowRecord{
		DispatcherID:       d.dispatcherID,
		FlowID:             flowID,
		DispatchType:       DispatchTypeApproval,
		DispatchPath:       unit.Path,
		TFConfigDir:        tfConfigDir,
		StartTime:          time.Now().Format(time.RFC3339),
		Status:             "running",
		PendingDispatch:    &pendingDispatch,
		// Also set legacy fields for backwards compatibility in terraform config
		PendingInstruction: unit.Dispatch.Instruction,
		PendingMode:        unit.Dispatch.Mode,
		PendingAgent:       unit.Dispatch.Agent,
	}
	d.writeFlowRecord(flowRecord)

	// Create the terraform configuration
	if err := d.createApprovalTerraformConfig(tfConfigDir, flowID, approvalOwner, approvalRepoName, sourceContext, unit.Dispatch); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = err.Error()
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("failed to create terraform config: %w", err)
	}

	// Run terraform init
	log.Printf("[%s] Running terraform init in %s", d.dispatcherID, tfConfigDir)
	if err := d.runTerraform(tfConfigDir, "init"); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = fmt.Sprintf("terraform init failed: %v", err)
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Run terraform apply
	log.Printf("[%s] Running terraform apply in %s", d.dispatcherID, tfConfigDir)
	if err := d.runTerraform(tfConfigDir, "apply", "-auto-approve"); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = fmt.Sprintf("terraform apply failed: %v", err)
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	// Initial terraform apply complete - approval PR has been created
	// Now enter monitoring phase to watch for PR merge/close
	flowRecord.Status = "monitoring"
	flowRecord.NeedsMonitoring = true
	flowRecord.LastPollTime = time.Now().Format(time.RFC3339)

	// Get the initial conclusion state
	conclusionState, _ := d.getTerraformOutput(tfConfigDir, "conclusion_state")
	flowRecord.ConclusionState = conclusionState

	// Get the PR URL and register for monitoring
	prURL, _ := d.getTerraformOutput(tfConfigDir, "pr_url")
	if prURL != "" {
		flowRecord.PRUrl = prURL
		log.Printf("[%s] Approval-gated PR: %s (awaiting approval for %s dispatch)", d.dispatcherID, prURL, unit.Dispatch.Type)

		// Register the PR for monitoring with the poller
		if d.prPoller != nil {
			prNumberStr, _ := d.getTerraformOutput(tfConfigDir, "pr_number")

			if prNumberStr != "" {
				var prNumber int
				fmt.Sscanf(prNumberStr, "%d", &prNumber)

				if prNumber > 0 {
					d.prPoller.Register(prpoller.PRRegistration{
						Owner:  approvalOwner,
						Repo:   approvalRepoName,
						Number: prNumber,
						OnChange: func(event prpoller.ChangeEvent) {
							log.Printf("[%s] Detected activity on approval-gated PR %s",
								d.dispatcherID, prURL)
							// Mark this flow as having changes for the polling loop
							d.flowChangesMu.Lock()
							d.flowChanges[flowID] = true
							d.flowChangesMu.Unlock()
						},
					})
					log.Printf("[%s] Registered approval-gated PR %s/%s#%d for monitoring", d.dispatcherID, approvalOwner, approvalRepoName, prNumber)
				}
			}
		}
	}

	d.writeFlowRecord(flowRecord)

	log.Printf("[%s] Approval-gated dispatch flow %s now monitoring for approval (pending type: %s)", d.dispatcherID, flowID, unit.Dispatch.Type)
	return nil
}

// processApprovalDispatch handles explicit approval dispatch (legacy, creates PR in approval repo, waits for merge to execute)
func (d *Dispatcher) processApprovalDispatch(unit DispatchUnit) error {
	log.Printf("[%s] Processing legacy approval dispatch: %s", d.dispatcherID, unit.ID)

	if d.githubPAT == "" {
		return fmt.Errorf("GITHUB_PAT or GH_TOKEN environment variable is required for approval dispatch")
	}

	// Get approval repo (default to sloppo)
	approvalRepo := unit.Dispatch.ApprovalRepo
	if approvalRepo == "" {
		approvalRepo = os.Getenv("APPROVAL_REPO")
	}
	if approvalRepo == "" {
		approvalRepo = "je-sidestuff/sloppo"
	}

	// Parse owner/repo if full name provided, otherwise use githubOwner
	var approvalRepoName string
	var approvalOwner string
	if strings.Contains(approvalRepo, "/") {
		parts := strings.SplitN(approvalRepo, "/", 2)
		approvalOwner = parts[0]
		approvalRepoName = parts[1]
	} else {
		approvalOwner = d.githubOwner
		approvalRepoName = approvalRepo
	}

	// Get source context
	sourceContext := unit.Dispatch.SourceContext
	if sourceContext == "" {
		sourceContext = "heuristic-request"
	}

	// Generate a unique flow ID
	flowID := generateFlowID(DispatchTypeApproval)

	// Create the terraform config directory
	tfConfigDir := filepath.Join(d.dispatcherLive, "flows", "approval", flowID)
	if err := os.MkdirAll(tfConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create terraform config directory: %w", err)
	}

	// Write the flow record with pending instruction details
	flowRecord := FlowRecord{
		DispatcherID:       d.dispatcherID,
		FlowID:             flowID,
		DispatchType:       DispatchTypeApproval,
		DispatchPath:       unit.Path,
		TFConfigDir:        tfConfigDir,
		StartTime:          time.Now().Format(time.RFC3339),
		Status:             "running",
		PendingInstruction: unit.Dispatch.Instruction,
		PendingMode:        unit.Dispatch.Mode,
		PendingAgent:       unit.Dispatch.Agent,
	}
	d.writeFlowRecord(flowRecord)

	// Create the terraform configuration
	if err := d.createApprovalTerraformConfig(tfConfigDir, flowID, approvalOwner, approvalRepoName, sourceContext, unit.Dispatch); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = err.Error()
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("failed to create terraform config: %w", err)
	}

	// Run terraform init
	log.Printf("[%s] Running terraform init in %s", d.dispatcherID, tfConfigDir)
	if err := d.runTerraform(tfConfigDir, "init"); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = fmt.Sprintf("terraform init failed: %v", err)
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Run terraform apply
	log.Printf("[%s] Running terraform apply in %s", d.dispatcherID, tfConfigDir)
	if err := d.runTerraform(tfConfigDir, "apply", "-auto-approve"); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = fmt.Sprintf("terraform apply failed: %v", err)
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(flowRecord)
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	// Initial terraform apply complete - approval PR has been created
	// Now enter monitoring phase to watch for PR merge/close
	flowRecord.Status = "monitoring"
	flowRecord.NeedsMonitoring = true
	flowRecord.LastPollTime = time.Now().Format(time.RFC3339)

	// Get the initial conclusion state
	conclusionState, _ := d.getTerraformOutput(tfConfigDir, "conclusion_state")
	flowRecord.ConclusionState = conclusionState

	// Get the PR URL and register for monitoring
	prURL, _ := d.getTerraformOutput(tfConfigDir, "pr_url")
	if prURL != "" {
		flowRecord.PRUrl = prURL
		log.Printf("[%s] Approval PR: %s (awaiting approval)", d.dispatcherID, prURL)

		// Register the PR for monitoring with the poller
		if d.prPoller != nil {
			prNumberStr, _ := d.getTerraformOutput(tfConfigDir, "pr_number")

			if prNumberStr != "" {
				var prNumber int
				fmt.Sscanf(prNumberStr, "%d", &prNumber)

				if prNumber > 0 {
					d.prPoller.Register(prpoller.PRRegistration{
						Owner:  approvalOwner,
						Repo:   approvalRepoName,
						Number: prNumber,
						OnChange: func(event prpoller.ChangeEvent) {
							log.Printf("[%s] Detected activity on approval PR %s",
								d.dispatcherID, prURL)
							// Mark this flow as having changes for the polling loop
							d.flowChangesMu.Lock()
							d.flowChanges[flowID] = true
							d.flowChangesMu.Unlock()
						},
					})
					log.Printf("[%s] Registered approval PR %s/%s#%d for monitoring", d.dispatcherID, approvalOwner, approvalRepoName, prNumber)
				}
			}
		}
	}

	d.writeFlowRecord(flowRecord)

	log.Printf("[%s] Approval dispatch flow %s now monitoring for approval/rejection", d.dispatcherID, flowID)
	return nil
}

// createRepoIsolationTerraformConfig creates the terraform configuration for repo-isolation dispatch
func (d *Dispatcher) createRepoIsolationTerraformConfig(configDir, flowID, targetRepo, isolationName string, dispatch *Dispatch) error {
	// Get the path to the repo-isolation module
	exe, err := os.Executable()
	var modulePath string
	if err == nil {
		modulePath = filepath.Join(filepath.Dir(exe), "modules", "containment", "repo-isolation")
		if _, err := os.Stat(modulePath); err != nil {
			// Try from CWD
			cwd, _ := os.Getwd()
			modulePath = filepath.Join(cwd, "agent-dispatch", "modules", "containment", "repo-isolation")
		}
	}

	// Default to absolute path if nothing works
	if modulePath == "" || !fileExists(modulePath) {
		modulePath = "/workspaces/workspace/sandbox/AI-sandboxing/agent-dispatch/modules/containment/repo-isolation"
	}

	// Create providers.tf
	providersTF := `terraform {
  required_providers {
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.9"
    }
  }
}

provider "github" {
  token = var.github_pat
}
`

	// Create variables.tf
	variablesTF := `variable "github_pat" {
  description = "GitHub Personal Access Token"
  type        = string
  sensitive   = true
}

variable "github_owner" {
  description = "The GitHub owner (user or organization) where the isolation repository will be created"
  type        = string
}

variable "instruction" {
  description = "The instruction to pass to the AI agent"
  type        = string
}

variable "instruction_mode" {
  description = "The mode for the instruction (prompt or execute)"
  type        = string
  default     = "execute"
}
`

	// Get mode from dispatch, default to "execute"
	mode := "execute"
	if dispatch.Mode != "" {
		mode = dispatch.Mode
	}

	// Create main.tf with module reference
	mainTF := fmt.Sprintf(`module "repo_isolation_dispatch" {
  source = "%s"

  name                   = "%s"
  dispatcher_name        = "%s"
  github_owner           = var.github_owner
  github_pat             = var.github_pat
  target_repo            = "%s"
  slopspaces_working_dir = "/workspaces/slopspaces/working/"
  instruction            = var.instruction
  instruction_mode       = var.instruction_mode
}
`, modulePath, isolationName, flowID, targetRepo)

	// Create outputs.tf
	outputsTF := `output "isolation_repo_ssh_clone_url" {
  value       = module.repo_isolation_dispatch.isolation_repo_ssh_clone_url
  description = "The SSH clone URL of the isolation repository"
}

output "branch_name" {
  value       = module.repo_isolation_dispatch.branch_name
  description = "The name of the containment branch"
}

output "unix_timestamp" {
  value       = module.repo_isolation_dispatch.unix_timestamp
  description = "The unix timestamp for this dispatch"
}

output "dispatch_time" {
  value       = module.repo_isolation_dispatch.dispatch_time
  description = "The RFC3339 formatted time when this dispatch was created"
}

output "pr_url" {
  value       = module.repo_isolation_dispatch.pr_url
  description = "The URL of the containment PR for monitoring"
}

output "pr_number" {
  value       = module.repo_isolation_dispatch.pr_number
  description = "The PR number for the containment PR"
}

output "isolation_repo_full_name" {
  value       = module.repo_isolation_dispatch.isolation_repo_full_name
  description = "The full name (owner/repo) of the isolation repository"
}

output "conclusion_state" {
  value       = module.repo_isolation_dispatch.conclusion_state
  description = "Simplified conclusion state: 'active', 'closed', or 'merged'"
}

output "reintegration_pr_url" {
  value       = module.repo_isolation_dispatch.reintegration_pr_url
  description = "The URL of the re-integration PR (only set when isolation PR is merged)"
}

output "reintegration_conclusion_state" {
  value       = module.repo_isolation_dispatch.reintegration_conclusion_state
  description = "Conclusion state for reintegration PR: 'none', 'active', 'closed', or 'merged'"
}
`

	// Escape the instruction for Terraform HCL
	escapedInstruction := strings.ReplaceAll(dispatch.Instruction, "\\", "\\\\")
	escapedInstruction = strings.ReplaceAll(escapedInstruction, "\"", "\\\"")
	escapedInstruction = strings.ReplaceAll(escapedInstruction, "\n", "\\n")

	// Create terraform.tfvars with the PAT, owner, and instruction
	tfvarsTF := fmt.Sprintf(`github_pat       = "%s"
github_owner     = "%s"
instruction      = "%s"
instruction_mode = "%s"
`, d.githubPAT, d.githubOwner, escapedInstruction, mode)

	// Write all the files
	files := map[string]string{
		"providers.tf":     providersTF,
		"variables.tf":     variablesTF,
		"main.tf":          mainTF,
		"outputs.tf":       outputsTF,
		"terraform.tfvars": tfvarsTF,
	}

	for filename, content := range files {
		path := filepath.Join(configDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Also write a dispatch record to the config dir for reference
	dispatchData, _ := json.MarshalIndent(dispatch, "", "  ")
	if err := os.WriteFile(filepath.Join(configDir, "DISPATCH_RECORD.json"), dispatchData, 0644); err != nil {
		log.Printf("Warning: failed to write dispatch record: %v", err)
	}

	return nil
}

// createApprovalTerraformConfig creates the terraform configuration for approval dispatch
func (d *Dispatcher) createApprovalTerraformConfig(configDir, flowID, approvalOwner, approvalRepoName, sourceContext string, dispatch *Dispatch) error {
	// Get the path to the approval module
	exe, err := os.Executable()
	var modulePath string
	if err == nil {
		modulePath = filepath.Join(filepath.Dir(exe), "modules", "containment", "approval")
		if _, err := os.Stat(modulePath); err != nil {
			// Try from CWD
			cwd, _ := os.Getwd()
			modulePath = filepath.Join(cwd, "agent-dispatch", "modules", "containment", "approval")
		}
	}

	// Default to absolute path if nothing works
	if modulePath == "" || !fileExists(modulePath) {
		modulePath = "/workspaces/workspace/sandbox/AI-sandboxing/agent-dispatch/modules/containment/approval"
	}

	// Encode metadata as JSON
	metadataJSON := "{}"
	if dispatch.Metadata != nil {
		metadataBytes, err := json.Marshal(dispatch.Metadata)
		if err == nil {
			metadataJSON = string(metadataBytes)
		}
	}

	// Create providers.tf
	providersTF := `terraform {
  required_providers {
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.9"
    }
  }
}

provider "github" {
  token = var.github_pat
}
`

	// Create variables.tf
	variablesTF := `variable "github_pat" {
  description = "GitHub Personal Access Token"
  type        = string
  sensitive   = true
}

variable "github_owner" {
  description = "The GitHub owner (user or organization) for the approval repository"
  type        = string
}
`

	// Escape the instruction for Terraform
	escapedInstruction := strings.ReplaceAll(dispatch.Instruction, "\\", "\\\\")
	escapedInstruction = strings.ReplaceAll(escapedInstruction, "\"", "\\\"")
	escapedInstruction = strings.ReplaceAll(escapedInstruction, "\n", "\\n")

	// Escape metadata JSON
	escapedMetadata := strings.ReplaceAll(metadataJSON, "\\", "\\\\")
	escapedMetadata = strings.ReplaceAll(escapedMetadata, "\"", "\\\"")

	// Create main.tf with module reference
	mainTF := fmt.Sprintf(`module "approval_dispatch" {
  source = "%s"

  dispatcher_name     = "%s"
  github_owner        = var.github_owner
  github_pat          = var.github_pat
  approval_repo       = "%s"
  pending_instruction = "%s"
  pending_mode        = "%s"
  pending_agent       = "%s"
  source_context      = "%s"
  metadata_json       = "%s"
}
`, modulePath, flowID, approvalRepoName, escapedInstruction, dispatch.Mode, dispatch.Agent, sourceContext, escapedMetadata)

	// Create outputs.tf
	outputsTF := `output "pr_url" {
  value       = module.approval_dispatch.pr_url
  description = "The URL of the approval PR"
}

output "pr_number" {
  value       = module.approval_dispatch.pr_number
  description = "The PR number for the approval PR"
}

output "approval_repo_full_name" {
  value       = module.approval_dispatch.approval_repo_full_name
  description = "The full name (owner/repo) of the approval repository"
}

output "branch_name" {
  value       = module.approval_dispatch.branch_name
  description = "The name of the approval branch"
}

output "unix_timestamp" {
  value       = module.approval_dispatch.unix_timestamp
  description = "The unix timestamp for this approval request"
}

output "dispatch_time" {
  value       = module.approval_dispatch.dispatch_time
  description = "The RFC3339 formatted time when this approval request was created"
}

output "conclusion_state" {
  value       = module.approval_dispatch.conclusion_state
  description = "Simplified conclusion state: 'active', 'closed', or 'merged'"
}

output "pending_instruction" {
  value       = module.approval_dispatch.pending_instruction
  description = "The instruction to execute if approved"
}

output "pending_mode" {
  value       = module.approval_dispatch.pending_mode
  description = "The mode for executing the pending instruction"
}

output "pending_agent" {
  value       = module.approval_dispatch.pending_agent
  description = "The agent override for executing the pending instruction"
}
`

	// Create terraform.tfvars with the PAT and owner
	tfvarsTF := fmt.Sprintf(`github_pat   = "%s"
github_owner = "%s"
`, d.githubPAT, approvalOwner)

	// Write all the files
	files := map[string]string{
		"providers.tf":     providersTF,
		"variables.tf":     variablesTF,
		"main.tf":          mainTF,
		"outputs.tf":       outputsTF,
		"terraform.tfvars": tfvarsTF,
	}

	for filename, content := range files {
		path := filepath.Join(configDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Also write a dispatch record to the config dir for reference
	dispatchData, _ := json.MarshalIndent(dispatch, "", "  ")
	if err := os.WriteFile(filepath.Join(configDir, "DISPATCH_RECORD.json"), dispatchData, 0644); err != nil {
		log.Printf("Warning: failed to write dispatch record: %v", err)
	}

	return nil
}

// createInRepoTerraformConfig creates the terraform configuration for in-repo dispatch
func (d *Dispatcher) createInRepoTerraformConfig(configDir, flowID, targetRepo string, dispatch *Dispatch) error {
	// Get the path to the in-repo module
	// Try relative to the executable first
	exe, err := os.Executable()
	var modulePath string
	if err == nil {
		modulePath = filepath.Join(filepath.Dir(exe), "modules", "containment", "in-repo")
		if _, err := os.Stat(modulePath); err != nil {
			// Try from CWD
			cwd, _ := os.Getwd()
			modulePath = filepath.Join(cwd, "agent-dispatch", "modules", "containment", "in-repo")
		}
	}

	// Default to absolute path if nothing works
	if modulePath == "" || !fileExists(modulePath) {
		modulePath = "/workspaces/workspace/sandbox/AI-sandboxing/agent-dispatch/modules/containment/in-repo"
	}

	// Create providers.tf
	providersTF := `terraform {
  required_providers {
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
  }
}

provider "github" {
  token = var.github_pat
}
`

	// Create variables.tf
	variablesTF := `variable "github_pat" {
  description = "GitHub Personal Access Token"
  type        = string
  sensitive   = true
}
`

	// Create main.tf with module reference
	mainTF := fmt.Sprintf(`module "in_repo_dispatch" {
  source = "%s"

  dispatcher_name       = "%s"
  github_pat           = var.github_pat
  target_repo          = "%s"
  slopspaces_working_dir = "/workspaces/slopspaces/working/"
}
`, modulePath, flowID, targetRepo)

	// Create outputs.tf
	outputsTF := `output "pr_url" {
  value       = module.in_repo_dispatch.pr_url
  description = "The URL of the created pull request"
}

output "branch_name" {
  value       = module.in_repo_dispatch.branch_name
  description = "The name of the containment branch"
}
`

	// Create terraform.tfvars with the PAT
	tfvarsTF := fmt.Sprintf(`github_pat = "%s"
`, d.githubPAT)

	// Write all the files
	files := map[string]string{
		"providers.tf":     providersTF,
		"variables.tf":     variablesTF,
		"main.tf":          mainTF,
		"outputs.tf":       outputsTF,
		"terraform.tfvars": tfvarsTF,
	}

	for filename, content := range files {
		path := filepath.Join(configDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Also write a dispatch record to the config dir for reference
	dispatchData, _ := json.MarshalIndent(dispatch, "", "  ")
	if err := os.WriteFile(filepath.Join(configDir, "DISPATCH_RECORD.json"), dispatchData, 0644); err != nil {
		log.Printf("Warning: failed to write dispatch record: %v", err)
	}

	return nil
}

// runTerraform executes a terraform command in the given directory
func (d *Dispatcher) runTerraform(workDir string, args ...string) error {
	cmd := exec.Command(d.terraformBinary, args...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// getTerraformOutput retrieves a terraform output value
func (d *Dispatcher) getTerraformOutput(workDir, outputName string) (string, error) {
	cmd := exec.Command(d.terraformBinary, "output", "-raw", outputName)
	cmd.Dir = workDir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// markDispatchComplete marks a dispatch as complete and moves to output
func (d *Dispatcher) markDispatchComplete(unit DispatchUnit, startTime, endTime time.Time) {
	duration := endTime.Sub(startTime)

	// For direct dispatch, the folder stays in place for worker pickup
	// For in-repo dispatch, we move to output since terraform lifecycle is complete

	if unit.Dispatch.Type == DispatchTypeInRepo {
		// Move folder to output directory
		destPath := filepath.Join(d.outputDir, unit.ID)
		if err := os.Rename(unit.Path, destPath); err != nil {
			log.Printf("[%s] Warning: failed to move dispatch to output: %v", d.dispatcherID, err)
			return
		}

		// Create DISPATCH_PROCESSED.md in the destination
		processedMD := filepath.Join(destPath, "DISPATCH_PROCESSED.md")
		processedContent := fmt.Sprintf("# Dispatch Processed\n\nWatcher ID: %s\nStarted: %s\nCompleted: %s\nDuration: %s\nType: %s\n",
			d.dispatcherID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
			duration.Round(time.Millisecond).String(), unit.Dispatch.Type)
		if err := os.WriteFile(processedMD, []byte(processedContent), 0644); err != nil {
			log.Printf("Warning: failed to create DISPATCH_PROCESSED.md: %v", err)
		}
	}
	// For direct dispatch, we don't move - we just transformed it for the worker

	// Write dispatch record
	d.writeDispatchRecordWatch(unit, startTime, endTime, true, "")
}

// markDispatchFailed marks a dispatch as failed
func (d *Dispatcher) markDispatchFailed(unit DispatchUnit, dispatchErr error, startTime, endTime time.Time) {
	duration := endTime.Sub(startTime)

	// Move to output with error marker
	destPath := filepath.Join(d.outputDir, unit.ID)
	if err := os.Rename(unit.Path, destPath); err != nil {
		log.Printf("[%s] Warning: failed to move failed dispatch to output: %v", d.dispatcherID, err)
		destPath = unit.Path // Use original path for the error file
	}

	// Create DISPATCH_FAILED.md
	failedMD := filepath.Join(destPath, "DISPATCH_FAILED.md")
	failedContent := fmt.Sprintf("# Dispatch Failed\n\nWatcher ID: %s\nStarted: %s\nFailed: %s\nDuration: %s\nType: %s\nError: %s\n",
		d.dispatcherID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
		duration.Round(time.Millisecond).String(), unit.Dispatch.Type, dispatchErr.Error())
	if err := os.WriteFile(failedMD, []byte(failedContent), 0644); err != nil {
		log.Printf("Warning: failed to create DISPATCH_FAILED.md: %v", err)
	}

	// Write dispatch record
	d.writeDispatchRecordWatch(unit, startTime, endTime, false, dispatchErr.Error())
}

// writeDispatchRecordWatch writes a record of the watch mode dispatch operation
func (d *Dispatcher) writeDispatchRecordWatch(unit DispatchUnit, startTime, endTime time.Time, success bool, errMsg string) {
	record := map[string]interface{}{
		"watcher_id":    d.dispatcherID,
		"dispatch_id":   unit.ID,
		"dispatch_type": unit.Dispatch.Type,
		"dispatch_path": unit.Path,
		"start_time":    startTime.Format(time.RFC3339),
		"end_time":      endTime.Format(time.RFC3339),
		"duration_ms":   endTime.Sub(startTime).Milliseconds(),
		"success":       success,
	}
	if errMsg != "" {
		record["error"] = errMsg
	}

	recordData, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		log.Printf("[%s] Warning: failed to marshal dispatch record: %v", d.dispatcherID, err)
		return
	}

	recordFilename := fmt.Sprintf("%s_%s_%d.json", d.dispatcherID, unit.ID, time.Now().Unix())
	recordPath := filepath.Join(d.recordsDir, "dispatch-watch", recordFilename)

	if err := os.WriteFile(recordPath, recordData, 0644); err != nil {
		log.Printf("[%s] Warning: failed to write dispatch record: %v", d.dispatcherID, err)
	}
}

// writeFlowRecord writes a flow tracking record
func (d *Dispatcher) writeFlowRecord(record FlowRecord) {
	recordData, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		log.Printf("[%s] Warning: failed to marshal flow record: %v", d.dispatcherID, err)
		return
	}

	recordFilename := fmt.Sprintf("flow_%s.json", record.FlowID)
	recordPath := filepath.Join(d.recordsDir, "dispatch-watch", recordFilename)

	if err := os.WriteFile(recordPath, recordData, 0644); err != nil {
		log.Printf("[%s] Warning: failed to write flow record: %v", d.dispatcherID, err)
	}
}

// fileExists checks if a file or directory exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// loadMonitoringFlows loads all flows that need monitoring from the records directory
func (d *Dispatcher) loadMonitoringFlows() ([]FlowRecord, error) {
	recordsPath := filepath.Join(d.recordsDir, "dispatch-watch")
	entries, err := os.ReadDir(recordsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var monitoringFlows []FlowRecord
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "flow_") && strings.HasSuffix(entry.Name(), ".json") {
			recordPath := filepath.Join(recordsPath, entry.Name())
			data, err := os.ReadFile(recordPath)
			if err != nil {
				log.Printf("[%s] Warning: failed to read flow record %s: %v", d.dispatcherID, entry.Name(), err)
				continue
			}

			var record FlowRecord
			if err := json.Unmarshal(data, &record); err != nil {
				log.Printf("[%s] Warning: failed to parse flow record %s: %v", d.dispatcherID, entry.Name(), err)
				continue
			}

			// Only include flows that need monitoring
			if record.NeedsMonitoring && record.Status == "monitoring" {
				monitoringFlows = append(monitoringFlows, record)
			}
		}
	}

	return monitoringFlows, nil
}

// pollMonitoringFlow runs terraform apply to refresh state and check conclusion state
func (d *Dispatcher) pollMonitoringFlow(record *FlowRecord) error {
	if record.TFConfigDir == "" {
		return fmt.Errorf("flow %s has no terraform config directory", record.FlowID)
	}

	// Check if config dir still exists
	if !fileExists(record.TFConfigDir) {
		log.Printf("[%s] Flow %s terraform config dir no longer exists, marking as completed", d.dispatcherID, record.FlowID)
		record.Status = "completed"
		record.NeedsMonitoring = false
		record.EndTime = time.Now().Format(time.RFC3339)
		d.writeFlowRecord(*record)
		return nil
	}

	log.Printf("[%s] Polling flow %s for conclusion state", d.dispatcherID, record.FlowID)

	// Run terraform apply to refresh state and potentially trigger revisions or re-integration
	if err := d.runTerraform(record.TFConfigDir, "apply", "-auto-approve"); err != nil {
		log.Printf("[%s] Flow %s terraform apply failed: %v", d.dispatcherID, record.FlowID, err)
		// Don't fail the flow yet - could be a transient error
		return err
	}

	// Update last poll time
	record.LastPollTime = time.Now().Format(time.RFC3339)

	// Get the current conclusion state (isolation PR state)
	newConclusionState, err := d.getTerraformOutput(record.TFConfigDir, "conclusion_state")
	if err != nil {
		log.Printf("[%s] Flow %s failed to get conclusion_state: %v", d.dispatcherID, record.FlowID, err)
		d.writeFlowRecord(*record)
		return nil // Continue monitoring
	}

	oldState := record.ConclusionState
	record.ConclusionState = newConclusionState

	// Check for isolation PR state change
	if oldState != newConclusionState {
		log.Printf("[%s] Flow %s conclusion state changed: %s -> %s", d.dispatcherID, record.FlowID, oldState, newConclusionState)
		return d.handleConclusionStateChange(record, newConclusionState)
	}

	// If isolation PR is already merged, check the reintegration PR state
	if newConclusionState == ConclusionStateMerged && record.DispatchType == DispatchTypeRepoIsolation {
		reintegrationState, err := d.getTerraformOutput(record.TFConfigDir, "reintegration_conclusion_state")
		if err == nil {
			// Check if reintegration PR is now merged or closed
			if reintegrationState == ReintegrationStateMerged || reintegrationState == ReintegrationStateClosed {
				log.Printf("[%s] Flow %s reintegration PR state: %s - ready for cleanup", d.dispatcherID, record.FlowID, reintegrationState)
				return d.handleReintegrationComplete(record, reintegrationState)
			}
			// Reintegration PR is still active (or not created yet) - keep monitoring
			if reintegrationState == ReintegrationStateActive {
				log.Printf("[%s] Flow %s awaiting reintegration PR (state: %s)", d.dispatcherID, record.FlowID, reintegrationState)
			}
		}
	}

	// No actionable state change - keep monitoring
	d.writeFlowRecord(*record)
	return nil
}

// handleConclusionStateChange reacts to a change in the conclusion state
func (d *Dispatcher) handleConclusionStateChange(record *FlowRecord, newState string) error {
	switch newState {
	case ConclusionStateMerged:
		return d.handleFlowMerged(record)
	case ConclusionStateClosed:
		return d.handleFlowClosed(record)
	case ConclusionStateActive:
		// Still active - keep monitoring
		d.writeFlowRecord(*record)
		return nil
	default:
		log.Printf("[%s] Flow %s has unknown conclusion state: %s", d.dispatcherID, record.FlowID, newState)
		d.writeFlowRecord(*record)
		return nil
	}
}

// handleFlowMerged handles a flow whose isolation PR has been merged
// For repo-isolation, this means we now need to wait for the reintegration PR
func (d *Dispatcher) handleFlowMerged(record *FlowRecord) error {
	log.Printf("[%s] Flow %s PR was merged", d.dispatcherID, record.FlowID)

	// For approval dispatch, create INSTRUCTION.json to execute the pending instruction
	if record.DispatchType == DispatchTypeApproval {
		return d.handleApprovalMerged(record)
	}

	// For repo-isolation, we need to wait for the reintegration PR to be merged/closed
	if record.DispatchType == DispatchTypeRepoIsolation {
		reintegrationURL, err := d.getTerraformOutput(record.TFConfigDir, "reintegration_pr_url")
		if err == nil && reintegrationURL != "" {
			record.ReintegrationURL = reintegrationURL
			log.Printf("[%s] Flow %s re-integration PR created: %s", d.dispatcherID, record.FlowID, reintegrationURL)
			log.Printf("[%s] Flow %s now awaiting reintegration PR merge/close before cleanup", d.dispatcherID, record.FlowID)
		}

		// Check if the reintegration PR is already merged/closed
		reintegrationState, err := d.getTerraformOutput(record.TFConfigDir, "reintegration_conclusion_state")
		if err == nil && (reintegrationState == ReintegrationStateMerged || reintegrationState == ReintegrationStateClosed) {
			// Reintegration PR is already done, proceed to cleanup
			return d.handleReintegrationComplete(record, reintegrationState)
		}

		// Keep monitoring - reintegration PR is still open
		d.writeFlowRecord(*record)
		return nil
	}

	// For non-repo-isolation flows, proceed with immediate cleanup
	return d.performFlowCleanup(record, "merged")
}

// handleApprovalMerged handles when an approval PR is merged - creates DISPATCH.json or INSTRUCTION.json to execute
func (d *Dispatcher) handleApprovalMerged(record *FlowRecord) error {
	// Check if we have a pending dispatch (new flow) or just pending instruction (legacy flow)
	if record.PendingDispatch != nil {
		return d.handleApprovalMergedDispatch(record)
	}
	return d.handleApprovalMergedInstruction(record)
}

// handleApprovalMergedDispatch creates DISPATCH.json after approval for approval-gated flows
func (d *Dispatcher) handleApprovalMergedDispatch(record *FlowRecord) error {
	log.Printf("[%s] Approval PR merged for flow %s - creating DISPATCH.json (type: %s)", d.dispatcherID, record.FlowID, record.PendingDispatch.Type)

	// Create a new work unit directory for the approved dispatch
	timestamp := time.Now().Format("20060102-150405")
	workUnitID := fmt.Sprintf("approved-%s-%s", record.FlowID, timestamp)
	workUnitPath := filepath.Join(d.inputDir, "any", workUnitID)

	if err := os.MkdirAll(workUnitPath, 0755); err != nil {
		log.Printf("[%s] Error creating work unit directory: %v", d.dispatcherID, err)
		record.Error = fmt.Sprintf("failed to create work unit directory: %v", err)
		d.writeFlowRecord(*record)
		return d.performFlowCleanup(record, "approved-error")
	}

	// The pending dispatch already has SkipApproval=true (set in createApprovalGatedDispatch)
	// This prevents infinite approval loops
	dispatch := record.PendingDispatch
	dispatch.Timestamp = time.Now().Format(time.RFC3339)

	dispatchData, err := json.MarshalIndent(dispatch, "", "  ")
	if err != nil {
		log.Printf("[%s] Error marshaling dispatch: %v", d.dispatcherID, err)
		record.Error = fmt.Sprintf("failed to marshal dispatch: %v", err)
		d.writeFlowRecord(*record)
		return d.performFlowCleanup(record, "approved-error")
	}

	dispatchPath := filepath.Join(workUnitPath, "DISPATCH.json")
	if err := os.WriteFile(dispatchPath, dispatchData, 0644); err != nil {
		log.Printf("[%s] Error writing DISPATCH.json: %v", d.dispatcherID, err)
		record.Error = fmt.Sprintf("failed to write DISPATCH.json: %v", err)
		d.writeFlowRecord(*record)
		return d.performFlowCleanup(record, "approved-error")
	}

	log.Printf("[%s] Created DISPATCH.json at %s for approved %s dispatch", d.dispatcherID, dispatchPath, dispatch.Type)
	log.Printf("[%s] Approved dispatch (type=%s, skip_approval=%v): %s", d.dispatcherID, dispatch.Type, dispatch.SkipApproval, truncateString(dispatch.Instruction, 100))

	// Cleanup the approval flow
	return d.performFlowCleanup(record, "approved")
}

// handleApprovalMergedInstruction creates INSTRUCTION.json after approval (legacy flow)
func (d *Dispatcher) handleApprovalMergedInstruction(record *FlowRecord) error {
	log.Printf("[%s] Approval PR merged for flow %s - creating INSTRUCTION.json (legacy)", d.dispatcherID, record.FlowID)

	// Get the pending instruction details from the flow record (stored at dispatch time)
	pendingInstruction := record.PendingInstruction
	pendingMode := record.PendingMode
	pendingAgent := record.PendingAgent

	// Also try to get from terraform outputs as backup
	if pendingInstruction == "" {
		pendingInstruction, _ = d.getTerraformOutput(record.TFConfigDir, "pending_instruction")
	}
	if pendingMode == "" {
		pendingMode, _ = d.getTerraformOutput(record.TFConfigDir, "pending_mode")
	}
	if pendingAgent == "" {
		pendingAgent, _ = d.getTerraformOutput(record.TFConfigDir, "pending_agent")
	}

	if pendingInstruction == "" {
		log.Printf("[%s] Warning: no pending instruction found for approved flow %s", d.dispatcherID, record.FlowID)
		return d.performFlowCleanup(record, "approved-no-instruction")
	}

	// Default mode to execute
	if pendingMode == "" {
		pendingMode = "execute"
	}

	// Create a new work unit directory for the approved instruction
	timestamp := time.Now().Format("20060102-150405")
	workUnitID := fmt.Sprintf("approved-%s-%s", record.FlowID, timestamp)
	workUnitPath := filepath.Join(d.inputDir, "any", workUnitID)

	if err := os.MkdirAll(workUnitPath, 0755); err != nil {
		log.Printf("[%s] Error creating work unit directory: %v", d.dispatcherID, err)
		record.Error = fmt.Sprintf("failed to create work unit directory: %v", err)
		d.writeFlowRecord(*record)
		return d.performFlowCleanup(record, "approved-error")
	}

	// Create INSTRUCTION.json
	instruction := Instruction{
		Instruction: pendingInstruction,
		Mode:        pendingMode,
		Agent:       pendingAgent,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	instructionData, err := json.MarshalIndent(instruction, "", "  ")
	if err != nil {
		log.Printf("[%s] Error marshaling instruction: %v", d.dispatcherID, err)
		record.Error = fmt.Sprintf("failed to marshal instruction: %v", err)
		d.writeFlowRecord(*record)
		return d.performFlowCleanup(record, "approved-error")
	}

	instructionPath := filepath.Join(workUnitPath, "INSTRUCTION.json")
	if err := os.WriteFile(instructionPath, instructionData, 0644); err != nil {
		log.Printf("[%s] Error writing INSTRUCTION.json: %v", d.dispatcherID, err)
		record.Error = fmt.Sprintf("failed to write INSTRUCTION.json: %v", err)
		d.writeFlowRecord(*record)
		return d.performFlowCleanup(record, "approved-error")
	}

	log.Printf("[%s] Created INSTRUCTION.json at %s for approved instruction (legacy)", d.dispatcherID, instructionPath)
	log.Printf("[%s] Approved instruction (mode=%s): %s", d.dispatcherID, pendingMode, truncateString(pendingInstruction, 100))

	// Cleanup the approval flow
	return d.performFlowCleanup(record, "approved")
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// handleReintegrationComplete handles cleanup after reintegration PR is merged/closed
func (d *Dispatcher) handleReintegrationComplete(record *FlowRecord, reintegrationState string) error {
	log.Printf("[%s] Flow %s reintegration PR is %s - performing cleanup", d.dispatcherID, record.FlowID, reintegrationState)
	return d.performFlowCleanup(record, fmt.Sprintf("merged+reintegration_%s", reintegrationState))
}

// performFlowCleanup runs terraform destroy and marks the flow as completed
func (d *Dispatcher) performFlowCleanup(record *FlowRecord, reason string) error {
	log.Printf("[%s] Running terraform destroy for flow %s (reason: %s)", d.dispatcherID, record.FlowID, reason)
	if err := d.runTerraform(record.TFConfigDir, "destroy", "-auto-approve"); err != nil {
		log.Printf("[%s] Flow %s terraform destroy failed: %v", d.dispatcherID, record.FlowID, err)
		record.Error = fmt.Sprintf("terraform destroy failed: %v", err)
		// Mark as completed anyway
	}

	// Mark as completed
	record.Status = "completed"
	record.NeedsMonitoring = false
	record.EndTime = time.Now().Format(time.RFC3339)
	d.writeFlowRecord(*record)

	log.Printf("[%s] Flow %s completed (%s)", d.dispatcherID, record.FlowID, reason)
	return nil
}

// handleFlowClosed handles a flow whose isolation PR was closed without being merged
func (d *Dispatcher) handleFlowClosed(record *FlowRecord) error {
	log.Printf("[%s] Flow %s isolation PR was closed without merge - cleaning up", d.dispatcherID, record.FlowID)
	return d.performFlowCleanup(record, "closed")
}

// pollAllMonitoringFlows checks all flows that are in monitoring state
// It only runs terraform apply when:
// 1. The PR poller detected new comments for that flow, OR
// 2. The periodic interval has elapsed (default 1 minute) - to catch PR state changes
func (d *Dispatcher) pollAllMonitoringFlows() {
	flows, err := d.loadMonitoringFlows()
	if err != nil {
		log.Printf("[%s] Error loading monitoring flows: %v", d.dispatcherID, err)
		return
	}

	if len(flows) == 0 {
		return
	}

	// Check if periodic poll is due
	now := time.Now()
	periodicPollDue := now.Sub(d.lastPeriodicPoll) >= d.periodicInterval

	polledCount := 0
	for i := range flows {
		flow := &flows[i]

		// Check if this flow has detected changes OR periodic poll is due
		hasChanges := d.hasFlowChanges(flow.FlowID)

		if hasChanges {
			log.Printf("[%s] Polling flow %s (detected PR activity)", d.dispatcherID, flow.FlowID)
			if err := d.pollMonitoringFlow(flow); err != nil {
				log.Printf("[%s] Error polling flow %s: %v", d.dispatcherID, flow.FlowID, err)
			}
			polledCount++
		} else if periodicPollDue {
			log.Printf("[%s] Periodic poll of flow %s (checking for PR state changes)", d.dispatcherID, flow.FlowID)
			if err := d.pollMonitoringFlow(flow); err != nil {
				log.Printf("[%s] Error polling flow %s: %v", d.dispatcherID, flow.FlowID, err)
			}
			polledCount++
		}
	}

	// Update last periodic poll time if we did a periodic poll
	if periodicPollDue {
		d.lastPeriodicPoll = now
	}

	// Log summary if we polled anything
	if polledCount > 0 {
		log.Printf("[%s] Polled %d/%d monitoring flows", d.dispatcherID, polledCount, len(flows))
	}
}

// runWatchLoop is the main watch loop
func (d *Dispatcher) runWatchLoop() {
	log.Printf("[%s] Dispatch watcher started", d.dispatcherID)
	log.Printf("[%s] Watching: %s", d.dispatcherID, filepath.Join(d.inputDir, "any"))
	log.Printf("[%s] Output: %s", d.dispatcherID, d.outputDir)
	log.Printf("[%s] Records: %s", d.dispatcherID, filepath.Join(d.recordsDir, "dispatch-watch"))
	log.Printf("[%s] Dispatcher Live: %s", d.dispatcherID, d.dispatcherLive)
	log.Printf("[%s] GitHub Owner: %s", d.dispatcherID, d.githubOwner)

	if d.githubPAT != "" {
		log.Printf("[%s] GitHub PAT: configured (in-repo and repo-isolation dispatch enabled)", d.dispatcherID)
	} else {
		log.Printf("[%s] GitHub PAT: not configured (in-repo and repo-isolation dispatch will fail)", d.dispatcherID)
	}

	// Start the PR poller if configured
	if d.prPoller != nil {
		log.Printf("[%s] Starting PR comment poller (30s interval)", d.dispatcherID)
		d.prPoller.Start()

		// Re-register any existing flows from disk
		d.reregisterActiveFlows()
	}

	// Set up graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Create a done channel for the main loop
	done := make(chan struct{})

	go func() {
		<-sigCh
		log.Printf("[%s] Shutdown signal received, stopping...", d.dispatcherID)
		if d.prPoller != nil {
			d.prPoller.Stop()
		}
		close(done)
	}()

	for {
		select {
		case <-done:
			log.Printf("[%s] Dispatcher stopped", d.dispatcherID)
			return
		default:
		}

		dispatchUnits, err := d.checkForDispatchUnits()
		if err != nil {
			log.Printf("[%s] Error checking for dispatch units: %v", d.dispatcherID, err)
		}

		if len(dispatchUnits) > 0 {
			// Reset backoff on activity
			d.lastActivity = time.Now()
			d.backoffIndex = 0
			d.nextBackoffLog = d.lastActivity.Add(backoffLevels[0])

			for _, unit := range dispatchUnits {
				if err := d.processDispatchUnit(unit); err != nil {
					log.Printf("[%s] Error processing dispatch unit %s: %v", d.dispatcherID, unit.ID, err)
				}
			}
		}

		// Phase 2: Poll all flows that are in monitoring state
		d.pollAllMonitoringFlows()

		// Check for inactivity logging
		if len(dispatchUnits) == 0 {
			now := time.Now()
			if now.After(d.nextBackoffLog) {
				timeSinceActivity := now.Sub(d.lastActivity)

				// Count monitoring flows
				flows, _ := d.loadMonitoringFlows()
				registeredPRs := 0
				if d.prPoller != nil {
					registeredPRs = len(d.prPoller.ListRegistered())
				}
				if len(flows) > 0 {
					log.Printf("[%s] No new dispatch activity for %s (%d flows being monitored, %d PR(s) registered)",
						d.dispatcherID, timeSinceActivity.Round(time.Second), len(flows), registeredPRs)
				} else {
					log.Printf("[%s] No new dispatch activity for %s (monitoring %d PR(s))",
						d.dispatcherID, timeSinceActivity.Round(time.Second), registeredPRs)
				}

				// Advance to next backoff level if not at max
				if d.backoffIndex < len(backoffLevels)-1 {
					d.backoffIndex++
				}
				d.nextBackoffLog = now.Add(backoffLevels[d.backoffIndex])
			}
		}

		time.Sleep(checkInterval)
	}
}

// reregisterActiveFlows scans existing flow records and re-registers any active PRs for monitoring
func (d *Dispatcher) reregisterActiveFlows() {
	if d.prPoller == nil {
		return
	}

	// Scan repo-isolation flow directories
	flowsDir := filepath.Join(d.dispatcherLive, "flows", "repo-isolation")
	entries, err := os.ReadDir(flowsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[%s] Warning: could not scan flows directory: %v", d.dispatcherID, err)
		}
		return
	}

	registered := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		flowDir := filepath.Join(flowsDir, entry.Name())

		// Check if this flow has terraform state (meaning it was successfully applied)
		statePath := filepath.Join(flowDir, "terraform.tfstate")
		if _, err := os.Stat(statePath); os.IsNotExist(err) {
			continue
		}

		// Get outputs
		prURL, err := d.getTerraformOutput(flowDir, "pr_url")
		if err != nil || prURL == "" {
			continue
		}

		repoFullName, _ := d.getTerraformOutput(flowDir, "isolation_repo_full_name")
		prNumberStr, _ := d.getTerraformOutput(flowDir, "pr_number")

		if repoFullName == "" || prNumberStr == "" {
			continue
		}

		var prNumber int
		fmt.Sscanf(prNumberStr, "%d", &prNumber)

		parts := strings.SplitN(repoFullName, "/", 2)
		if len(parts) != 2 || prNumber <= 0 {
			continue
		}

		owner, repo := parts[0], parts[1]
		flowID := entry.Name()

		d.prPoller.Register(prpoller.PRRegistration{
			Owner:  owner,
			Repo:   repo,
			Number: prNumber,
			// NOTE: TerraformAction is intentionally NOT set here.
			// The pollAllMonitoringFlows loop already runs terraform apply every 10s,
			// so we don't need the PR poller to also trigger terraform - that causes
			// state lock conflicts. The PR poller is used only for logging/awareness.
			OnChange: func(event prpoller.ChangeEvent) {
				log.Printf("[%s] Detected %d new comment(s) on %s (will be processed by monitoring loop)",
					d.dispatcherID, len(event.NewComments), prURL)
			},
		})
		registered++
		log.Printf("[%s] Re-registered existing flow %s: %s/%s#%d", d.dispatcherID, flowID, owner, repo, prNumber)
	}

	if registered > 0 {
		log.Printf("[%s] Re-registered %d existing flow(s) for PR monitoring", d.dispatcherID, registered)
	}
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

func main() {
	// CLI flags for single-shot mode (--once)
	onceFlag := flag.Bool("once", false, "Single-shot mode: dispatch one work unit and exit")
	instructionFlag := flag.String("i", "", "Instruction to dispatch (requires --once)")
	reportFlag := flag.String("r", "", "Report type to dispatch: custom, daily, weekly, monthly (requires --once)")
	contentFlag := flag.String("c", "", "Content for custom reports (requires --once)")
	modeFlag := flag.String("m", "prompt", "Mode for instructions: prompt or execute (requires --once)")
	agentFlag := flag.String("a", "", "Agent to use (optional, requires --once)")
	timeoutFlag := flag.Duration("t", defaultTimeout, "Timeout for dispatch operation (requires --once)")
	asyncFlag := flag.Bool("async", false, "Dispatch asynchronously without waiting (requires --once)")
	checkFlag := flag.String("check", "", "Check status of a work unit by ID (requires --once)")
	waitFlag := flag.String("wait", "", "Wait for a work unit to complete by ID (requires --once)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "agent-dispatch: Watch for and process dispatch work units\n\n")
		fmt.Fprintf(os.Stderr, "By default, runs in watch mode, continuously monitoring for DISPATCH.json/md files.\n")
		fmt.Fprintf(os.Stderr, "Use --once for single-shot dispatch operations.\n\n")
		fmt.Fprintf(os.Stderr, "WATCH MODE (default):\n")
		fmt.Fprintf(os.Stderr, "  agent-dispatch\n\n")
		fmt.Fprintf(os.Stderr, "SINGLE-SHOT MODE (--once):\n")
		fmt.Fprintf(os.Stderr, "  agent-dispatch --once -i \"instruction\" [-m mode] [-a agent] [-t timeout] [--async]\n")
		fmt.Fprintf(os.Stderr, "  agent-dispatch --once -r type [-c content] [-a agent] [-t timeout] [--async]\n")
		fmt.Fprintf(os.Stderr, "  agent-dispatch --once --check <work-unit-id>\n")
		fmt.Fprintf(os.Stderr, "  agent-dispatch --once --wait <work-unit-id> [-t timeout]\n\n")
		fmt.Fprintf(os.Stderr, "Environment Variables:\n")
		fmt.Fprintf(os.Stderr, "  INPUT_DIR         Input directory (default: %s)\n", defaultInputDir)
		fmt.Fprintf(os.Stderr, "  OUTPUT_DIR        Output directory (default: %s)\n", defaultOutputDir)
		fmt.Fprintf(os.Stderr, "  RECORDS_DIR       Records directory (default: %s)\n", defaultRecordsDir)
		fmt.Fprintf(os.Stderr, "  DISPATCHER_LIVE   Dispatcher live directory for terraform configs (default: %s)\n", defaultDispatcherLive)
		fmt.Fprintf(os.Stderr, "  GITHUB_PAT        GitHub Personal Access Token (required for in-repo/repo-isolation)\n")
		fmt.Fprintf(os.Stderr, "  GH_TOKEN          Alternative to GITHUB_PAT\n")
		fmt.Fprintf(os.Stderr, "  GITHUB_OWNER      GitHub owner for isolation repos (default: je-sidestuff)\n")
		fmt.Fprintf(os.Stderr, "  TERRAFORM_BINARY  Path to terraform binary (default: %s)\n\n", defaultTerraformBinary)
		fmt.Fprintf(os.Stderr, "Watch Mode Dispatch Types:\n")
		fmt.Fprintf(os.Stderr, "  direct         - Transform to INSTRUCTION.json for worker pickup (fire-and-forget)\n")
		fmt.Fprintf(os.Stderr, "  in-repo        - Create PR in target repo, monitor until merged/closed, then cleanup\n")
		fmt.Fprintf(os.Stderr, "  repo-isolation - Create isolation repo, monitor PR, re-integrate on merge, then cleanup\n\n")
		fmt.Fprintf(os.Stderr, "Notes:\n")
		fmt.Fprintf(os.Stderr, "  - DISPATCH.md files auto-transform to DISPATCH.json with type='direct'\n")
		fmt.Fprintf(os.Stderr, "  - Terraform configs are stored in DISPATCHER_LIVE/flows/{in-repo,repo-isolation}/\n")
		fmt.Fprintf(os.Stderr, "  - Flows are polled for PR merge/close and cleaned up automatically\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	dispatcher := NewDispatcher()

	if err := dispatcher.ensureDirectories(); err != nil {
		log.Fatalf("Failed to ensure directories: %v", err)
	}

	// Check if any single-shot flags are used (they imply --once)
	singleShotFlagsUsed := *instructionFlag != "" || *reportFlag != "" || *checkFlag != "" || *waitFlag != ""

	// If single-shot flags used without --once, enable --once automatically
	if singleShotFlagsUsed {
		*onceFlag = true
	}

	// Single-shot mode (--once)
	if *onceFlag {
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
				fmt.Printf("Check status with: agent-dispatch --once --check %s\n", workUnitID)
				fmt.Printf("Wait for completion with: agent-dispatch --once --wait %s\n", workUnitID)
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
				fmt.Printf("Check status with: agent-dispatch --once --check %s\n", workUnitID)
				fmt.Printf("Wait for completion with: agent-dispatch --once --wait %s\n", workUnitID)
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

		// --once specified but no action
		fmt.Fprintf(os.Stderr, "Error: --once requires -i, -r, --check, or --wait flag\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Watch mode (default)
	dispatcher.runWatchLoop()
}
