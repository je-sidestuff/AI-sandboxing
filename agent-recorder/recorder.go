// Package recorder provides a common record-emitting interface for agents.
package recorder

import (
	"fmt"
	"time"
)

// AgentType identifies whether the record came from a deterministic or heuristic agent.
type AgentType string

const (
	Deterministic AgentType = "deterministic"
	Heuristic     AgentType = "heuristic"
)

// Record holds a single agent event record.
type Record struct {
	AgentID   string
	AgentType AgentType
	Timestamp time.Time
	Event     string
	Payload   map[string]string
}

// New creates a new Record with the current timestamp.
func New(agentID string, agentType AgentType, event string, payload map[string]string) Record {
	return Record{
		AgentID:   agentID,
		AgentType: agentType,
		Timestamp: time.Now().UTC(),
		Event:     event,
		Payload:   payload,
	}
}

// Emit prints the record to stdout in a structured format.
func Emit(r Record) {
	fmt.Printf("[%s] agent=%s type=%s event=%s",
		r.Timestamp.Format(time.RFC3339),
		r.AgentID,
		r.AgentType,
		r.Event,
	)
	for k, v := range r.Payload {
		fmt.Printf(" %s=%s", k, v)
	}
	fmt.Println()
}
