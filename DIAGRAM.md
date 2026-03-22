# containment-dispatcher — Architecture Diagram

```mermaid
flowchart TD
    classDef input     fill:#1e3a5f,stroke:#4a9eff,color:#e0f0ff
    classDef component fill:#1a3a1a,stroke:#4caf50,color:#d0ffd0
    classDef output    fill:#3a1a1a,stroke:#ff6b6b,color:#ffd0d0
    classDef storage   fill:#2a2a1a,stroke:#f0c040,color:#fff0c0
    classDef terraform fill:#3a1a3a,stroke:#c084fc,color:#f0d0ff
    classDef external  fill:#1a2a3a,stroke:#60a0d0,color:#d0e8f8

    %% ── Inputs ──────────────────────────────────────────────────────────────
    subgraph INPUTS["  Inputs  "]
        HMD["HEURISTIC.md\n(high-level intent)"]:::input
        DJS["DISPATCH.json / .md\n(direct · in-repo · repo-isolation)"]:::input
        IJS["INSTRUCTION.json / .md\n(task for agent)"]:::input
        RJS["REPORT.json / .md\n(daily · weekly · monthly · custom)"]:::input
        EJS["events/config/*.json\n(timer · schedule)"]:::input
    end

    %% ── Components ──────────────────────────────────────────────────────────
    subgraph COMPONENTS["  Components  "]
        HR["heuristic-request\n(prompt → DISPATCH / INSTRUCTION)"]:::component
        AE["agent-events\n(cron scheduler)"]:::component
        ADW["agent-dispatch-watch\n(file watcher)"]:::component
        AD["agent-dispatch\n(unified CLI / watch)"]:::component
        AW["agent-worker\n(executes work units)"]:::component
        AA["ambiguous-agent\n(interactive REPL)"]:::component
    end

    %% ── AI Agents ────────────────────────────────────────────────────────────
    subgraph AGENTS["  AI Agents  "]
        direction LR
        AC["claude"]:::external
        AG["gemini"]:::external
        ACP["copilot"]:::external
        AOC["opencode"]:::external
        ACX["codex"]:::external
    end

    %% ── Storage ─────────────────────────────────────────────────────────────
    subgraph STORE["  Shared Storage  "]
        INQ["input/any/\n(work-unit queue)"]:::storage
        REC["agent-records/\n(audit logs · session records)"]:::storage
        REQ["requests/\n(heuristic output)"]:::storage
    end

    %% ── Terraform ────────────────────────────────────────────────────────────
    subgraph TF["  Terraform (PR / Isolation)  "]
        TFIR["in-repo\nPR branch"]:::terraform
        TFRI["repo-isolation\nclone"]:::terraform
    end

    %% ── Outputs ──────────────────────────────────────────────────────────────
    subgraph OUTPUTS["  Outputs  "]
        OC["output/content/\n(agent responses)"]:::output
        OR["output/records/\n(run metadata)"]:::output
        OP["output/&lt;dispatch-id&gt;/\n(terraform results)"]:::output
    end

    %% ── Flow: heuristics ─────────────────────────────────────────────────────
    HMD -->|"pending/<id>/HEURISTIC.md"| HR
    HR  -->|"prompt agent"| AC
    HR  -->|"writes DISPATCH.json / INSTRUCTION.json"| REQ
    REQ -->|"picked up"| ADW

    %% ── Flow: events ─────────────────────────────────────────────────────────
    EJS --> AE
    AE  -->|"writes REPORT.json"| INQ

    %% ── Flow: dispatch ───────────────────────────────────────────────────────
    DJS --> ADW
    DJS --> AD
    ADW -->|"direct → INSTRUCTION.json"| INQ
    ADW -->|"in-repo"| TFIR
    ADW -->|"repo-isolation"| TFRI
    AD  -->|"single-shot / watch"| ADW
    TFIR --> OP
    TFRI --> OP

    %% ── Flow: instructions ───────────────────────────────────────────────────
    IJS --> INQ
    RJS --> INQ
    INQ --> AW

    %% ── Flow: worker → agents ────────────────────────────────────────────────
    AW --> AC & AG & ACP & AOC & ACX
    AC & AG & ACP & AOC & ACX --> OC
    AW --> OR
    AW --> REC

    %% ── Flow: interactive ────────────────────────────────────────────────────
    AA -.->|"manual dispatch"| INQ

    %% ── Records ──────────────────────────────────────────────────────────────
    ADW --> REC
    HR  --> REC
```

## Data Flow Summary

| Source | Component | Destination |
|--------|-----------|-------------|
| `HEURISTIC.md` | `heuristic-request` | `requests/*/DISPATCH.json` or `INSTRUCTION.json` |
| `events/config/*.json` | `agent-events` | `input/any/*/REPORT.json` |
| `DISPATCH.json` (direct) | `agent-dispatch-watch` | `input/any/*/INSTRUCTION.json` |
| `DISPATCH.json` (in-repo) | `agent-dispatch-watch` → Terraform | `output/<id>/` (PR branch) |
| `DISPATCH.json` (repo-isolation) | `agent-dispatch-watch` → Terraform | `output/<id>/` (isolated clone) |
| `INSTRUCTION.json` / `REPORT.json` | `agent-worker` | `output/content/` + `output/records/` |

## Containment Strategies

```mermaid
flowchart LR
    classDef strategy fill:#2a1a3a,stroke:#c084fc,color:#f0d0ff

    D["DISPATCH.json"]
    D -->|type: direct| S1["In-place INSTRUCTION\n(same workspace)"]:::strategy
    D -->|type: in-repo| S2["PR branch\n(target repo, isolated branch)"]:::strategy
    D -->|type: repo-isolation| S3["Full clone\n(completely isolated repo)"]:::strategy
```
