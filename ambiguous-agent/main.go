package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/chzyer/readline"
	"github.com/google/uuid"
)

const defaultRecordsPath = "/workspaces/agent-records/"
const defaultAgent = "claude"

// Available agent presets (must match invoke-agent.sh presets)
var availableAgents = []string{"copilot", "gemini", "claude", "opencode", "codex", "grok"}

// Agent colors for visual distinction
var agentColors = map[string]lipgloss.Color{
	"copilot":  lipgloss.Color("39"),  // Cyan (GitHub blue)
	"gemini":   lipgloss.Color("33"),  // Blue (Google blue)
	"claude":   lipgloss.Color("208"), // Orange (Anthropic)
	"opencode": lipgloss.Color("34"),  // Green
	"codex":    lipgloss.Color("99"),  // Purple (OpenAI)
	"grok":     lipgloss.Color("196"), // Red (xAI)
}

var (
	// Styles - subtle decoration
	sessionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	exitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("34"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	agentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141")).
			Bold(true)

	continuationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("243"))
)

// ShellCompleter implements readline.AutoCompleter for the ambiguous shell.
// It provides extensible tab-completion, currently supporting filepath completion.
type ShellCompleter struct {
	cwd *string // Pointer to track current working directory changes
}

// Do implements readline.AutoCompleter. It returns completion candidates for
// the current line at the given position.
func (c *ShellCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	lineStr := string(line[:pos])

	// Extract the word being completed (last space-separated token)
	lastSpace := strings.LastIndex(lineStr, " ")
	var prefix string
	if lastSpace == -1 {
		prefix = lineStr
	} else {
		prefix = lineStr[lastSpace+1:]
	}

	// For now, use filepath completion for everything
	// This can be extended later to add command-specific completers
	candidates := completeFilepath(prefix, *c.cwd)

	// Convert candidates to readline format
	// length is how many characters to replace from the cursor position
	length = len(prefix)
	for _, cand := range candidates {
		// Return only the suffix that completes the prefix
		suffix := []rune(cand[len(prefix):])
		newLine = append(newLine, suffix)
	}

	return newLine, length
}

// completeFilepath returns filepath completion candidates for the given prefix.
// It handles absolute paths, relative paths, tilde expansion, and directory traversal.
func completeFilepath(prefix string, cwd string) []string {
	if prefix == "" {
		// Complete from current directory
		return listDir(cwd, "", true)
	}

	home := os.Getenv("HOME")

	// Expand tilde
	expandedPrefix := prefix
	tildeExpanded := false
	if strings.HasPrefix(prefix, "~/") {
		expandedPrefix = filepath.Join(home, prefix[2:])
		tildeExpanded = true
	} else if prefix == "~" {
		expandedPrefix = home
		tildeExpanded = true
	}

	// Determine the directory to search and the partial filename
	var searchDir, partial string
	if filepath.IsAbs(expandedPrefix) {
		searchDir = filepath.Dir(expandedPrefix)
		partial = filepath.Base(expandedPrefix)
	} else {
		// Relative path
		searchDir = filepath.Join(cwd, filepath.Dir(expandedPrefix))
		partial = filepath.Base(expandedPrefix)
	}

	// Handle case where prefix ends with separator (completing inside a directory)
	if strings.HasSuffix(expandedPrefix, string(filepath.Separator)) || expandedPrefix == home {
		searchDir = expandedPrefix
		partial = ""
	}

	// Get candidates from the directory
	candidates := listDir(searchDir, partial, false)

	// Convert back to original format (with tilde if needed)
	var result []string
	for _, cand := range candidates {
		fullPath := filepath.Join(searchDir, cand)

		// Check if it's a directory and append separator
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			cand += string(filepath.Separator)
		}

		// Build the completion string
		var completion string
		if tildeExpanded {
			// Convert back to tilde notation
			if strings.HasPrefix(fullPath, home) {
				completion = "~" + fullPath[len(home):]
			} else {
				completion = fullPath
			}
		} else if filepath.IsAbs(prefix) || strings.Contains(prefix, string(filepath.Separator)) {
			// Preserve the original path structure
			dir := filepath.Dir(prefix)
			if strings.HasSuffix(prefix, string(filepath.Separator)) {
				completion = prefix + cand
			} else {
				completion = filepath.Join(dir, cand)
			}
			// Re-add trailing separator for directories
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() && !strings.HasSuffix(completion, string(filepath.Separator)) {
				completion += string(filepath.Separator)
			}
		} else {
			completion = cand
			// Add trailing separator for directories
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() && !strings.HasSuffix(completion, string(filepath.Separator)) {
				completion += string(filepath.Separator)
			}
		}

		result = append(result, completion)
	}

	return result
}

// listDir returns entries in dir that start with prefix.
// If showHidden is false, entries starting with '.' are excluded unless prefix starts with '.'.
func listDir(dir string, prefix string, showHidden bool) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	showDotFiles := showHidden || strings.HasPrefix(prefix, ".")

	var result []string
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless explicitly requested
		if !showDotFiles && strings.HasPrefix(name, ".") {
			continue
		}

		// Filter by prefix
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}

		result = append(result, name)
	}

	sort.Strings(result)
	return result
}

// CommandRecord holds metadata for each command
type CommandRecord struct {
	ID        string `json:"id"`
	Command   string `json:"cmd"`
	Timestamp string `json:"ts"`
	DeltaMs   int64  `json:"delta_ms"`
	ExitCode  int    `json:"exit"`
}

func main() {
	recordsPath := os.Getenv("AGENT_RECORDS_PATH")
	if recordsPath == "" {
		recordsPath = defaultRecordsPath
	}

	// Initialize current agent from environment or use default
	currentAgent := os.Getenv("AGENT_PRESET")
	if currentAgent == "" {
		currentAgent = defaultAgent
	}

	now := time.Now()
	sessionID := fmt.Sprintf("%s_%d", now.Format("2006-01-02_15-04-05"), now.Unix())
	sessionDir := filepath.Join(recordsPath, "session-"+sessionID)

	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating session directory: %v\n", err)
		os.Exit(1)
	}

	os.Setenv("RECORDS_PATH", sessionDir)

	logPath := filepath.Join(sessionDir, "session.jsonl")
	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating session log: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	fmt.Println(sessionStyle.Render(fmt.Sprintf("● session: %s", sessionDir)))
	fmt.Println(sessionStyle.Render(fmt.Sprintf("  agent: %s | 'set-agent <name>' to change | 'list-agents' for options", currentAgent)))
	fmt.Println(sessionStyle.Render("  type 'exit!' to end | 'agent <prompt>' to invoke AI"))
	fmt.Println(sessionStyle.Render("  multi-line: trailing \\, unclosed quotes, or <<<DELIMITER"))
	fmt.Println()

	readlineRecords := filepath.Join(os.Getenv("HOME"), ".ambiguous_records")
	cwd, _ := os.Getwd()
	oldCwd := cwd // For cd - support

	// Create completer with pointer to cwd so it tracks directory changes
	completer := &ShellCompleter{cwd: &cwd}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          buildPrompt(cwd, currentAgent),
		HistoryFile:     readlineRecords,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		AutoComplete:    completer,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error initializing readline: %v\n", err)
		os.Exit(1)
	}
	defer rl.Close()

	var lastCommandTime time.Time
	encoder := json.NewEncoder(logFile)

	for {
		initialLine, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if len(initialLine) == 0 {
				break
			}
			continue
		}
		if err == io.EOF {
			break
		}

		initialLine = strings.TrimSpace(initialLine)
		if initialLine == "" {
			continue
		}

		// Handle multi-line input (backslash, heredoc, unclosed quotes)
		mainPrompt := buildPrompt(cwd, currentAgent)
		line, err := readMultiLine(rl, initialLine, mainPrompt)
		if err == readline.ErrInterrupt {
			continue // User cancelled multi-line input
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, errorStyle.Render(fmt.Sprintf("input error: %v", err)))
			continue
		}

		if line == "exit!" {
			fmt.Println(exitStyle.Render("session ended."))
			break
		}

		// Calculate delta from last command
		var deltaMs int64
		commandTime := time.Now()
		if !lastCommandTime.IsZero() {
			deltaMs = commandTime.Sub(lastCommandTime).Milliseconds()
		}
		lastCommandTime = commandTime

		// Handle special commands
		var exitCode int

		// Handle cd command specially (persists directory change)
		if line == "cd" || strings.HasPrefix(line, "cd ") {
			target := strings.TrimSpace(strings.TrimPrefix(line, "cd"))
			newDir, err := handleCd(target, cwd, oldCwd)
			if err != nil {
				fmt.Fprintln(os.Stderr, errorStyle.Render(fmt.Sprintf("cd: %v", err)))
			} else {
				oldCwd = cwd
				cwd = newDir
				rl.SetPrompt(buildPrompt(cwd, currentAgent))
			}
			continue
		}

		// Handle export command specially (persists environment variable)
		if line == "export" || strings.HasPrefix(line, "export ") {
			arg := strings.TrimSpace(strings.TrimPrefix(line, "export"))
			if err := handleExport(arg); err != nil {
				fmt.Fprintln(os.Stderr, errorStyle.Render(fmt.Sprintf("export: %v", err)))
			}
			continue
		}

		if strings.HasPrefix(line, "set-agent ") {
			newAgent := strings.TrimSpace(strings.TrimPrefix(line, "set-agent "))
			if isValidAgent(newAgent) {
				currentAgent = newAgent
				rl.SetPrompt(buildPrompt(cwd, currentAgent))
				fmt.Println(successStyle.Render(fmt.Sprintf("agent set to: %s", currentAgent)))
			} else {
				fmt.Println(errorStyle.Render(fmt.Sprintf("unknown agent: %s", newAgent)))
				fmt.Println(sessionStyle.Render(fmt.Sprintf("available: %s", strings.Join(availableAgents, ", "))))
			}
			continue
		} else if line == "set-agent" {
			fmt.Println(errorStyle.Render("usage: set-agent <name>"))
			fmt.Println(sessionStyle.Render(fmt.Sprintf("available: %s", strings.Join(availableAgents, ", "))))
			continue
		} else if line == "list-agents" {
			fmt.Println(sessionStyle.Render("available agents:"))
			for _, a := range availableAgents {
				color := agentColors[a]
				if color == "" {
					color = lipgloss.Color("141") // Fallback purple
				}
				agentNameStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
				if a == currentAgent {
					fmt.Println(agentNameStyle.Render(fmt.Sprintf("  → %s (selected)", a)))
				} else {
					fmt.Println(agentNameStyle.Render(fmt.Sprintf("    %s", a)))
				}
			}
			continue
		} else if strings.HasPrefix(line, "agent ") {
			prompt := strings.TrimPrefix(line, "agent ")
			exitCode = runAgent(prompt, currentAgent, sessionDir, logFile)
		} else if line == "agent" {
			fmt.Println(errorStyle.Render("usage: agent <prompt>"))
			continue
		} else {
			exitCode = runCommand(line, logFile)
		}

		// Write metadata record
		record := CommandRecord{
			ID:        uuid.New().String()[:8],
			Command:   line,
			Timestamp: commandTime.Format(time.RFC3339),
			DeltaMs:   deltaMs,
			ExitCode:  exitCode,
		}
		encoder.Encode(record)

		// Update prompt (cwd might have changed via cd)
		newCwd, _ := os.Getwd()
		if newCwd != cwd {
			cwd = newCwd
			rl.SetPrompt(buildPrompt(cwd, currentAgent))
		}
	}
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

// handleCd processes a cd command and returns the new directory or an error.
// Handles: cd (home), cd - (previous), cd ~ (home), cd ~/path, and regular paths.
func handleCd(target string, cwd string, oldCwd string) (string, error) {
	home := os.Getenv("HOME")

	// Determine target directory
	var targetDir string
	switch {
	case target == "" || target == "~":
		// cd or cd ~ -> home directory
		targetDir = home
	case target == "-":
		// cd - -> previous directory
		if oldCwd == cwd {
			return "", fmt.Errorf("OLDPWD not set")
		}
		targetDir = oldCwd
		fmt.Println(targetDir) // bash prints the directory when using cd -
	case strings.HasPrefix(target, "~/"):
		// cd ~/path -> expand tilde
		targetDir = filepath.Join(home, target[2:])
	default:
		// Handle quoted strings by stripping outer quotes
		if (strings.HasPrefix(target, "\"") && strings.HasSuffix(target, "\"")) ||
			(strings.HasPrefix(target, "'") && strings.HasSuffix(target, "'")) {
			target = target[1 : len(target)-1]
		}
		targetDir = target
	}

	// Make absolute if relative
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(cwd, targetDir)
	}

	// Clean the path
	targetDir = filepath.Clean(targetDir)

	// Attempt to change directory
	if err := os.Chdir(targetDir); err != nil {
		return "", err
	}

	// Get the actual path (resolves symlinks, normalizes)
	newCwd, err := os.Getwd()
	if err != nil {
		return targetDir, nil // fallback to what we computed
	}
	return newCwd, nil
}

// handleExport processes an export command to set environment variables.
// Supports: export VAR=value, export VAR="value", export VAR (promotes existing var).
func handleExport(arg string) error {
	if arg == "" {
		// export with no args: list all exported variables (like bash)
		for _, env := range os.Environ() {
			fmt.Printf("declare -x %s\n", env)
		}
		return nil
	}

	// Handle multiple exports: export VAR1=val1 VAR2=val2
	assignments := parseExportArgs(arg)
	for _, assignment := range assignments {
		if err := processExportAssignment(assignment); err != nil {
			return err
		}
	}
	return nil
}

// parseExportArgs splits export arguments respecting quotes.
// Returns individual VAR=value or VAR assignments.
func parseExportArgs(input string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range input {
		switch {
		case !inQuote && (r == '"' || r == '\''):
			inQuote = true
			quoteChar = r
			current.WriteRune(r)
		case inQuote && r == quoteChar:
			inQuote = false
			quoteChar = 0
			current.WriteRune(r)
		case !inQuote && r == ' ':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// processExportAssignment handles a single VAR=value or VAR assignment.
func processExportAssignment(assignment string) error {
	// Check if it's VAR=value or just VAR
	eqIdx := strings.Index(assignment, "=")
	if eqIdx == -1 {
		// Just "export VAR" - variable should already exist, nothing to do
		// (In a real shell this marks it for export to child processes,
		// but os.Environ() already exports all set variables)
		if os.Getenv(assignment) == "" {
			// Variable doesn't exist, but that's fine - bash allows this too
		}
		return nil
	}

	name := assignment[:eqIdx]
	value := assignment[eqIdx+1:]

	// Validate variable name
	if name == "" {
		return fmt.Errorf("invalid variable name")
	}
	if !isValidVarName(name) {
		return fmt.Errorf("'%s': not a valid identifier", name)
	}

	// Strip outer quotes from value if present
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	return os.Setenv(name, value)
}

// isValidVarName checks if a string is a valid shell variable name.
// Valid names start with a letter or underscore, followed by letters, digits, or underscores.
func isValidVarName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
				return false
			}
		} else {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return false
			}
		}
	}
	return true
}

// abbreviatePath shortens a path for display in the prompt.
// Shows ~ for home, and abbreviates long paths to fit within maxLen.
func abbreviatePath(path string, maxLen int) string {
	home := os.Getenv("HOME")

	// Replace home with ~
	if home != "" && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}

	// If it fits, return as-is
	if len(path) <= maxLen {
		return path
	}

	// For root or very short paths
	if path == "/" || path == "~" {
		return path
	}

	// Strategy: keep the last components that fit, prefix with ...
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) <= 2 {
		// Already minimal, just truncate
		if len(path) > maxLen {
			return "..." + path[len(path)-(maxLen-3):]
		}
		return path
	}

	// Build from the end, keeping as many components as fit
	result := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		candidate := parts[i] + string(filepath.Separator) + result
		if len(candidate)+4 > maxLen { // +4 for ".../""
			break
		}
		result = candidate
	}

	// If we didn't include all parts, add prefix
	if !strings.HasPrefix(path, result) {
		result = ".../" + result
	}

	return result
}

func buildPrompt(cwd string, agent string) string {
	// Show abbreviated path (max 30 chars for the path portion)
	dir := abbreviatePath(cwd, 30)
	// Use agent-specific color if available
	color := agentColors[agent]
	if color == "" {
		color = lipgloss.Color("141") // Fallback purple
	}
	agentPromptStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	return agentPromptStyle.Render("["+agent+"]") + " " + promptStyle.Render(dir) + " › "
}

func runCommand(cmdLine string, logFile *os.File) int {
	cmd := exec.Command("bash", "-c", cmdLine)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating stdout pipe: %v\n", err)
		return 1
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating stderr pipe: %v\n", err)
		return 1
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error starting command: %v\n", err)
		return 1
	}

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(io.MultiWriter(os.Stdout, logFile), stdoutPipe)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(io.MultiWriter(os.Stderr, logFile), stderrPipe)
		done <- struct{}{}
	}()
	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

func runAgent(prompt string, agent string, sessionDir string, logFile *os.File) int {
	// Find invoke-agent.sh relative to executable or use PATH
	invokeScript := findInvokeScript()
	if invokeScript == "" {
		fmt.Println(errorStyle.Render("invoke-agent.sh not found"))
		return 1
	}

	// Parse the prompt into arguments, respecting quoted strings
	args := parseArgs(prompt)

	// Determine mode: default to -e (execute), but allow -p (prompt-only)
	mode := "-e"
	var promptArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-p" {
			mode = "-p"
		} else if args[i] == "-e" {
			mode = "-e"
		} else {
			promptArgs = append(promptArgs, args[i])
		}
	}

	// Build command with invoke-agent.sh and parsed arguments
	// Include -s flag to indicate this is invoked from a session context
	// Include -a flag to specify the agent preset
	cmdArgs := append([]string{mode, "-s", "-a", agent}, promptArgs...)
	cmd := exec.Command(invokeScript, cmdArgs...)
	cmd.Env = append(os.Environ(), "AGENT_RECORDS_PATH="+sessionDir, "AGENT_PRESET="+agent)
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	cmd.Stderr = io.MultiWriter(os.Stderr, logFile)

	fmt.Println(sessionStyle.Render(fmt.Sprintf("invoking %s...", agent)))

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			fmt.Println(errorStyle.Render(fmt.Sprintf("agent exited: %d", code)))
			return code
		}
		fmt.Println(errorStyle.Render(fmt.Sprintf("agent error: %v", err)))
		return 1
	}

	fmt.Println(successStyle.Render("agent completed"))
	return 0
}

// parseArgs splits a string into arguments, respecting quoted strings
func parseArgs(input string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range input {
		switch {
		case !inQuote && (r == '"' || r == '\''):
			inQuote = true
			quoteChar = r
		case inQuote && r == quoteChar:
			inQuote = false
			quoteChar = 0
		case !inQuote && r == ' ':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

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

	// Check PATH
	path, err := exec.LookPath("invoke-agent.sh")
	if err == nil {
		return path
	}

	return ""
}

// readMultiLine reads a potentially multi-line input, handling:
// 1. Backslash continuation (trailing \)
// 2. Heredoc-style input (<<<DELIMITER ... DELIMITER)
// 3. Unclosed single or double quotes
// Returns the complete input and any error encountered.
func readMultiLine(rl *readline.Instance, initialLine string, mainPrompt string) (string, error) {
	line := initialLine
	continuationPrompt := continuationStyle.Render("  > ")

	// Check for heredoc syntax: <<<DELIMITER
	if strings.HasPrefix(line, "<<<") {
		delimiter := strings.TrimSpace(strings.TrimPrefix(line, "<<<"))
		if delimiter == "" {
			delimiter = "EOF" // Default delimiter
		}
		rl.SetPrompt(continuationPrompt)
		var lines []string
		for {
			nextLine, err := rl.Readline()
			if err != nil {
				rl.SetPrompt(mainPrompt)
				return "", err
			}
			if strings.TrimSpace(nextLine) == delimiter {
				break
			}
			lines = append(lines, nextLine)
		}
		rl.SetPrompt(mainPrompt)
		return strings.Join(lines, "\n"), nil
	}

	// Check for backslash continuation or unclosed quotes
	for {
		needsContinuation, quoteChar := checkContinuation(line)
		if !needsContinuation {
			break
		}

		// Set appropriate continuation prompt
		if quoteChar != 0 {
			rl.SetPrompt(continuationStyle.Render(string(quoteChar) + "> "))
		} else {
			rl.SetPrompt(continuationPrompt)
		}

		nextLine, err := rl.Readline()
		if err != nil {
			rl.SetPrompt(mainPrompt)
			return "", err
		}

		// For backslash continuation, remove the trailing backslash and join
		if quoteChar == 0 && strings.HasSuffix(strings.TrimRight(line, " \t"), "\\") {
			line = strings.TrimSuffix(strings.TrimRight(line, " \t"), "\\") + "\n" + nextLine
		} else {
			// For quote continuation, just append with newline
			line = line + "\n" + nextLine
		}
	}

	rl.SetPrompt(mainPrompt)
	return line, nil
}

// checkContinuation determines if the line needs continuation.
// Returns (needsContinuation, quoteChar) where quoteChar is the unclosed quote
// character ('"' or '\'') or 0 if continuation is due to backslash.
func checkContinuation(line string) (bool, rune) {
	// Check for trailing backslash (not escaped)
	trimmed := strings.TrimRight(line, " \t")
	if strings.HasSuffix(trimmed, "\\") && !strings.HasSuffix(trimmed, "\\\\") {
		return true, 0
	}

	// Count unescaped quotes to check for unclosed strings
	var singleQuotes, doubleQuotes int
	inSingle, inDouble := false, false
	escaped := false

	for _, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && !inSingle {
			// Backslash only escapes in double quotes or outside quotes
			escaped = true
			continue
		}
		if r == '\'' && !inDouble {
			inSingle = !inSingle
			singleQuotes++
		}
		if r == '"' && !inSingle {
			inDouble = !inDouble
			doubleQuotes++
		}
	}

	// Odd number of quotes means unclosed
	if singleQuotes%2 != 0 {
		return true, '\''
	}
	if doubleQuotes%2 != 0 {
		return true, '"'
	}

	return false, 0
}
