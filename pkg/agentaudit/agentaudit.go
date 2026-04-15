// Package agentaudit captures full snapshots of an agent's universe at invocation time.
//
// Auditing is controlled by the AGENT_AUDIT environment variable:
//   - unset / empty: no audit is written
//   - "FULL": a full audit (prompt + filesystem tree) is written to /agent-audit/<type>/<id>/<timestamp>/
package agentaudit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const auditRoot = "/agent-audit"

// Input holds the data needed for a full audit snapshot.
type Input struct {
	// AgentType identifies the component capturing the audit (e.g. "heuristic-request", "agent-worker").
	AgentType string
	// ID identifies the watcher or worker instance.
	ID string
	// Agent is the name of the agent being invoked (e.g. "claude").
	Agent string
	// Prompt is the complete prompt text fed to the agent.
	Prompt string
	// FSPaths are filesystem paths to snapshot (tree listing only, no file contents).
	FSPaths []string
}

// IsEnabled reports whether AGENT_AUDIT=FULL is set.
func IsEnabled() bool {
	return os.Getenv("AGENT_AUDIT") == "FULL"
}

// Capture writes a full audit snapshot if AGENT_AUDIT=FULL.
// It is non-fatal: callers should log the returned error and continue.
func Capture(input Input) error {
	if !IsEnabled() {
		return nil
	}

	ts := time.Now().UTC().Format("2006-01-02_15-04-05.000")
	auditDir := filepath.Join(auditRoot, input.AgentType, input.ID, ts)
	if err := os.MkdirAll(auditDir, 0755); err != nil {
		return fmt.Errorf("failed to create audit dir %s: %w", auditDir, err)
	}

	// prompt.txt — the complete prompt fed to the agent
	if err := os.WriteFile(filepath.Join(auditDir, "prompt.txt"), []byte(input.Prompt), 0644); err != nil {
		return fmt.Errorf("failed to write prompt.txt: %w", err)
	}

	// filesystem.txt — tree listing of all visible paths
	var sb strings.Builder
	for _, fsPath := range input.FSPaths {
		sb.WriteString("=== " + fsPath + " ===\n")
		info, statErr := os.Stat(fsPath)
		if statErr != nil {
			sb.WriteString("(path not readable: " + statErr.Error() + ")\n")
		} else if !info.IsDir() {
			sb.WriteString(info.Name() + " (file)\n")
		} else {
			if walkErr := walkEntries(fsPath, "", &sb); walkErr != nil {
				sb.WriteString("(error walking tree: " + walkErr.Error() + ")\n")
			}
		}
		sb.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(auditDir, "filesystem.txt"), []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write filesystem.txt: %w", err)
	}

	// audit.json — metadata
	meta := map[string]interface{}{
		"agent_type": input.AgentType,
		"id":         input.ID,
		"agent":      input.Agent,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"fs_paths":   input.FSPaths,
		"audit_dir":  auditDir,
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(auditDir, "audit.json"), metaData, 0644); err != nil {
		return fmt.Errorf("failed to write audit.json: %w", err)
	}

	return nil
}

// walkEntries recursively appends a tree-style listing of dir to sb.
func walkEntries(dir, prefix string, sb *strings.Builder) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for i, entry := range entries {
		isLast := i == len(entries)-1
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		sb.WriteString(prefix + connector + name + "\n")
		if entry.IsDir() {
			// Best-effort: skip subtrees we cannot read
			_ = walkEntries(filepath.Join(dir, entry.Name()), childPrefix, sb)
		}
	}
	return nil
}
