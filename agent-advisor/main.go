package main

import (
	"encoding/json"
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
	defaultAdvisorConfigDir = "/workspaces/slopspaces/advisor/config"
	defaultRequestDir       = "/workspaces/slopspaces/input/any"
	defaultRecordsDir       = "/workspaces/slopspaces/agent-records/"
	checkInterval           = 10 * time.Second
	defaultAgent            = "claude"
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

// AdvisorConfig represents a single advisor configuration
type AdvisorConfig struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ScheduleAt  string   `json:"schedule_at"` // Time of day to run (e.g., "08:00")
	Topics      []string `json:"topics"`      // Areas to advise on
	Enabled     bool     `json:"enabled"`
}

// AdvisorState tracks when an advisor last ran
type AdvisorState struct {
	LastRun    time.Time `json:"last_run"`
	LastResult string    `json:"last_result"` // date string of last successful run
	RunCount   int       `json:"run_count"`
}

// Instruction represents the JSON structure for instruction work units
type Instruction struct {
	Instruction string `json:"instruction"`
	Mode        string `json:"mode"`
}

// ExtractedFile represents a file extracted from agent output
type ExtractedFile struct {
	Filename string
	Content  string
}

// AdvisorManager manages the advisor processing loop
type AdvisorManager struct {
	managerID    string
	configDir    string
	requestDir   string
	recordsDir   string
	currentAgent string
	configs      map[string]*AdvisorConfig
	states       map[string]*AdvisorState
	lastActivity time.Time
	backoffIndex int
	nextBackoffLog time.Time
}

// NewAdvisorManager creates a new advisor manager instance
func NewAdvisorManager() *AdvisorManager {
	configDir := os.Getenv("ADVISOR_CONFIG_DIR")
	if configDir == "" {
		configDir = defaultAdvisorConfigDir
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
	managerID := uuid.New().String()[:8]

	return &AdvisorManager{
		managerID:      managerID,
		configDir:      configDir,
		requestDir:     requestDir,
		recordsDir:     recordsDir,
		currentAgent:   currentAgent,
		configs:        make(map[string]*AdvisorConfig),
		states:         make(map[string]*AdvisorState),
		lastActivity:   now,
		backoffIndex:   0,
		nextBackoffLog: now.Add(backoffLevels[0]),
	}
}

// ensureDirectories creates necessary directories
func (m *AdvisorManager) ensureDirectories() error {
	dirs := []string{
		m.configDir,
		m.requestDir,
		filepath.Join(m.recordsDir, "advisor"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// createDefaultConfigs creates the default advisor configurations if none exist
func (m *AdvisorManager) createDefaultConfigs() error {
	entries, err := os.ReadDir(m.configDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read config directory: %w", err)
	}

	// Check if any config files exist
	hasConfigs := false
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			hasConfigs = true
			break
		}
	}

	if !hasConfigs {
		log.Printf("[%s] No configs found, creating default configs", m.managerID)

		defaultConfig := AdvisorConfig{
			Name:        "default-daily-advisor",
			Description: "Daily advisor reviewing system health and suggesting improvements",
			ScheduleAt:  "08:00",
			Topics: []string{
				"agent activity and health",
				"pending work unit backlog",
				"recent errors or failures",
			},
			Enabled: true,
		}

		data, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal default config: %w", err)
		}

		configPath := filepath.Join(m.configDir, "default-daily-advisor.json")
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write default config: %w", err)
		}
		log.Printf("[%s] Created default advisor config: %s", m.managerID, configPath)
	}

	return nil
}

// loadConfigs reads all advisor configurations from the config directory
func (m *AdvisorManager) loadConfigs() error {
	entries, err := os.ReadDir(m.configDir)
	if err != nil {
		return fmt.Errorf("failed to read config directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		configPath := filepath.Join(m.configDir, entry.Name())
		data, err := os.ReadFile(configPath)
		if err != nil {
			log.Printf("[%s] Warning: failed to read config %s: %v", m.managerID, configPath, err)
			continue
		}

		var config AdvisorConfig
		if err := json.Unmarshal(data, &config); err != nil {
			log.Printf("[%s] Warning: failed to parse config %s: %v", m.managerID, configPath, err)
			continue
		}

		if !config.Enabled {
			log.Printf("[%s] Config %s is disabled, skipping", m.managerID, config.Name)
			continue
		}

		m.configs[config.Name] = &config

		if _, exists := m.states[config.Name]; !exists {
			m.states[config.Name] = &AdvisorState{}
		}

		log.Printf("[%s] Loaded advisor config: %s (schedule: %s, topics: %d)",
			m.managerID, config.Name, config.ScheduleAt, len(config.Topics))
	}

	return nil
}

// buildAdvisoryPrompt creates the prompt for the agent
func buildAdvisoryPrompt(config *AdvisorConfig, date string) string {
	topicList := ""
	for i, topic := range config.Topics {
		topicList += fmt.Sprintf("%d. %s\n", i+1, topic)
	}

	return fmt.Sprintf(`You are an advisor agent. Your role is to review the current state of the system and provide actionable recommendations.

## Advisory Session

Date: %s
Advisor: %s
Description: %s

## Topics to Advise On

%s

## Your Task

Review the above topics and produce a single, focused recommendation or suggested action. This will be submitted as a prompt-only instruction for human review and approval before any action is taken.

Keep the recommendation concrete and actionable. Focus on one clear suggestion rather than a broad overview.

## Output Format

Output exactly ONE file in the following format:

`+"```"+`json INSTRUCTION.json
{
  "instruction": "Your specific recommendation or suggested action here. Be concrete and actionable.",
  "mode": "prompt"
}
`+"```"+`

The instruction will be reviewed and approved by a human before execution. Write it as a clear directive.
`, date, config.Name, config.Description, topicList)
}

// findInvokeScript locates the invoke-agent.sh script
func findInvokeScript() string {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "invoke-agent.sh")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	cwd, err := os.Getwd()
	if err == nil {
		candidate := filepath.Join(cwd, "invoke-agent.sh")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	if err == nil {
		candidate := filepath.Join(cwd, "ambiguous-agent", "invoke-agent.sh")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	if err == nil {
		candidate := filepath.Join(filepath.Dir(cwd), "ambiguous-agent", "invoke-agent.sh")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	path, err := exec.LookPath("invoke-agent.sh")
	if err == nil {
		return path
	}

	return ""
}

// extractFilesFromOutput parses agent output to extract INSTRUCTION files from code blocks
func extractFilesFromOutput(output string) []ExtractedFile {
	var files []ExtractedFile

	pattern := regexp.MustCompile("(?s)```(?:json|markdown|md)?\\s*(INSTRUCTION\\.(?:md|json))\\s*\n(.*?)```")

	matches := pattern.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			filename := strings.TrimSpace(match[1])
			content := match[2]

			if filename == "INSTRUCTION.md" || filename == "INSTRUCTION.json" {
				files = append(files, ExtractedFile{
					Filename: filename,
					Content:  content,
				})
			}
		}
	}

	return files
}

// executeAgent runs the agent in prompt-only mode and captures output
func (m *AdvisorManager) executeAgent(workDir, prompt string) (string, int, error) {
	invokeScript := findInvokeScript()
	if invokeScript == "" {
		return "", 1, fmt.Errorf("invoke-agent.sh not found")
	}

	promptFile := filepath.Join(workDir, ".prompt.tmp")
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return "", 1, fmt.Errorf("failed to write prompt file: %w", err)
	}
	defer os.Remove(promptFile)

	cmdArgs := []string{"-p", "-a", m.currentAgent, "-f", promptFile}

	cmd := exec.Command(invokeScript, cmdArgs...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"AGENT_PRESET="+m.currentAgent,
		"AGENT_RECORDS_PATH="+m.recordsDir,
	)

	outputFile := filepath.Join(workDir, "agent_output.txt")
	outFile, err := os.Create(outputFile)
	if err != nil {
		return "", 1, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	cmd.Stderr = outFile
	cmd.Stdin = os.Stdin

	log.Printf("[%s] Invoking agent %s in prompt-only mode", m.managerID, m.currentAgent)

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", 1, fmt.Errorf("failed to run agent: %w", err)
		}
	}

	output, err := os.ReadFile(outputFile)
	if err != nil {
		return "", exitCode, fmt.Errorf("failed to read output file: %w", err)
	}

	return string(output), exitCode, nil
}

// runAdvisor executes a single advisory session for a given config
func (m *AdvisorManager) runAdvisor(config *AdvisorConfig, date string) error {
	log.Printf("[%s] Running advisor: %s (date: %s)", m.managerID, config.Name, date)

	// Create a temp work directory for this session
	sessionID := fmt.Sprintf("advisor_%s_%s", date, uuid.New().String()[:8])
	workDir := filepath.Join(os.TempDir(), sessionID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	prompt := buildAdvisoryPrompt(config, date)

	output, exitCode, err := m.executeAgent(workDir, prompt)
	if err != nil {
		return fmt.Errorf("agent execution failed: %w", err)
	}

	extractedFiles := extractFilesFromOutput(output)
	if len(extractedFiles) == 0 {
		return fmt.Errorf("no INSTRUCTION file extracted from agent output (exit: %d)", exitCode)
	}

	// Create work unit in request dir
	workUnitName := fmt.Sprintf("advisor_%s_%s", date, uuid.New().String()[:8])
	workUnitPath := filepath.Join(m.requestDir, workUnitName)
	if err := os.MkdirAll(workUnitPath, 0755); err != nil {
		return fmt.Errorf("failed to create work unit dir: %w", err)
	}

	for _, file := range extractedFiles {
		filePath := filepath.Join(workUnitPath, file.Filename)
		if err := os.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
			log.Printf("[%s] Warning: failed to write %s: %v", m.managerID, file.Filename, err)
		} else {
			log.Printf("[%s] Wrote %s to %s", m.managerID, file.Filename, workUnitPath)
		}
	}

	// Write a summary note alongside the instruction
	notePath := filepath.Join(workUnitPath, "ADVISOR_SOURCE.md")
	noteContent := fmt.Sprintf("# Advisor Source\n\nAdvisor: %s\nDate: %s\nTopics: %s\nAgent: %s\nSession: %s\n",
		config.Name, date, strings.Join(config.Topics, ", "), m.currentAgent, sessionID)
	_ = os.WriteFile(notePath, []byte(noteContent), 0644)

	log.Printf("[%s] Created advisory work unit: %s", m.managerID, workUnitName)

	// Write record
	m.writeAdvisorRecord(config.Name, date, sessionID, exitCode, workUnitName, nil)

	return nil
}

// writeAdvisorRecord writes a record of the advisor run
func (m *AdvisorManager) writeAdvisorRecord(configName, date, sessionID string, exitCode int, workUnitName string, runErr error) {
	success := runErr == nil
	errStr := ""
	if runErr != nil {
		errStr = fmt.Sprintf(`,
  "error": %q`, runErr.Error())
	}

	recordContent := fmt.Sprintf(`{
  "manager_id": "%s",
  "config_name": "%s",
  "date": "%s",
  "session_id": "%s",
  "timestamp": "%s",
  "agent": "%s",
  "exit_code": %d,
  "work_unit": "%s",
  "success": %v%s
}`,
		m.managerID, configName, date, sessionID,
		time.Now().Format(time.RFC3339), m.currentAgent,
		exitCode, workUnitName, success, errStr)

	recordFilename := fmt.Sprintf("%s_%s_%d.json", m.managerID, configName, time.Now().Unix())
	recordPath := filepath.Join(m.recordsDir, "advisor", recordFilename)

	if err := os.WriteFile(recordPath, []byte(recordContent), 0644); err != nil {
		log.Printf("[%s] Warning: failed to write advisor record: %v", m.managerID, err)
	}
}

// todayDate returns today's date in YYYY-MM-DD format
func todayDate() string {
	return time.Now().Format("2006-01-02")
}

// processAdvisors checks all configs and runs any that are due
func (m *AdvisorManager) processAdvisors() int {
	today := todayDate()
	now := time.Now()
	ran := 0

	for name, config := range m.configs {
		state := m.states[name]

		// Skip if already ran today
		if state.LastResult == today {
			continue
		}

		// Parse scheduled time
		scheduleTime, err := time.Parse("15:04", config.ScheduleAt)
		if err != nil {
			log.Printf("[%s] Invalid schedule_at for %s: %v", m.managerID, name, err)
			continue
		}

		// Check if current time is past the scheduled time
		currentTime := time.Date(2000, 1, 1, now.Hour(), now.Minute(), 0, 0, time.UTC)
		scheduledTime := time.Date(2000, 1, 1, scheduleTime.Hour(), scheduleTime.Minute(), 0, 0, time.UTC)

		if currentTime.Before(scheduledTime) {
			continue // Not yet time
		}

		if err := m.runAdvisor(config, today); err != nil {
			log.Printf("[%s] Error running advisor %s: %v", m.managerID, name, err)
			m.writeAdvisorRecord(name, today, "", 1, "", err)
		} else {
			state.LastResult = today
			state.LastRun = now
			state.RunCount++
			ran++
		}
	}

	return ran
}

// run is the main processing loop
func (m *AdvisorManager) run() {
	log.Printf("[%s] Agent advisor started", m.managerID)
	log.Printf("[%s] Config dir: %s", m.managerID, m.configDir)
	log.Printf("[%s] Request dir: %s", m.managerID, m.requestDir)
	log.Printf("[%s] Records dir: %s", m.managerID, filepath.Join(m.recordsDir, "advisor"))
	log.Printf("[%s] Default agent: %s", m.managerID, m.currentAgent)

	for name, config := range m.configs {
		log.Printf("[%s] Advisor '%s': schedule=%s, topics=%d",
			m.managerID, name, config.ScheduleAt, len(config.Topics))
	}

	for {
		ran := m.processAdvisors()

		if ran > 0 {
			m.lastActivity = time.Now()
			m.backoffIndex = 0
			m.nextBackoffLog = m.lastActivity.Add(backoffLevels[0])
		} else {
			now := time.Now()
			if now.After(m.nextBackoffLog) {
				timeSinceActivity := now.Sub(m.lastActivity)
				log.Printf("[%s] No advisor activity for %s", m.managerID, timeSinceActivity.Round(time.Second))

				if m.backoffIndex < len(backoffLevels)-1 {
					m.backoffIndex++
				}
				m.nextBackoffLog = now.Add(backoffLevels[m.backoffIndex])
			}
		}

		time.Sleep(checkInterval)
	}
}

// runOnce processes all due advisors once and exits
func (m *AdvisorManager) runOnce() {
	ran := m.processAdvisors()
	if ran == 0 {
		log.Printf("[%s] No advisors were due to run", m.managerID)
	} else {
		log.Printf("[%s] Ran %d advisor(s)", m.managerID, ran)
	}
}

func main() {
	watchFlag := flag.Bool("watch", false, "Start the advisor watch loop (default behavior)")
	onceFlag := flag.Bool("once", false, "Run due advisors once and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "agent-advisor: Heuristic harness agent that produces advisory instructions on a schedule\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  agent-advisor [--watch|--once]\n\n")
		fmt.Fprintf(os.Stderr, "Environment Variables:\n")
		fmt.Fprintf(os.Stderr, "  ADVISOR_CONFIG_DIR  Directory containing advisor configs (default: %s)\n", defaultAdvisorConfigDir)
		fmt.Fprintf(os.Stderr, "  REQUEST_DIR         Directory to place work units for downstream processing (default: %s)\n", defaultRequestDir)
		fmt.Fprintf(os.Stderr, "  RECORDS_DIR         Records directory (default: %s)\n", defaultRecordsDir)
		fmt.Fprintf(os.Stderr, "  AGENT_PRESET        Agent to use (default: %s)\n\n", defaultAgent)
		fmt.Fprintf(os.Stderr, "How it works:\n")
		fmt.Fprintf(os.Stderr, "  1. Reads JSON configs from ADVISOR_CONFIG_DIR (auto-creates defaults if none exist)\n")
		fmt.Fprintf(os.Stderr, "  2. On schedule, invokes an agent in prompt-only mode (-p) for each enabled advisor\n")
		fmt.Fprintf(os.Stderr, "  3. Extracts INSTRUCTION.json (mode: prompt) from agent output\n")
		fmt.Fprintf(os.Stderr, "  4. Places the instruction in REQUEST_DIR for human review and approval\n\n")
		fmt.Fprintf(os.Stderr, "Config file format (JSON):\n")
		fmt.Fprintf(os.Stderr, "  {\n")
		fmt.Fprintf(os.Stderr, "    \"name\": \"my-advisor\",\n")
		fmt.Fprintf(os.Stderr, "    \"description\": \"What this advisor does\",\n")
		fmt.Fprintf(os.Stderr, "    \"schedule_at\": \"08:00\",\n")
		fmt.Fprintf(os.Stderr, "    \"topics\": [\"topic 1\", \"topic 2\"],\n")
		fmt.Fprintf(os.Stderr, "    \"enabled\": true\n")
		fmt.Fprintf(os.Stderr, "  }\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	manager := NewAdvisorManager()

	if err := manager.ensureDirectories(); err != nil {
		log.Fatalf("Failed to ensure directories: %v", err)
	}

	if err := manager.createDefaultConfigs(); err != nil {
		log.Fatalf("Failed to create default configs: %v", err)
	}

	if err := manager.loadConfigs(); err != nil {
		log.Fatalf("Failed to load configs: %v", err)
	}

	if *onceFlag {
		manager.runOnce()
		return
	}

	// Default to watch mode
	_ = *watchFlag
	manager.run()
}
