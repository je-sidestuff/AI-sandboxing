# Schema Proposals for Better Flow Correlation

This document outlines proposed schema changes to improve flow correlation across the AI sandboxing system. The goal is to enable seamless tracking of work from any entry point (heuristic request, event, instruction, report) through to completion.

## Current Challenges

### 1. Missing Correlation IDs

Currently, work units in different stages often share the same directory name (e.g., `dispatch-repo-isolation_2026-03-25_23-50-11_3b003a59-1774482618`), but there's no explicit `flow_id` or `correlation_id` field linking records across stages.

**Proposed Solution:**

Add a standardized `flow_id` field to all data structures:

```go
// Add to Instruction, Report, Dispatch, and FlowRecord
type CommonFlowFields struct {
    FlowID      string `json:"flow_id"`           // UUID for end-to-end correlation
    ParentFlow  string `json:"parent_flow,omitempty"` // For nested/child flows
    OriginType  string `json:"origin_type"`       // "heuristic", "event", "instruction", "report", "dispatch"
    OriginID    string `json:"origin_id"`         // ID of original triggering entity
}
```

### 2. No Unified Event Timeline

Each component (heuristic-request, agent-events, agent-dispatch, agent-worker) maintains its own records, but there's no unified event log showing the complete lifecycle of a flow.

**Proposed Solution:**

Create a centralized event log structure:

```go
type FlowEvent struct {
    EventID     string            `json:"event_id"`
    FlowID      string            `json:"flow_id"`
    Timestamp   time.Time         `json:"timestamp"`
    Stage       string            `json:"stage"`        // "heuristic", "dispatch", "execution", "monitoring", "complete"
    Action      string            `json:"action"`       // "created", "started", "completed", "failed", "transitioned"
    Source      string            `json:"source"`       // Component that generated event
    PreviousID  string            `json:"previous_id,omitempty"` // Link to previous event
    Metadata    map[string]string `json:"metadata,omitempty"`
}
```

Write events to a shared location: `/workspaces/slopspaces/flow-events/` or append to a unified log file.

### 3. Heuristic to Dispatch Linkage

When heuristic-request processes a HEURISTIC.md and generates DISPATCH.json/INSTRUCTION.json, the output files don't reference back to the original heuristic.

**Proposed Solution:**

Add origin tracking to generated files:

```json
// In DISPATCH.json or INSTRUCTION.json generated from heuristic
{
  "type": "repo-isolation",
  "instruction": "...",
  "origin": {
    "type": "heuristic",
    "path": "/workspaces/slopspaces/heuristic/processed/heuristic_xyz_123",
    "heuristic_id": "heuristic_xyz_123",
    "processed_at": "2026-03-25T23:50:11Z"
  }
}
```

### 4. Event to Report to Instruction Chain

When agent-events creates a report, and that report becomes an instruction, the chain is implicit rather than explicit.

**Proposed Solution:**

Extend EventConfig and Report structures:

```go
type EventConfig struct {
    // ... existing fields ...

    // New fields for correlation
    FlowIDPrefix string `json:"flow_id_prefix,omitempty"` // Optional prefix for generated flow IDs
}

type Report struct {
    // ... existing fields ...

    // New correlation fields
    EventConfigName string `json:"event_config_name,omitempty"` // Which event triggered this
    EventTriggerID  string `json:"event_trigger_id,omitempty"`  // Unique ID for this trigger instance
}
```

### 5. Missing Stage Timestamps

FlowRecord tracks `StartTime` and `EndTime`, but doesn't track when the flow transitioned between stages.

**Proposed Solution:**

Add stage transition tracking:

```go
type FlowRecord struct {
    // ... existing fields ...

    // Stage transition timestamps
    StageTransitions []StageTransition `json:"stage_transitions,omitempty"`
}

type StageTransition struct {
    Stage       string    `json:"stage"`
    EnteredAt   time.Time `json:"entered_at"`
    ExitedAt    time.Time `json:"exited_at,omitempty"`
    Status      string    `json:"status"`  // "completed", "failed", "skipped"
    DurationMs  int64     `json:"duration_ms,omitempty"`
}
```

### 6. Unified Flow State File

Consider creating a single `FLOW_STATE.json` file that travels with the work unit:

```json
{
  "flow_id": "flow_abc123",
  "created_at": "2026-03-25T20:00:00Z",
  "origin": {
    "type": "heuristic",
    "id": "heuristic_xyz",
    "path": "/workspaces/slopspaces/heuristic/processed/..."
  },
  "stages": [
    {
      "name": "heuristic",
      "status": "completed",
      "path": "/workspaces/slopspaces/heuristic/processed/...",
      "entered_at": "2026-03-25T20:00:00Z",
      "exited_at": "2026-03-25T20:01:30Z"
    },
    {
      "name": "dispatch",
      "status": "completed",
      "path": "/workspaces/slopspaces/requests/...",
      "entered_at": "2026-03-25T20:01:30Z",
      "exited_at": "2026-03-25T20:02:00Z"
    },
    {
      "name": "execution",
      "status": "active",
      "path": "/workspaces/slopspaces/dispatcher/live/flows/...",
      "entered_at": "2026-03-25T20:02:00Z"
    }
  ],
  "current_stage": "execution",
  "current_status": "active",
  "metadata": {
    "type": "repo-isolation",
    "target_repo": "owner/repo",
    "pr_url": "https://github.com/..."
  }
}
```

## Implementation Priority

### High Priority

1. **Add `flow_id` to all data structures** - This is the foundation for all other improvements.

2. **Create FLOW_STATE.json** - This portable state file enables:
   - Easy correlation across directories
   - Self-documenting flow history
   - Simple querying and display in agent-viewer

3. **Update heuristic-request output** - Add origin tracking to generated DISPATCH/INSTRUCTION files.

### Medium Priority

4. **Centralized event log** - Create `/workspaces/slopspaces/flow-events/` with per-flow event files.

5. **Stage transition tracking** - Extend FlowRecord with detailed transition history.

### Lower Priority

6. **Event chain correlation** - Link event configs to their triggered reports and subsequent flows.

7. **Parent-child flow relationships** - For complex workflows that spawn sub-flows.

## Schema Migration Path

To maintain backward compatibility:

1. All new fields should be optional (`omitempty`)
2. Flow correlation should work even without explicit `flow_id` by falling back to directory name matching
3. Agent-viewer should handle both old and new formats gracefully
4. Consider adding a `schema_version` field to enable versioned parsing

## Agent-Viewer Integration Notes

The current agent-viewer implementation correlates flows by matching directory names across different scan targets. With the proposed schema changes:

1. Primary correlation becomes `flow_id` lookup
2. Directory name matching serves as fallback
3. FLOW_STATE.json provides pre-computed stage information
4. Event log enables real-time timeline display

## Example: Complete Flow Lifecycle

```
1. User creates: /heuristic/pending/task_20260325_001/HEURISTIC.md

2. heuristic-request processes and outputs:
   /heuristic/processed/task_20260325_001/
     - HEURISTIC.md (original)
     - DISPATCH.json (with origin.type="heuristic")
     - FLOW_STATE.json (flow_id="flow_task_001", stage="heuristic", status="completed")

   /requests/task_20260325_001/
     - DISPATCH.json (copied)
     - FLOW_STATE.json (updated: stage="dispatch", status="pending")

3. agent-dispatch picks up and processes:
   /dispatcher/live/flows/task_20260325_001/
     - flow_task_001.json (terraform flow record)
     - FLOW_STATE.json (updated: stage="execution", status="active")

4. Upon completion:
   /output/task_20260325_001/
     - result files
     - FLOW_STATE.json (updated: stage="complete", status="completed")
```

This structure enables agent-viewer to:
- Show complete flow history from any entry point
- Display accurate stage progression
- Link related PRs and repos
- Track timing and duration at each stage
