# Complete Agent & Model Selection for Heuristic Agents

## Overview

This document outlines the implementation of full agent and model selection capabilities across the heuristic agent set (`heuristic-request`, `agent-worker`, and supporting components like `agent-dispatch` and `invoke-agent.sh`). 

The goal is to support **two selection modes**:
1. **Direct parameters**: Explicit `agent`, `model`, or `capability` specified in HEURISTIC.json, INSTRUCTION.json, DISPATCH.json, env vars, or CLI flags.
2. **Capability requirements**: Automatic selection based on task analysis against a registry of agent/model capabilities (reasoning depth, code execution, web access, cost tier, speed, vision, etc.).

This builds directly on the Agent vs Model separation from `SmartModels.md` and the partial capability support already present in `ambiguous-agent` and `invoke-agent.sh`.

## Current State (from codebase audit)

- `ambiguous-agent/main.go`: Has `capabilityConfigs` map (e.g. "image", "cheap") and `extractCapability()`, `runAgent()` logic for `-c`.
- `invoke-agent.sh`: Supports `-c <capability>`, resolves via shell functions to override AGENT/MODEL, records capability in metadata.
- `heuristic-request/main.go` & `agent-worker/main.go`: Support direct `agent` (and partial `model`) overrides from JSON structs; pass `-a`/`-m` + env to invoke-agent.sh. Limited capability propagation.
- `HeuristicData`, `Instruction`, `Report`, `Dispatch` structs have `Agent` but inconsistent `Model`/`Capability` fields.
- Heuristic prompt in `heuristic-request` is sophisticated but does not yet output recommended agent/model/capability.
- No shared `pkg/agentselect` — duplication across components.
- Auditing (`pkg/agentaudit`) and records capture current agent but not full selection reasoning.

Selection flow today: env/default > JSON direct override > capability lookup (partial) > invoke-agent.sh final resolution.

## Proposed Implementation

### 1. Core Selector (`pkg/agentselect`)
- New package with:
  - `CapabilityRegistry`: map of capabilities to preferred (agent, model, priority, rationale).
  - `AgentModelProfile`: capabilities, strengths, cost tier, flags supported per pair.
  - `Select(ctx, taskDesc, directAgent, directModel, capability string) (agent, model, usedCapability string, rationale string)`
- Supports fallback chain and optional lightweight heuristic call.
- Reusable by all mains, dispatch, and ambiguous-agent.

### 2. Data Model Updates
- Standardize `Agent`, `Model`, `Capability`, `SelectionRationale` fields across all JSON structs and Go types.
- Update heuristic prompt to emit these in output JSON when not explicitly provided.

### 3. Component Updates
- **heuristic-request**: Use selector in `processHeuristicUnit()` / `executeAgent()`. Let the heuristic AI recommend capabilities.
- **agent-worker**: Extend `getAgent()` / `execute*Agent()` to use selector for INSTRUCTION/REPORT.
- **agent-dispatch**: Propagate selection fields through Terraform flows and approval.
- **invoke-agent.sh**: Full support for capability-driven selection, expose `--list-capabilities`.

## Next Steps

Updated with implementation plan incorporating all HUMAN ANSWERS (incremental: only 'image','cheap'; registry in code; substring triggers anywhere in input for "We will use agent X"/"model Y"/"agent with Z capability"; direct precedence (error on incompatible agent+model+capability); heuristic-request uses default unless driven by strings; worker schema for settings (not always in prompt); agent-dispatch minimal conveyance; full picture in audits).

**Dispatch Choice**: Break up into many smaller tasks for fast coding agents (aligns with incremental strategy and heuristic task breakdown). One large task to powerful reasoning model rejected to leverage the system being built. All testing together by humans once complete (no during-testing).

### Detailed Implementation Plan
1. Create `pkg/agentselect` (static Go registry for 'image'/'cheap' -> agent/model mappings, Select() fn handling direct/capability/substring parsing, precedence, rationale, error on conflicts).
2. Standardize structs across heuristic-request, agent-worker, agent-dispatch, reports, audits (add Model, Capability, SelectionRationale).
3. Update heuristic-request: parse selection substrings in input, enhance prompt for optional selection JSON output, integrate selector in process/execute (default high-capability unless specified).
4. Update invoke-agent.sh, ambiguous-agent/main.go for full capability support, --list-capabilities, substring recognition.
5. Update agent-worker to use selector for INSTRUCTION/REPORT units, propagate via schema without always surfacing in prompt.
6. Update agent-dispatch to convey selection fields from heuristic output to workers/Terraform/approvals.
7. Enhance pkg/agentaudit and session records for complete selection picture (rationale, conflicts, etc.).
8. Revise SmartModels.md + this doc; expand registry later.

(Original steps 2-7 now mapped to above; start with pkg/agentselect.)

## Open Questions for Humans

1. **Selection Strategy**: Should capability selection be purely static registry (fast, deterministic) or allow a meta-heuristic LLM call for complex tasks? What is the performance budget?

```HUMAN ANSWER: 
  When heuristic-agent creates tasks and flows for agent-worker to action it will be aware of the ability to select, but at first it will not be encouraged to with default language, it will only understand when the requester is asking.

  When requests are input into heuristic-request we can add a special exact instruction substring to trigger the use of different agents/models. We will be able to say:
  - "We will use agent <agent>" (ie: We will use agent grok.)
  - "We will use model <model>" (ie: We will use model xai/grok-4.20-0309-reasoning to create a comprehensive plan)
  - "We will use an agent with <capability> capability" (ie: We will use an agent with video capability to draw the diagram.)

  A substring is allowed anywhere in the input and will be interpreted by the heuristic interpreter as well as recognized as a specific instruction string.

  The setting will become available as input in the schema of the worker work units and will not need to be visible in the message/prompt to take effect.
```

2. **Capability Taxonomy**: What specific capabilities should be in the initial registry? Please provide prioritized list (e.g. reasoning-depth, tool-use, multimodal, cost-sensitivity, context-window, speed).

```HUMAN ANSWER: 
  We are only starting with the two currently defined capabilities: 'image', and 'cheap'.

  We are starting with these as an incremental development strategy, we will expand this subsequently.
```

3. **Heuristic Self-Selection**: Should `heuristic-request` always run on a fixed high-capability model (e.g. `grok-4.20-reasoning`), or allow it to be selected via the same system?

```HUMAN ANSWER: 
  The heuristic-request agent will use its default by default, and will be drivable through the specific selection-stringa.
```

4. **Conflict Resolution**: Precedence rules when direct params, capability, and heuristic recommendation disagree? How to surface conflicts in audits?

```HUMAN ANSWER: 
  Direct assignment will take precedents over a capability but will error if an incompatible agent+model and capability are selected.
```
5. **Registry Maintenance**: Should profiles be in code, JSON config, or dynamically updated? Any desire for per-workspace or per-user overrides?

```HUMAN ANSWER:
  In code as a first step, specifically for incremental development purposes.
```
6. **Observability**: What additional fields should `audit.json` and session records capture about the selection decision?

```HUMAN ANSWER:
  Those needed to paint a ccomplete picture.
```

7. **Scope**: Should this also extend to `agent-dispatch` Terraform flows and approval gates? Any constraints from containment/execution modules?

```HUMAN ANSWER:
  Agent dispatch needs to have the minimum capacity necessary to properly convey the output of heuristic-request to get it to an agent-worker.
```

---
*Last updated: 2026-04-11. References previous SmartModels.md and agent audit work.* -- xai/grok-4.20-0309-reasoning
*Last updated: 2026-04-11. Meat-poked answers into place.* -- human
