package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Default paths
const (
	defaultEventsConfigDir = "/workspaces/slopspaces/events/config"
	defaultInputDir        = "/workspaces/slopspaces/input/"
	checkInterval          = 10 * time.Second
)

// Event type constants
const (
	EventTypeTimer    = "timer"
	EventTypeSchedule = "schedule"
)

// Exponential backoff levels for logging inactivity
var backoffLevels = []time.Duration{
	30 * time.Second,
	5 * time.Minute,
	1 * time.Hour,
	24 * time.Hour,
}

// Random word lists for topic generation
var adjectives = []string{
	"brilliant", "curious", "dazzling", "elegant", "fantastic",
	"graceful", "harmonious", "innovative", "joyful", "keen",
	"luminous", "magnificent", "noble", "optimistic", "peaceful",
	"quirky", "radiant", "serene", "thoughtful", "unique",
	"vibrant", "whimsical", "xenial", "youthful", "zealous",
}

var nouns = []string{
	"algorithm", "butterfly", "cascade", "diamond", "ecosystem",
	"frontier", "galaxy", "horizon", "innovation", "journey",
	"kaleidoscope", "lighthouse", "momentum", "nebula", "oasis",
	"paradigm", "quantum", "renaissance", "symphony", "telescope",
	"universe", "velocity", "wavelength", "xenolith", "zenith",
}

// EventConfig represents a single event configuration
type EventConfig struct {
	Name        string `json:"name"`
	Type        string `json:"type"`          // "timer" or "schedule"
	Description string `json:"description"`   // Human-readable description
	Interval    string `json:"interval"`      // For timer: duration string (e.g., "6h")
	ScheduleAt  string `json:"schedule_at"`   // For schedule: time of day to run (e.g., "09:00")
	ReportType  string `json:"report_type"`   // Report type to generate: "daily", "weekly", "monthly", "custom"
	TopicStyle  string `json:"topic_style"`   // For custom reports: "random_words" to use adjective+noun
	Enabled     bool   `json:"enabled"`
}

// EventState tracks the state of an event
type EventState struct {
	LastRun    time.Time `json:"last_run"`
	NextRun    time.Time `json:"next_run"`
	RunCount   int       `json:"run_count"`
	LastResult string    `json:"last_result"`
}

// Report represents the JSON structure for report work units (same as agent-worker)
type Report struct {
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	Agent     string `json:"agent,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Date      string `json:"date,omitempty"` // For daily reports: the date being reported on
}

// EventsManager manages the event processing loop
type EventsManager struct {
	managerID       string
	eventsConfigDir string
	inputDir        string
	configs         map[string]*EventConfig
	states          map[string]*EventState
	lastActivity    time.Time
	backoffIndex    int
	nextBackoffLog  time.Time
}

// NewEventsManager creates a new events manager instance
func NewEventsManager() *EventsManager {
	eventsConfigDir := os.Getenv("EVENTS_CONFIG_DIR")
	if eventsConfigDir == "" {
		eventsConfigDir = defaultEventsConfigDir
	}

	inputDir := os.Getenv("INPUT_DIR")
	if inputDir == "" {
		inputDir = defaultInputDir
	}

	now := time.Now()
	managerID := uuid.New().String()[:8]

	return &EventsManager{
		managerID:       managerID,
		eventsConfigDir: eventsConfigDir,
		inputDir:        inputDir,
		configs:         make(map[string]*EventConfig),
		states:          make(map[string]*EventState),
		lastActivity:    now,
		backoffIndex:    0,
		nextBackoffLog:  now.Add(backoffLevels[0]),
	}
}

// ensureDirectories creates necessary directories
func (m *EventsManager) ensureDirectories() error {
	dirs := []string{
		m.eventsConfigDir,
		filepath.Join(m.inputDir, "any"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// createDefaultConfigs creates the default event configurations if none exist
func (m *EventsManager) createDefaultConfigs() error {
	entries, err := os.ReadDir(m.eventsConfigDir)
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

		// Create default daily report config
		dailyConfig := EventConfig{
			Name:        "default-daily-report",
			Type:        EventTypeSchedule,
			Description: "Creates a daily report for yesterday's activities",
			ScheduleAt:  "09:00",
			ReportType:  "daily",
			Enabled:     true,
		}

		dailyData, err := json.MarshalIndent(dailyConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal daily config: %w", err)
		}

		dailyPath := filepath.Join(m.eventsConfigDir, "default-daily-report.json")
		if err := os.WriteFile(dailyPath, dailyData, 0644); err != nil {
			return fmt.Errorf("failed to write daily config: %w", err)
		}
		log.Printf("[%s] Created default daily report config: %s", m.managerID, dailyPath)

		// Create custom heartbeat report config (timer-based, every 6 hours)
		heartbeatConfig := EventConfig{
			Name:        "custom-heartbeat-report",
			Type:        EventTypeTimer,
			Description: "Custom heartbeat report - announces itself on startup and every 6 hours",
			Interval:    "6h",
			ReportType:  "custom",
			TopicStyle:  "random_words",
			Enabled:     true,
		}

		heartbeatData, err := json.MarshalIndent(heartbeatConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal heartbeat config: %w", err)
		}

		heartbeatPath := filepath.Join(m.eventsConfigDir, "custom-heartbeat-report.json")
		if err := os.WriteFile(heartbeatPath, heartbeatData, 0644); err != nil {
			return fmt.Errorf("failed to write heartbeat config: %w", err)
		}
		log.Printf("[%s] Created custom heartbeat report config: %s", m.managerID, heartbeatPath)
	}

	return nil
}

// loadConfigs reads all event configurations from the config directory
func (m *EventsManager) loadConfigs() error {
	entries, err := os.ReadDir(m.eventsConfigDir)
	if err != nil {
		return fmt.Errorf("failed to read config directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		configPath := filepath.Join(m.eventsConfigDir, entry.Name())
		data, err := os.ReadFile(configPath)
		if err != nil {
			log.Printf("[%s] Warning: failed to read config %s: %v", m.managerID, configPath, err)
			continue
		}

		var config EventConfig
		if err := json.Unmarshal(data, &config); err != nil {
			log.Printf("[%s] Warning: failed to parse config %s: %v", m.managerID, configPath, err)
			continue
		}

		if !config.Enabled {
			log.Printf("[%s] Config %s is disabled, skipping", m.managerID, config.Name)
			continue
		}

		m.configs[config.Name] = &config

		// Initialize state if not exists
		if _, exists := m.states[config.Name]; !exists {
			m.states[config.Name] = &EventState{}
		}

		log.Printf("[%s] Loaded config: %s (type: %s)", m.managerID, config.Name, config.Type)
	}

	return nil
}

// generateRandomTopic creates an "adjective noun" topic using random word chooser
func generateRandomTopic() string {
	adj := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]
	return fmt.Sprintf("%s %s", adj, noun)
}

// yesterdayDate returns yesterday's date in YYYY-MM-DD format
func yesterdayDate() string {
	return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
}

// todayDate returns today's date in YYYY-MM-DD format
func todayDate() string {
	return time.Now().Format("2006-01-02")
}

// generateWorkUnitName creates a unique work unit folder name
func generateWorkUnitName(prefix string) string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	shortID := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%s_%s", prefix, timestamp, shortID)
}

// checkPendingWorkUnit checks if there's already a pending work unit for the given type and date
func (m *EventsManager) checkPendingWorkUnit(reportType string, date string) (bool, error) {
	anyDir := filepath.Join(m.inputDir, "any")
	entries, err := os.ReadDir(anyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		reportJSON := filepath.Join(anyDir, entry.Name(), "REPORT.json")
		data, err := os.ReadFile(reportJSON)
		if err != nil {
			continue // Not a report work unit or can't read
		}

		var report Report
		if err := json.Unmarshal(data, &report); err != nil {
			continue
		}

		if report.Type == reportType && report.Date == date {
			return true, nil // Found a pending work unit for this report type and date
		}
	}

	return false, nil
}

// createWorkUnit creates a new work unit in the input directory
func (m *EventsManager) createWorkUnit(name string, report *Report) error {
	workUnitPath := filepath.Join(m.inputDir, "any", name)

	if err := os.MkdirAll(workUnitPath, 0755); err != nil {
		return fmt.Errorf("failed to create work unit directory: %w", err)
	}

	reportData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	reportPath := filepath.Join(workUnitPath, "REPORT.json")
	if err := os.WriteFile(reportPath, reportData, 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

// handleTimerEvent processes a timer-type event
func (m *EventsManager) handleTimerEvent(config *EventConfig, state *EventState) error {
	now := time.Now()

	// Parse the interval
	interval, err := time.ParseDuration(config.Interval)
	if err != nil {
		return fmt.Errorf("invalid interval %s: %w", config.Interval, err)
	}

	// Check if this is first run (NextRun is zero) or if it's time to run
	shouldRun := state.NextRun.IsZero() || now.After(state.NextRun)

	if !shouldRun {
		return nil
	}

	log.Printf("[%s] Timer event triggered: %s", m.managerID, config.Name)

	// Generate the report
	var report Report
	report.Type = config.ReportType
	report.Timestamp = now.Format(time.RFC3339)

	if config.TopicStyle == "random_words" {
		topic := generateRandomTopic()
		report.Content = fmt.Sprintf("# Custom Heartbeat Report\n\nThis is a custom heartbeat report about the '%s'.\n\nPlease generate an interesting and creative report on this topic, exploring its significance, applications, or philosophical implications.", topic)
		log.Printf("[%s] Generated topic: %s", m.managerID, topic)
	}

	// Create work unit
	workUnitName := generateWorkUnitName(config.Name)
	if err := m.createWorkUnit(workUnitName, &report); err != nil {
		return fmt.Errorf("failed to create work unit: %w", err)
	}

	// Update state
	state.LastRun = now
	state.NextRun = now.Add(interval)
	state.RunCount++
	state.LastResult = "success"

	log.Printf("[%s] Created work unit: %s (next run: %s)", m.managerID, workUnitName, state.NextRun.Format(time.RFC3339))

	return nil
}

// handleScheduleEvent processes a schedule-type event
func (m *EventsManager) handleScheduleEvent(config *EventConfig, state *EventState) error {
	now := time.Now()
	yesterday := yesterdayDate()

	// For daily reports, check if we've already handled yesterday
	if config.ReportType == "daily" {
		// Check if we already ran for yesterday
		if state.LastResult == yesterday {
			return nil // Already processed yesterday's report
		}

		// Check if there's already a pending work unit for yesterday
		pending, err := m.checkPendingWorkUnit("daily", yesterday)
		if err != nil {
			log.Printf("[%s] Warning: failed to check pending work units: %v", m.managerID, err)
		}
		if pending {
			log.Printf("[%s] Work unit already pending for daily report on %s", m.managerID, yesterday)
			state.LastResult = yesterday
			return nil
		}

		// Parse schedule time
		scheduleTime, err := time.Parse("15:04", config.ScheduleAt)
		if err != nil {
			return fmt.Errorf("invalid schedule_at %s: %w", config.ScheduleAt, err)
		}

		// Check if current time is past the scheduled time
		currentTime := time.Date(2000, 1, 1, now.Hour(), now.Minute(), 0, 0, time.UTC)
		scheduledTime := time.Date(2000, 1, 1, scheduleTime.Hour(), scheduleTime.Minute(), 0, 0, time.UTC)

		if currentTime.Before(scheduledTime) {
			return nil // Not yet time to run
		}

		log.Printf("[%s] Schedule event triggered: %s (for date: %s)", m.managerID, config.Name, yesterday)

		// Create the daily report work unit
		report := Report{
			Type:      "daily",
			Date:      yesterday,
			Timestamp: now.Format(time.RFC3339),
		}

		workUnitName := generateWorkUnitName(fmt.Sprintf("daily-report-%s", yesterday))
		if err := m.createWorkUnit(workUnitName, &report); err != nil {
			return fmt.Errorf("failed to create work unit: %w", err)
		}

		// Update state
		state.LastRun = now
		state.RunCount++
		state.LastResult = yesterday

		log.Printf("[%s] Created daily report work unit: %s", m.managerID, workUnitName)
	}

	return nil
}

// processEvents checks and processes all configured events
func (m *EventsManager) processEvents() (int, error) {
	processed := 0

	for name, config := range m.configs {
		state := m.states[name]

		var err error
		switch config.Type {
		case EventTypeTimer:
			err = m.handleTimerEvent(config, state)
		case EventTypeSchedule:
			err = m.handleScheduleEvent(config, state)
		default:
			log.Printf("[%s] Unknown event type: %s", m.managerID, config.Type)
			continue
		}

		if err != nil {
			log.Printf("[%s] Error processing event %s: %v", m.managerID, name, err)
		} else {
			processed++
		}
	}

	return processed, nil
}

// run is the main processing loop
func (m *EventsManager) run() {
	log.Printf("[%s] Agent events manager started", m.managerID)
	log.Printf("[%s] Events config: %s", m.managerID, m.eventsConfigDir)
	log.Printf("[%s] Input directory: %s", m.managerID, filepath.Join(m.inputDir, "any"))

	// Load and display configs
	for name, config := range m.configs {
		log.Printf("[%s] Event '%s': type=%s, enabled=%v", m.managerID, name, config.Type, config.Enabled)
		if config.Type == EventTypeTimer {
			log.Printf("[%s]   - interval: %s", m.managerID, config.Interval)
		} else if config.Type == EventTypeSchedule {
			log.Printf("[%s]   - schedule_at: %s", m.managerID, config.ScheduleAt)
		}
	}

	for {
		processed, err := m.processEvents()
		if err != nil {
			log.Printf("[%s] Error processing events: %v", m.managerID, err)
		}

		if processed > 0 {
			// Reset backoff on activity
			m.lastActivity = time.Now()
			m.backoffIndex = 0
			m.nextBackoffLog = m.lastActivity.Add(backoffLevels[0])
		} else {
			// No activity - check if we should log with backoff
			now := time.Now()
			if now.After(m.nextBackoffLog) {
				timeSinceActivity := now.Sub(m.lastActivity)
				log.Printf("[%s] No new activity detected for %s", m.managerID, timeSinceActivity.Round(time.Second))

				// Advance to next backoff level if not at max
				if m.backoffIndex < len(backoffLevels)-1 {
					m.backoffIndex++
				}
				m.nextBackoffLog = now.Add(backoffLevels[m.backoffIndex])
			}
		}

		time.Sleep(checkInterval)
	}
}

func main() {
	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	manager := NewEventsManager()

	if err := manager.ensureDirectories(); err != nil {
		log.Fatalf("Failed to ensure directories: %v", err)
	}

	if err := manager.createDefaultConfigs(); err != nil {
		log.Fatalf("Failed to create default configs: %v", err)
	}

	if err := manager.loadConfigs(); err != nil {
		log.Fatalf("Failed to load configs: %v", err)
	}

	manager.run()
}
