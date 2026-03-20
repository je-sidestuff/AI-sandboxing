package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Default paths
const (
	defaultInputDir        = "/workspaces/slopspaces/input/"
	defaultOutputDir       = "/workspaces/slopspaces/output/"
	defaultRecordsDir      = "/workspaces/slopspaces/agent-records/"
	defaultDispatcherLive  = "/workspaces/slopspaces/dispatcher/live"
	checkInterval          = 10 * time.Second
	defaultTerraformBinary = "terraform"
)

// Dispatch type constants
const (
	DispatchTypeDirect = "direct"
	DispatchTypeInRepo = "in-repo"
)

// Exponential backoff levels for logging inactivity (same as agent-worker)
var backoffLevels = []time.Duration{
	30 * time.Second,
	5 * time.Minute,
	1 * time.Hour,
	24 * time.Hour,
}

// Dispatch represents the JSON structure for dispatch work units
type Dispatch struct {
	Type        string            `json:"type"`                  // "direct" or "in-repo"
	Instruction string            `json:"instruction"`           // The instruction to dispatch
	Mode        string            `json:"mode,omitempty"`        // "prompt" or "execute" (default: "execute")
	Agent       string            `json:"agent,omitempty"`       // Optional agent override
	TargetRepo  string            `json:"target_repo,omitempty"` // For in-repo: "owner/repo"
	PRTitle     string            `json:"pr_title,omitempty"`    // For in-repo: optional PR title
	PRBody      string            `json:"pr_body,omitempty"`     // For in-repo: optional PR body
	Timestamp   string            `json:"timestamp,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"` // Additional metadata
}

// Instruction represents the JSON structure for work instructions (for direct dispatch)
type Instruction struct {
	Instruction string `json:"instruction"`
	Mode        string `json:"mode"`
	Agent       string `json:"agent,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

// DispatchUnit represents a discovered dispatch work unit
type DispatchUnit struct {
	Path     string
	ID       string
	Dispatch *Dispatch
}

// FlowRecord holds metadata for tracked terraform flows
type FlowRecord struct {
	DispatcherID string `json:"dispatcher_id"`
	FlowID       string `json:"flow_id"`
	DispatchType string `json:"dispatch_type"`
	DispatchPath string `json:"dispatch_path"`
	TFConfigDir  string `json:"tf_config_dir,omitempty"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time,omitempty"`
	Status       string `json:"status"` // "pending", "running", "completed", "failed"
	Error        string `json:"error,omitempty"`
	PRUrl        string `json:"pr_url,omitempty"`
}

// DispatchWatcher manages the dispatch watch loop
type DispatchWatcher struct {
	watcherID       string
	inputDir        string
	outputDir       string
	recordsDir      string
	dispatcherLive  string
	terraformBinary string
	githubPAT       string
	lastActivity    time.Time
	backoffIndex    int
	nextBackoffLog  time.Time
}

// NewDispatchWatcher creates a new watcher instance
func NewDispatchWatcher() *DispatchWatcher {
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

	now := time.Now()
	watcherID := uuid.New().String()[:8]

	return &DispatchWatcher{
		watcherID:       watcherID,
		inputDir:        inputDir,
		outputDir:       outputDir,
		recordsDir:      recordsDir,
		dispatcherLive:  dispatcherLive,
		terraformBinary: terraformBinary,
		githubPAT:       githubPAT,
		lastActivity:    now,
		backoffIndex:    0,
		nextBackoffLog:  now.Add(backoffLevels[0]),
	}
}

// ensureDirectories creates necessary directories
func (w *DispatchWatcher) ensureDirectories() error {
	dirs := []string{
		filepath.Join(w.inputDir, "any"),
		w.outputDir,
		filepath.Join(w.recordsDir, "dispatch-watch"),
		filepath.Join(w.dispatcherLive, "flows", "in-repo"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// generateFlowID creates a unique flow identifier
func generateFlowID(dispatchType string) string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	shortID := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%s_%s", dispatchType, timestamp, shortID)
}

// checkForDispatchUnits scans the input directory for DISPATCH.json/md files
func (w *DispatchWatcher) checkForDispatchUnits() ([]DispatchUnit, error) {
	anyDir := filepath.Join(w.inputDir, "any")
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
				dispatch, err := w.handleDispatchFiles(folderPath)
				if err != nil {
					log.Printf("[%s] Error handling dispatch files in %s: %v", w.watcherID, entry.Name(), err)
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
func (w *DispatchWatcher) handleDispatchFiles(folderPath string) (*Dispatch, error) {
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
		if dispatch.Type != DispatchTypeDirect && dispatch.Type != DispatchTypeInRepo {
			return nil, fmt.Errorf("invalid dispatch type: %s (must be 'direct' or 'in-repo')", dispatch.Type)
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

		log.Printf("[%s] Converted DISPATCH.md to DISPATCH.json (type: direct)", w.watcherID)
		return &dispatch, nil
	}

	return nil, fmt.Errorf("no dispatch file found")
}

// processDispatchUnit handles a single dispatch work unit
func (w *DispatchWatcher) processDispatchUnit(unit DispatchUnit) error {
	log.Printf("[%s] Processing dispatch unit: %s (type: %s)", w.watcherID, unit.ID, unit.Dispatch.Type)

	startTime := time.Now()

	// Create DISPATCHING.md to mark we're working on it
	dispatchingMD := filepath.Join(unit.Path, "DISPATCHING.md")
	dispatchingContent := fmt.Sprintf("# Dispatching\n\nWatcher ID: %s\nStarted: %s\nType: %s\n",
		w.watcherID, startTime.Format(time.RFC3339), unit.Dispatch.Type)
	if err := os.WriteFile(dispatchingMD, []byte(dispatchingContent), 0644); err != nil {
		return fmt.Errorf("failed to create DISPATCHING.md: %w", err)
	}

	var err error
	switch unit.Dispatch.Type {
	case DispatchTypeDirect:
		err = w.processDirectDispatch(unit)
	case DispatchTypeInRepo:
		err = w.processInRepoDispatch(unit)
	default:
		err = fmt.Errorf("unsupported dispatch type: %s", unit.Dispatch.Type)
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	if err != nil {
		// Mark as failed
		w.markDispatchFailed(unit, err, startTime, endTime)
		return err
	}

	// Mark as completed and move to output
	w.markDispatchComplete(unit, startTime, endTime)

	log.Printf("[%s] Completed dispatch unit: %s (duration: %s)", w.watcherID, unit.ID, duration.Round(time.Millisecond))
	return nil
}

// processDirectDispatch handles direct dispatch (creates INSTRUCTION.json in-place)
func (w *DispatchWatcher) processDirectDispatch(unit DispatchUnit) error {
	log.Printf("[%s] Processing direct dispatch: %s", w.watcherID, unit.ID)

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

	log.Printf("[%s] Direct dispatch transformed to INSTRUCTION.json, ready for worker pickup", w.watcherID)
	return nil
}

// processInRepoDispatch handles in-repo dispatch with terraform lifecycle
func (w *DispatchWatcher) processInRepoDispatch(unit DispatchUnit) error {
	log.Printf("[%s] Processing in-repo dispatch: %s", w.watcherID, unit.ID)

	if w.githubPAT == "" {
		return fmt.Errorf("GITHUB_PAT or GH_TOKEN environment variable is required for in-repo dispatch")
	}

	targetRepo := unit.Dispatch.TargetRepo
	if targetRepo == "" {
		targetRepo = "je-sidestuff/AI-sandboxing" // Default
	}

	// Generate a unique flow ID
	flowID := generateFlowID(DispatchTypeInRepo)

	// Create the terraform config directory
	tfConfigDir := filepath.Join(w.dispatcherLive, "flows", "in-repo", flowID)
	if err := os.MkdirAll(tfConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create terraform config directory: %w", err)
	}

	// Write the flow record
	flowRecord := FlowRecord{
		DispatcherID: w.watcherID,
		FlowID:       flowID,
		DispatchType: DispatchTypeInRepo,
		DispatchPath: unit.Path,
		TFConfigDir:  tfConfigDir,
		StartTime:    time.Now().Format(time.RFC3339),
		Status:       "running",
	}
	w.writeFlowRecord(flowRecord)

	// Create the terraform configuration
	if err := w.createInRepoTerraformConfig(tfConfigDir, flowID, targetRepo, unit.Dispatch); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = err.Error()
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		w.writeFlowRecord(flowRecord)
		return fmt.Errorf("failed to create terraform config: %w", err)
	}

	// Run terraform init
	log.Printf("[%s] Running terraform init in %s", w.watcherID, tfConfigDir)
	if err := w.runTerraform(tfConfigDir, "init"); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = fmt.Sprintf("terraform init failed: %v", err)
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		w.writeFlowRecord(flowRecord)
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Run terraform apply
	log.Printf("[%s] Running terraform apply in %s", w.watcherID, tfConfigDir)
	if err := w.runTerraform(tfConfigDir, "apply", "-auto-approve"); err != nil {
		flowRecord.Status = "failed"
		flowRecord.Error = fmt.Sprintf("terraform apply failed: %v", err)
		flowRecord.EndTime = time.Now().Format(time.RFC3339)
		w.writeFlowRecord(flowRecord)
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	// Terraform lifecycle is complete - PR has been created and work dispatched
	// Mark the flow as complete
	flowRecord.Status = "completed"
	flowRecord.EndTime = time.Now().Format(time.RFC3339)

	// Try to get PR URL from terraform output
	prURL, _ := w.getTerraformOutput(tfConfigDir, "pr_url")
	if prURL != "" {
		flowRecord.PRUrl = prURL
		log.Printf("[%s] In-repo dispatch complete, PR URL: %s", w.watcherID, prURL)
	}

	w.writeFlowRecord(flowRecord)

	log.Printf("[%s] In-repo dispatch terraform lifecycle complete for flow %s", w.watcherID, flowID)
	return nil
}

// createInRepoTerraformConfig creates the terraform configuration for in-repo dispatch
func (w *DispatchWatcher) createInRepoTerraformConfig(configDir, flowID, targetRepo string, dispatch *Dispatch) error {
	// Get the path to the in-repo module
	// Try relative to the executable first
	exe, err := os.Executable()
	var modulePath string
	if err == nil {
		modulePath = filepath.Join(filepath.Dir(exe), "..", "agent-dispatch", "modules", "containment", "in-repo")
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
`, w.githubPAT)

	// Write all the files
	files := map[string]string{
		"providers.tf":    providersTF,
		"variables.tf":    variablesTF,
		"main.tf":         mainTF,
		"outputs.tf":      outputsTF,
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
func (w *DispatchWatcher) runTerraform(workDir string, args ...string) error {
	cmd := exec.Command(w.terraformBinary, args...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// getTerraformOutput retrieves a terraform output value
func (w *DispatchWatcher) getTerraformOutput(workDir, outputName string) (string, error) {
	cmd := exec.Command(w.terraformBinary, "output", "-raw", outputName)
	cmd.Dir = workDir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// markDispatchComplete marks a dispatch as complete and moves to output
func (w *DispatchWatcher) markDispatchComplete(unit DispatchUnit, startTime, endTime time.Time) {
	duration := endTime.Sub(startTime)

	// For direct dispatch, the folder stays in place for worker pickup
	// For in-repo dispatch, we move to output since terraform lifecycle is complete

	if unit.Dispatch.Type == DispatchTypeInRepo {
		// Move folder to output directory
		destPath := filepath.Join(w.outputDir, unit.ID)
		if err := os.Rename(unit.Path, destPath); err != nil {
			log.Printf("[%s] Warning: failed to move dispatch to output: %v", w.watcherID, err)
			return
		}

		// Create DISPATCH_PROCESSED.md in the destination
		processedMD := filepath.Join(destPath, "DISPATCH_PROCESSED.md")
		processedContent := fmt.Sprintf("# Dispatch Processed\n\nWatcher ID: %s\nStarted: %s\nCompleted: %s\nDuration: %s\nType: %s\n",
			w.watcherID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
			duration.Round(time.Millisecond).String(), unit.Dispatch.Type)
		if err := os.WriteFile(processedMD, []byte(processedContent), 0644); err != nil {
			log.Printf("Warning: failed to create DISPATCH_PROCESSED.md: %v", err)
		}
	}
	// For direct dispatch, we don't move - we just transformed it for the worker

	// Write dispatch record
	w.writeDispatchRecord(unit, startTime, endTime, true, "")
}

// markDispatchFailed marks a dispatch as failed
func (w *DispatchWatcher) markDispatchFailed(unit DispatchUnit, dispatchErr error, startTime, endTime time.Time) {
	duration := endTime.Sub(startTime)

	// Move to output with error marker
	destPath := filepath.Join(w.outputDir, unit.ID)
	if err := os.Rename(unit.Path, destPath); err != nil {
		log.Printf("[%s] Warning: failed to move failed dispatch to output: %v", w.watcherID, err)
		destPath = unit.Path // Use original path for the error file
	}

	// Create DISPATCH_FAILED.md
	failedMD := filepath.Join(destPath, "DISPATCH_FAILED.md")
	failedContent := fmt.Sprintf("# Dispatch Failed\n\nWatcher ID: %s\nStarted: %s\nFailed: %s\nDuration: %s\nType: %s\nError: %s\n",
		w.watcherID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339),
		duration.Round(time.Millisecond).String(), unit.Dispatch.Type, dispatchErr.Error())
	if err := os.WriteFile(failedMD, []byte(failedContent), 0644); err != nil {
		log.Printf("Warning: failed to create DISPATCH_FAILED.md: %v", err)
	}

	// Write dispatch record
	w.writeDispatchRecord(unit, startTime, endTime, false, dispatchErr.Error())
}

// writeDispatchRecord writes a record of the dispatch operation
func (w *DispatchWatcher) writeDispatchRecord(unit DispatchUnit, startTime, endTime time.Time, success bool, errMsg string) {
	record := map[string]interface{}{
		"watcher_id":    w.watcherID,
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
		log.Printf("[%s] Warning: failed to marshal dispatch record: %v", w.watcherID, err)
		return
	}

	recordFilename := fmt.Sprintf("%s_%s_%d.json", w.watcherID, unit.ID, time.Now().Unix())
	recordPath := filepath.Join(w.recordsDir, "dispatch-watch", recordFilename)

	if err := os.WriteFile(recordPath, recordData, 0644); err != nil {
		log.Printf("[%s] Warning: failed to write dispatch record: %v", w.watcherID, err)
	}
}

// writeFlowRecord writes a flow tracking record
func (w *DispatchWatcher) writeFlowRecord(record FlowRecord) {
	recordData, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		log.Printf("[%s] Warning: failed to marshal flow record: %v", w.watcherID, err)
		return
	}

	recordFilename := fmt.Sprintf("flow_%s.json", record.FlowID)
	recordPath := filepath.Join(w.recordsDir, "dispatch-watch", recordFilename)

	if err := os.WriteFile(recordPath, recordData, 0644); err != nil {
		log.Printf("[%s] Warning: failed to write flow record: %v", w.watcherID, err)
	}
}

// fileExists checks if a file or directory exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// run is the main watch loop
func (w *DispatchWatcher) run() {
	log.Printf("[%s] Dispatch watcher started", w.watcherID)
	log.Printf("[%s] Watching: %s", w.watcherID, filepath.Join(w.inputDir, "any"))
	log.Printf("[%s] Output: %s", w.watcherID, w.outputDir)
	log.Printf("[%s] Records: %s", w.watcherID, filepath.Join(w.recordsDir, "dispatch-watch"))
	log.Printf("[%s] Dispatcher Live: %s", w.watcherID, w.dispatcherLive)

	if w.githubPAT != "" {
		log.Printf("[%s] GitHub PAT: configured (in-repo dispatch enabled)", w.watcherID)
	} else {
		log.Printf("[%s] GitHub PAT: not configured (in-repo dispatch will fail)", w.watcherID)
	}

	for {
		dispatchUnits, err := w.checkForDispatchUnits()
		if err != nil {
			log.Printf("[%s] Error checking for dispatch units: %v", w.watcherID, err)
		}

		if len(dispatchUnits) > 0 {
			// Reset backoff on activity
			w.lastActivity = time.Now()
			w.backoffIndex = 0
			w.nextBackoffLog = w.lastActivity.Add(backoffLevels[0])

			for _, unit := range dispatchUnits {
				if err := w.processDispatchUnit(unit); err != nil {
					log.Printf("[%s] Error processing dispatch unit %s: %v", w.watcherID, unit.ID, err)
				}
			}
		} else {
			// No activity - check if we should log with backoff
			now := time.Now()
			if now.After(w.nextBackoffLog) {
				timeSinceActivity := now.Sub(w.lastActivity)
				log.Printf("[%s] No new dispatch activity for %s", w.watcherID, timeSinceActivity.Round(time.Second))

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
	watchFlag := flag.Bool("watch", false, "Start the dispatch watch loop (default behavior)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "agent-dispatch-watch: Watch for DISPATCH.json/md files and process them\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  agent-dispatch-watch [--watch]\n\n")
		fmt.Fprintf(os.Stderr, "Environment Variables:\n")
		fmt.Fprintf(os.Stderr, "  INPUT_DIR         Input directory (default: %s)\n", defaultInputDir)
		fmt.Fprintf(os.Stderr, "  OUTPUT_DIR        Output directory (default: %s)\n", defaultOutputDir)
		fmt.Fprintf(os.Stderr, "  RECORDS_DIR       Records directory (default: %s)\n", defaultRecordsDir)
		fmt.Fprintf(os.Stderr, "  DISPATCHER_LIVE   Dispatcher live directory for terraform configs (default: %s)\n", defaultDispatcherLive)
		fmt.Fprintf(os.Stderr, "  GITHUB_PAT        GitHub Personal Access Token (required for in-repo dispatch)\n")
		fmt.Fprintf(os.Stderr, "  GH_TOKEN          Alternative to GITHUB_PAT\n")
		fmt.Fprintf(os.Stderr, "  TERRAFORM_BINARY  Path to terraform binary (default: %s)\n\n", defaultTerraformBinary)
		fmt.Fprintf(os.Stderr, "Dispatch Types:\n")
		fmt.Fprintf(os.Stderr, "  direct   - Transform to INSTRUCTION.json for worker pickup (fire-and-forget)\n")
		fmt.Fprintf(os.Stderr, "  in-repo  - Create terraform flow, wait for lifecycle, then complete\n\n")
		fmt.Fprintf(os.Stderr, "Notes:\n")
		fmt.Fprintf(os.Stderr, "  - DISPATCH.md files auto-transform to DISPATCH.json with type='direct'\n")
		fmt.Fprintf(os.Stderr, "  - In-repo terraform configs are stored in DISPATCHER_LIVE/flows/in-repo/\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Default to watch mode
	_ = *watchFlag // Accept flag but always run in watch mode

	watcher := NewDispatchWatcher()

	if err := watcher.ensureDirectories(); err != nil {
		log.Fatalf("Failed to ensure directories: %v", err)
	}

	watcher.run()
}
