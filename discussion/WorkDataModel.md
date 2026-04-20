# Work Data Model Discussion

This document describes the data model of the AI-sandboxing application suite in its current experimental state, explores potential directions for the alpha release, and poses open-ended questions to guide decision-making.

---

## Current Experimental State

### File-Based Work Passing

The system uses a **hierarchical filesystem-based work queue** where work items flow through directories. Files serve as both data containers and state markers.

#### Entry Point File Types

| File Type | Location | Purpose |
|-----------|----------|---------|
| `INSTRUCTION.json` | `INPUT_DIR/any/<work-unit-id>/` | Simple work units for direct execution |
| `REPORT.json` | `INPUT_DIR/any/<work-unit-id>/` | Report generation requests |
| `DISPATCH.json` | `INPUT_DIR/any/<work-unit-id>/` | Complex orchestration requests |
| `HEURISTIC.json` / `HEURISTIC.md` | `HEURISTIC_DIR/pending/<folder>/` | Raw heuristic requests requiring AI interpretation |

#### Core Data Structures

**Instruction** (simplest form):
```go
type Instruction struct {
    Instruction string    // The work to perform
    Mode        string    // "prompt" or "execute"
    Agent       string    // Optional agent override
    Role        string    // Optional role (e.g., "code-implementer")
    Timestamp   string
}
```

**Report**:
```go
type Report struct {
    Type      string    // "custom", "daily", "weekly", "monthly"
    Content   string    // Report specification
    Agent     string
    Timestamp string
    Date      string
}
```

**Dispatch** (most complex - orchestration):
```go
type Dispatch struct {
    Type                  string              // "direct", "in-repo", "repo-isolation", "sequence-to-new-repo"
    Instruction           string
    Mode                  string
    Agent                 string
    TargetRepo            string              // For in-repo/repo-isolation
    PRTitle               string
    PRBody                string
    IsolationName         string
    SkipApproval          bool
    SequenceRepoName      string
    SequenceCommands      []string
    SequenceMinutesBetween int
    WriteSpaceWorkingDir  string              // Custom working directory override
    WriteSpaceInputDir    string
    WriteSpaceOutputDir   string
    Metadata              map[string]string
}
```

### State Tracking

State is tracked through filesystem markers co-located with work items:

```
INPUT_DIR/any/<work-unit-id>/
├── DISPATCH.json                    ← Initial state (pending)
├── PROCESSING.md                    ← Lock/in-progress marker
├── DISPATCHING.md                   ← Specific to dispatch watching
├── APPROVED.md                      ← For approval-gated flows
├── EXECUTING.md                     ← Execution started
├── PROCESSING-<timestamp>.md        ← Timestamped progress markers
└── PROCESSED-<timestamp>.md         ← Completion marker
```

#### State Progression

1. **Pending** - Work file exists, no markers
2. **Processing** - `PROCESSING.md` created (lock file)
3. **Executing** - `EXECUTING.md` created by agent-worker
4. **Completed** - `PROCESSED-<timestamp>.md` written to both:
   - `OUTPUT_DIR/content/<id>/` (agent output)
   - `OUTPUT_DIR/records/<id>/` (processing log)

#### Persistent Flow Records

For complex flows requiring long-term state management:

```go
type FlowRecord struct {
    DispatcherID          string
    FlowID                string
    DispatchType          string
    DispatchPath          string
    StartTime             string
    EndTime               string
    Status                string      // "pending", "running", "monitoring", "completed", "failed"
    Error                 string
    PRUrl                 string
    ConclusionState       string      // "active", "closed", "merged"
    NeedsMonitoring       bool
    LastPollTime          string
    ReintegrationURL      string
    PendingDispatch       *Dispatch
    SequenceStartTimeMillis int64
    SequenceMinutesBetween  int
    SequenceLastCompletedIdx int
    SequenceTotalSteps      int
    DirectWorkUnitID        string
}
```

Flow records are stored at: `RECORDS_DIR/dispatch-watch/flow_<flow-id>.json`

### Data Ownership by Component

| Component | Input Ownership | Output Ownership | Responsibility |
|-----------|----------------|------------------|----------------|
| **heuristic-request** | `HEURISTIC_DIR/pending/<id>/` | `HEURISTIC_DIR/processed/<id>/` + generated `DISPATCH.json` | Parse heuristic, invoke AI, extract dispatch specification |
| **agent-events** | Event configs at `EVENTS_CONFIG_DIR/` | `INPUT_DIR/any/<id>/REPORT.json` | Generate scheduled/timer-based reports |
| **agent-dispatch** | `INPUT_DIR/any/<id>/DISPATCH.json` | `OUTPUT_DIR/content/<id>/PROCESSED-*.md` | Orchestrate work via terraform, manage PRs |
| **agent-worker** | `INPUT_DIR/any/<id>/INSTRUCTION.json` or `REPORT.json` | `OUTPUT_DIR/content/<id>/PROCESSED-*.md` | Execute work, invoke agent, collect output |
| **agent-viewer** | Scans all directories | Web UI/REST API | Aggregate and display flow state |

**Ownership Pattern**:
- **Input ownership**: The component that creates a file owns transforming it through the first processing stage
- **Output ownership**: The component that reads a file owns moving it to output/records
- **Shared monitoring**: The PR Poller (in agent-dispatch) monitors GitHub for changes and marks flows for re-processing

### GitHub Exposure for Approval

#### PR Polling System

```go
type PRState struct {
    Owner         string
    Repo          string
    Number        int
    LastCommentID string
    CommentCount  int
    Comments      []CommentInfo
    LastChecked   time.Time
}

type ChangeEvent struct {
    PR          PRState
    NewComments []CommentInfo
    Timestamp   time.Time
}
```

#### Dispatch Types with GitHub Integration

| Dispatch Type | GitHub Interaction | PR Lifecycle |
|--------------|-------------------|--------------|
| **Direct** | None | Fire-and-forget |
| **In-Repo** | Creates branch in target repo | PR created with work unit details, monitored for approval/rejection |
| **Repo-Isolation** | Creates separate isolation repo | Clones target repo state, creates PR in isolation repo, reintegration PR on approval |
| **Approval-Gated** | Requires manual approval | Stores PendingDispatch, polls for approval comment, executes when approved |

#### Monitoring Mechanism
- GraphQL API batch queries (multiple PRs per query)
- 30-second polling interval
- Detects new comments, triggers terraform apply on changes
- Marks flows for reprocessing when comments detected

### Record Collection

The system implements a three-layer recording architecture:

#### Layer 1: File Story (Operation Audit)

```
Controlled by: FILE_STORY_PATH env var
Records:
- File create/modify/read operations
- Checksums of all files (SHA256, 8-char prefix)
- Directory tree snapshots
- Diff before/after operations
```

#### Layer 2: Agent Audit (Invocation Context)

```
/agent-audit/
├── heuristic-request/<watcher-id>/<timestamp>/
│   ├── audit.json          # metadata
│   ├── prompt.txt          # exact prompt to agent
│   └── filesystem.txt      # tree of visible paths
└── agent-worker/<worker-id>/<timestamp>/
    ├── audit.json
    ├── prompt.txt
    └── filesystem.txt

Activation: AGENT_AUDIT=FULL environment variable
Purpose: Capture agent invocation context for replay/debugging
```

#### Layer 3: Dispatch Records

```go
type DispatchRecord struct {
    DispatcherID  string
    WorkUnitID    string
    WorkUnitType  string    // "instruction", "report"
    DispatchTime  string
    CompleteTime  string
    DurationMs    int64
    InputPath     string
    OutputPath    string
    Success       bool
    ExitCode      int
    Error         string
}
```

Stored across:
- `RECORDS_DIR/dispatch/` - Single-shot dispatch records
- `RECORDS_DIR/dispatch-watch/` - Watch mode flow records
- `agent-records/heuristic/` - Heuristic processing logs
- `agent-records/agent-worker/` - Worker invocation records

---

## Potential Directions for Alpha

### Direction 1: Explicit Flow Correlation Model

Introduce a first-class `FlowID` that explicitly links all stages of work processing:

```go
type UnifiedFlow struct {
    FlowID    string              // UUID for end-to-end correlation
    CreatedAt time.Time
    Origin    FlowOrigin
    Stages    []FlowStage
    Current   string              // Current stage name
    Status    string              // Aggregate status
    Metadata  map[string]string
}

type FlowOrigin struct {
    Type string    // "heuristic", "event", "instruction", "report", "dispatch"
    ID   string
    Path string
}

type FlowStage struct {
    Name      string    // "heuristic", "dispatch", "execution", "complete"
    Status    string    // "pending", "completed", "failed", "skipped"
    Path      string
    EnteredAt time.Time
    ExitedAt  time.Time
}
```

**Key Changes**:
- Every work item receives a FlowID at creation
- FlowID propagates through all transformations
- Centralized flow registry enables cross-component queries
- Parent-child relationships for nested workflows

**Trade-offs**:
- (+) Full observability across system boundaries
- (+) Enables sophisticated debugging and replay
- (-) Requires coordination between all components
- (-) Adds complexity to simple single-shot operations

### Direction 2: Event Sourcing with Filesystem Events

Transform the filesystem markers into a formal event stream:

```go
type FlowEvent struct {
    EventID     string
    FlowID      string
    EventType   string    // "created", "processing", "approved", "failed", etc.
    Timestamp   time.Time
    Component   string    // Which component emitted this event
    Payload     map[string]interface{}
    Previous    string    // Previous event ID (chain)
}
```

**Key Changes**:
- Marker files become append-only event logs
- Each component appends events rather than replacing state
- Event log enables reconstruction of any historical state
- Standardized event schema across all components

**Trade-offs**:
- (+) Complete audit trail with no information loss
- (+) Enables time-travel debugging
- (+) Natural fit for distributed systems
- (-) Larger storage footprint
- (-) More complex to query current state
- (-) Requires event compaction/cleanup strategy

### Direction 3: Database-Backed State with File Triggers

Separate data storage from work triggering:

```
Filesystem (Triggers):
  INPUT_DIR/any/<id>/DISPATCH.json  ← Triggers processing

Database (State):
  flows table: id, type, status, created_at, updated_at
  stages table: flow_id, name, status, entered_at, exited_at
  events table: flow_id, event_type, timestamp, payload
  records table: flow_id, audit_data, file_checksums
```

**Key Changes**:
- Filesystem remains the trigger mechanism (familiar pattern)
- All state transitions recorded in SQLite/PostgreSQL
- API-first access to flow state
- File markers become optional (backward compatibility)

**Trade-offs**:
- (+) Standard query patterns for state
- (+) ACID guarantees for state transitions
- (+) Easier to build dashboards and analytics
- (-) Introduces database dependency
- (-) Loses "everything is a file" simplicity
- (-) Requires migration strategy

---

## Questions for Alpha

### Identity and Correlation

1. How should work items be identified across system boundaries when they transform from one type to another (heuristic → dispatch → instruction)?

2. What is the relationship between a work unit's directory name, its internal identifiers, and any external identifiers (PR numbers, commit SHAs)?

3. Should flows support branching (one input creating multiple outputs) and if so, how should lineage be tracked?

4. How do we handle identity when work is retried or revised - is it a new flow or a continuation?

### State and Transitions

5. What state transitions should be considered atomic vs. eventually consistent?

6. How should conflicting state markers be resolved (e.g., both PROCESSING.md and PROCESSED.md exist)?

7. Should there be a concept of "stale" work that has been pending too long?

8. What happens to in-flight work when a component restarts?

### Boundaries and Ownership

9. At what point does ownership of data transfer from one component to another?

10. Should components be able to modify data they don't own, and under what circumstances?

11. How should cross-repository work be tracked when the isolation model creates new repositories?

12. What is the boundary between "system data" (flow state, audit logs) and "user data" (work outputs)?

### GitHub Integration

13. How should the system handle GitHub rate limiting during heavy PR polling?

14. What should happen when a PR is force-pushed or rebased - does the flow continue or restart?

15. Should approval come from specific users, teams, or CODEOWNERS?

16. How should the system handle repositories with branch protection rules that prevent direct pushes?

### Records and Observability

17. What is the retention policy for different types of records (audit logs, flow records, agent outputs)?

18. How should sensitive information (tokens, credentials, internal paths) be handled in records?

19. What level of detail should be recorded for failed operations vs. successful ones?

20. How do we balance storage costs with the need for comprehensive audit trails?

### Scalability and Performance

21. How many concurrent flows should the system support, and what are the bottlenecks?

22. Should large agent outputs be stored inline or referenced externally?

23. How should the filesystem-watching approach scale to thousands of pending work items?

24. What caching strategies are appropriate for frequently-accessed state?

### Recovery and Error Handling

25. How should partial failures be handled when a flow spans multiple components?

26. What mechanisms exist for manual intervention when automated recovery fails?

27. How should the system detect and recover from orphaned lock files?

28. Should there be a dead-letter queue for permanently failed work items?

### Evolution and Migration

29. How should schema changes to data structures be versioned and migrated?

30. What backward compatibility guarantees should exist between versions?

31. How should the system handle work items created by older versions of components?

32. What is the strategy for deprecating and removing obsolete data formats?

---

*This document is part of the design exploration for evolving the AI-sandboxing data model from experimental to alpha status.*
