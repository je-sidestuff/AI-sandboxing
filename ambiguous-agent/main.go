package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/chzyer/readline"
	"github.com/google/uuid"
)

const defaultRecordsPath = "/workspaces/agent-records/"

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
)

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
	fmt.Println(sessionStyle.Render("  type 'exit!' to end | 'agent <prompt>' to invoke AI"))
	fmt.Println()

	readlineRecords := filepath.Join(os.Getenv("HOME"), ".ambiguous_records")
	cwd, _ := os.Getwd()

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          buildPrompt(cwd),
		HistoryFile:     readlineRecords,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error initializing readline: %v\n", err)
		os.Exit(1)
	}
	defer rl.Close()

	var lastCommandTime time.Time
	encoder := json.NewEncoder(logFile)

	for {
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			}
			continue
		}
		if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
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

		// Handle 'agent' command specially
		var exitCode int
		if strings.HasPrefix(line, "agent ") {
			prompt := strings.TrimPrefix(line, "agent ")
			exitCode = runAgent(prompt, sessionDir, logFile)
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
			rl.SetPrompt(buildPrompt(cwd))
		}
	}
}

func buildPrompt(cwd string) string {
	dir := filepath.Base(cwd)
	if dir == "/" {
		dir = "/"
	}
	return promptStyle.Render(dir) + " › "
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

func runAgent(prompt string, sessionDir string, logFile *os.File) int {
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
	cmdArgs := append([]string{mode, "-s"}, promptArgs...)
	cmd := exec.Command(invokeScript, cmdArgs...)
	cmd.Env = append(os.Environ(), "AGENT_RECORDS_PATH="+sessionDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	cmd.Stderr = io.MultiWriter(os.Stderr, logFile)

	fmt.Println(sessionStyle.Render("invoking agent..."))

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
