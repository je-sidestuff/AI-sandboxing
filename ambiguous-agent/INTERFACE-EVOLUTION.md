# Ambiguous Agent Shell — Interface Evolution Strategies

> Design notes for evolving the ambiguous agent shell from a script-wrapping REPL
> into a full interface suite covering async dispatch, observability, and tool execution.

---

## Background: Current State

`ambiguous-agent/main.go` is a ~770-line Go REPL built on `readline` + `lipgloss`.
Its current surface area:

| Capability | Implementation |
|---|---|
| Interactive input | `readline` with multi-line support (backslash, heredoc, unclosed quotes) |
| Agent invocation | `agent <prompt>` → forks `invoke-agent.sh` synchronously |
| Agent switching | `set-agent <name>` updates prompt, passes `-a` flag to script |
| Model selection | `set-model <name>` / `list-models` for supported agents (e.g., opencode, grok) |
| Command execution | Arbitrary shell via `bash -c`, stdout/stderr tee'd to session log |
| Session recording | JSONL log + per-invocation `metadata.txt` / `raw_output.txt` |
| Tab completion | File-path completions via `ShellCompleter` |

What it **does not do** today:

- **Async dispatch** — `runAgent` blocks; the shell is frozen while the agent runs.
- **Dispatch type selection** — no surface for the three dispatch modes
  (`direct`, `in-repo`, `repo-isolation`) that `agent-dispatch` supports.
- **Observability** — no live view of in-flight invocations, past sessions, or
  worker state; that lives entirely in `agent-viewer` (separate web process).
- **Tool execution** — `declarative-tool-tool` (T3) exists but is not wired into
  the shell; there is no `tool <name> <inputs>` command.

The strategies below describe four distinct paths to closing these gaps, with
different trade-off profiles on complexity, disruption to the existing code,
and richness of the resulting interface.

---

## Strategy 1 — Command Surface Expansion (Additive / Low Disruption)

### Architecture

Keep `readline` and the synchronous REPL loop intact. Add new first-class
commands that delegate asynchronously through the existing dispatch and tool
infrastructure.

```
New shell commands
├── dispatch <type> <prompt>     — writes DISPATCH.json, forks agent-dispatch --once
├── dispatch-status [id]         — tails agent-records for a dispatch work unit
├── watch                        — starts/stops agent-dispatch in watch mode (background goroutine)
├── tool <name> [inputs...]      — invokes T3 via tool-tool execute
└── tools                        — lists registered T3s from ~/.tool-tool/registry.yaml
```

**Async model:** `runDispatch()` writes a `DISPATCH.json` to
`/workspaces/slopspaces/input/any/<uuid>/`, then launches `agent-dispatch --once
--check <uuid>` as a `cmd.Start()` (non-blocking). It records the PID and work
unit ID in an in-process `dispatchTracker` map. The user gets an immediate
prompt return and can check progress with `dispatch-status <id>`.

**Observability:** `dispatch-status` tails `agent-records/dispatch/<id>/` using
`fsnotify` or a simple polling loop, printing new lines as they appear — similar
to `tail -f` but inline in the shell. A `watch` command starts a background
goroutine that periodically scans `input/any/` for `DISPATCHING.md` files and
prints one-line status summaries.

**Tool execution:** `runTool()` is a thin wrapper: it locates the tool in
`~/.tool-tool/registry.yaml`, writes a temp input file, and shells out to
`tool-tool execute <name> <input>`.

### Trade-offs

| | |
|---|---|
| + | Zero disruption to existing REPL flow; new commands are purely additive |
| + | No new dependencies; uses existing dispatch infrastructure as-is |
| + | Async tracking is simple: a `map[string]*exec.Cmd` + a goroutine per job |
| − | Observability is text-stream-only; no structured live dashboard |
| − | Multiple concurrent dispatches produce interleaved output; hard to track |
| − | Tab-completion for dispatch types and tool names requires extending `ShellCompleter` |

### Implementation Approach

1. Add `dispatchTracker` struct to `main.go` with `sync.Map` of `id → cmd`.
2. Implement `parseDispatchArgs(line string) (dispatchType, prompt string)`.
3. In the main loop, add `dispatch`, `dispatch-status`, `watch`, `tool`, `tools`
   cases before the `agent` fallthrough.
4. Extend `ShellCompleter.Do` to provide dispatch-type and tool-name completions
   when the command prefix is `dispatch` or `tool`.
5. Add a `tailRecords(id string)` goroutine that prints new lines from
   `agent-records/dispatch/<id>/raw_output.txt` until the metadata shows
   `Exit Code:`.

Estimated scope: ~200–300 lines added to `main.go`, no new files required.

---

## Strategy 2 — Bubbletea Split-Screen TUI

### Architecture

Replace `readline` with [Bubbletea](https://github.com/charmbracelet/bubbletea)
(same Charm ecosystem as `lipgloss`, already a transitive dep). The terminal is
divided into two regions:

```
┌─────────────────────────────────────────────────────────────┐
│  OBSERVABILITY PANE                                          │
│  • Active dispatches (id, type, elapsed, status)             │
│  • Recent invocation tail (live from agent-records)          │
│  • Worker queue depth                                        │
├─────────────────────────────────────────────────────────────┤
│  INPUT PANE                                                  │
│  [claude] ~/workspace › dispatch repo-isolation "refactor…" │
└─────────────────────────────────────────────────────────────┘
```

The `Model` is the single source of truth. A background `tea.Cmd` uses
`fsnotify` to watch `agent-records/` and `input/any/` and sends `tea.Msg`
updates that the `Update()` function merges into the model. Dispatch, tool, and
agent commands fire goroutines via `tea.Cmd` (non-blocking by design in
Bubbletea).

**Dispatch integration:** `dispatch <type> <prompt>` writes a `DISPATCH.json`
and registers a `DispatchEntry{id, type, startTime, status}` in `model.active`.
The fsnotify watcher updates `status` as `DISPATCHING.md` / `PROCESSED.md`
appear.

**Observability data sources:**

| Data | Source | Update mechanism |
|---|---|---|
| Active dispatches | `input/any/*/DISPATCHING.md` | fsnotify |
| Completed invocations | `agent-records/*/metadata.txt` | fsnotify |
| Worker queue | `agent-records/worker/*.json` | fsnotify |
| Raw output tail | `agent-records/<id>/raw_output.txt` | fsnotify + line buffer |

**Tool execution:** `tool <name> <inputs>` fires a `tea.Cmd` goroutine that
runs `tool-tool execute`, captures stdout into a scrollable viewport in the
observability pane.

### Trade-offs

| | |
|---|---|
| + | Native live TUI; observability data is persistent on screen, not scrolled away |
| + | Multiple concurrent dispatches displayed simultaneously with individual status |
| + | Bubbletea's architecture enforces clean separation of state and rendering |
| + | Scrollable panes (via `bubbles/viewport`) let the user review past output |
| − | `readline` must be replaced or embedded inside a `textarea` bubble; history handling changes |
| − | `fsnotify` is a new dependency (though lightweight) |
| − | Bubbletea and Elm-style architecture require restructuring `main.go` significantly |
| − | Terminal resize handling, alternate-screen mode, and raw input all need care |

### Implementation Approach

1. Introduce `tui/model.go` with a `Model` struct holding:
   `activeDispatches []DispatchEntry`, `invocationTail []string`,
   `inputBuffer string`, `viewport viewport.Model`, `textarea textarea.Model`.
2. Move the command dispatch logic from the current `for` loop into a
   `executeCommand(m Model, line string) (Model, tea.Cmd)` function.
3. Create a `watchRecords(recordsPath string) tea.Cmd` that starts an fsnotify
   watcher and returns a channel-based `tea.Msg` stream.
4. Keep `invoke-agent.sh` and `agent-dispatch` as subprocess targets; they don't
   need to change.
5. The existing session JSONL logging can be preserved by wiring the
   `CommandRecord` encoder into the Bubbletea update cycle.

This is a moderate rewrite: the core shell logic (~300 lines of command
handling) is preserved, but the input/output plumbing changes substantially.

---

## Strategy 3 — Shell-as-Client / Daemon Model

### Architecture

Split the shell into two processes:

```
ambiguous-agent (thin client / TUI)
        │
        │  Unix domain socket  /tmp/ambiguous-<uid>.sock
        │
ambiguous-daemon (persistent background server)
        ├── Dispatch queue manager
        ├── Tool registry
        ├── Observability aggregator (fsnotify → event stream)
        └── Session multiplexer (multiple client connections)
```

The daemon manages all long-running state. The shell becomes a `netcat`-style
client that renders whatever the daemon sends. Multiple terminal sessions can
connect to the same daemon and see shared dispatch state.

**Protocol:** Newline-delimited JSON over the Unix socket:
```jsonl
{"type":"cmd","payload":{"line":"dispatch direct \"fix lint\""}}
{"type":"event","payload":{"kind":"dispatch_started","id":"abc123","dispatch_type":"direct"}}
{"type":"event","payload":{"kind":"output_line","id":"abc123","line":"Running agent..."}}
{"type":"event","payload":{"kind":"dispatch_done","id":"abc123","exit_code":0}}
```

The daemon holds the `dispatchTracker`, tool registry cache, and the fsnotify
watcher. It fans events out to all connected clients. Clients display events in
whatever TUI they choose (Bubbletea, raw ANSI, or the existing readline loop
with inline status lines).

**Async dispatch:** The daemon receives a `cmd` message, writes `DISPATCH.json`,
starts `agent-dispatch --once` as a managed subprocess, and streams its stdout
as `output_line` events back to all clients.

**Observability:** The daemon runs a single fsnotify instance watching
`agent-records/` and `input/any/`, aggregates events into a timeline, and
sends structured updates to clients. Clients can request a snapshot of current
state on connect.

### Trade-offs

| | |
|---|---|
| + | Multiple shell sessions share live state; ideal for tmux/split-pane workflows |
| + | Daemon survives shell exit; dispatches continue even if terminal is closed |
| + | Clean separation: daemon owns reliability, client owns presentation |
| + | Extendable to remote control (swap Unix socket for TCP/TLS without client changes) |
| − | Significant new code: daemon lifecycle, socket server, reconnect logic |
| − | Daemon crash recovery and state persistence add operational complexity |
| − | Debugging is harder: two processes, async protocol, potential race conditions |
| − | Overkill if only one session is ever active at a time |

### Implementation Approach

1. Create `ambiguous-daemon/main.go` with:
   - `net.Listen("unix", socketPath)` accept loop
   - `dispatchManager` that maps `id → *DispatchJob`
   - fsnotify watcher goroutine feeding an `events chan Event`
   - fan-out broadcaster to connected clients
2. Refactor `ambiguous-agent/main.go` to:
   - On startup, check for existing daemon socket; start daemon if absent
   - Replace `runAgent` / `runDispatch` with socket writes
   - Render incoming `event` messages as inline status or TUI pane
3. Define a shared `protocol/messages.go` package used by both binaries.
4. The daemon can be started via `systemd --user` or simply auto-forked by the
   shell on first launch.

---

## Strategy 4 — Declarative Command Manifest (T3-Driven Interface)

### Architecture

Lean into the existing `declarative-tool-tool` (T3) infrastructure to define
every shell command as a registered T3. The shell's job is reduced to:
1. Parse the input line into `(command-name, input-map)`.
2. Look up the T3 in the registry.
3. Validate inputs against the T3 schema.
4. Execute the T3 entrypoint as a subprocess.
5. Stream output back to the terminal.

```
shell input: "dispatch repo-isolation target=myorg/myrepo prompt='fix lint'"
        │
        ▼
Registry lookup: T3 "dispatch" → schema {type, target?, prompt, mode?}
        │
        ▼
Input validation: type=repo-isolation ✓, target=myorg/myrepo ✓, prompt=... ✓
        │
        ▼
Entrypoint: python dispatch_t3.py --input /tmp/input-<uuid>.json
        │
        ▼
Stdout streaming back to readline pane
```

New T3s are registered by dropping a YAML file into `~/.tool-tool/registry/`.
The shell auto-discovers them on startup and adds them to `ShellCompleter`.

**Async dispatch T3:** `dispatch_t3.py` writes a `DISPATCH.json`, launches
`agent-dispatch --once --wait`, and streams a JSON event stream on stdout. The
shell renders progress lines as they arrive. Cancellation sends SIGTERM to the
entrypoint.

**Observability T3:** A separate `observe_t3.py` T3 queries `agent-records/`
and prints a formatted ASCII table of recent invocations, active dispatches,
and worker state. The shell calls it periodically via a background goroutine, or
the user runs `observe` explicitly.

**Tool execution:** Any registered T3 is automatically a first-class shell
command. `tool-tool register <path>` makes it available; no shell changes
needed.

### Trade-offs

| | |
|---|---|
| + | Shell core stays small; all capability is in T3 definitions (Python, shell, etc.) |
| + | New capabilities added without recompiling the shell |
| + | Schema validation is built-in; bad inputs get clear errors before any subprocess starts |
| + | T3 library grows organically; useful outside the shell (CI, other tooling) |
| − | Shell must become a capable T3 runtime (schema parsing, input marshalling, async streaming) |
| − | Observability is pull-based unless T3s emit structured events the shell knows how to render |
| − | Cross-cutting concerns (session recording, JSONL log) must be factored into every T3 or handled by the shell wrapper |
| − | Python subprocess overhead per command; cold-start latency for frequently-used T3s |

### Implementation Approach

1. Complete `declarative-tool-tool` stubs: implement `register` (write to
   `~/.tool-tool/registry.yaml`) and `execute` (read registry, validate schema,
   run entrypoint with JSON input file).
2. Add a `loadRegistry() []T3` call in `main.go` at shell startup; store in
   a `registry []T3` field on the shell state.
3. Extend `ShellCompleter.Do` to complete registered T3 names and their input
   keys (`key=` completions).
4. Add a `runT3(name string, inputs map[string]string)` path in the main loop
   that delegates to `tool-tool execute`.
5. Define core T3s for dispatch (`dispatch_t3.py`), observe (`observe_t3.py`),
   and agent invocation (`agent_t3.py`) — replacing or wrapping `invoke-agent.sh`.

---

## Comparison Matrix

| Criterion | Strategy 1 (Additive) | Strategy 2 (Bubbletea TUI) | Strategy 3 (Daemon) | Strategy 4 (T3-Driven) |
|---|---|---|---|---|
| Async dispatch | Background goroutine + polling | `tea.Cmd` goroutine | Daemon subprocess manager | T3 entrypoint + event stream |
| Observability richness | Inline tail lines | Split-pane live dashboard | Shared state across sessions | Pull-based ASCII table |
| Tool execution | `tool-tool` subprocess | `tool-tool` via `tea.Cmd` | Via daemon, streamed to clients | Native via T3 registry |
| Disruption to existing code | Low (~200–300 lines added) | Medium (input loop restructured) | High (new daemon binary) | Medium (registry runtime added) |
| New dependencies | None | `bubbletea`, `bubbles`, `fsnotify` | `fsnotify` | None (T3 runtime in stdlib) |
| Multi-session support | No | No | Yes | No |
| Survives terminal close | No | No | Yes | No |
| Extensibility | Add cases to switch | Add bubbles | Add daemon handlers | Register new T3 YAML |

---

## Recommended Path

For the near term, **Strategy 1** (additive command surface) unblocks all three
requirements with the least risk:

- Async dispatch is achievable with `cmd.Start()` + a `dispatchTracker` map.
- Observability via `dispatch-status <id>` tail is immediately useful.
- Tool execution via `tool-tool execute` wires in T3s without a new runtime.

**Strategy 2** (Bubbletea TUI) is the right next step once the command surface
is stable — it reuses the same underlying dispatch and observability logic but
surfaces it in a persistent live view.

**Strategy 4** (T3-driven) is the right long-term architecture for tool
extensibility, and its prerequisites (completing `tool-tool register/execute`)
are needed regardless of which TUI strategy is chosen.

**Strategy 3** (daemon) should be deferred until there is a demonstrated need
for multi-session shared state or background-persistent dispatch jobs.
