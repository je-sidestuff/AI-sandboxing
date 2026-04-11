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

### 4. Capability Registry Design
Define categories like:
- `high-reasoning` → grok-4.20-reasoning or claude-opus
- `code-analysis` → opencode with strong model
- `fast-cheap` → gemini-flash or o3-mini
- `execution` → agents with tool use
- `heuristic-processor` → dedicated strong model for the heuristic itself

## Next Steps

1. Create `discussion/CompleteAgentSelect.md` (this document) ✓
2. Implement `pkg/agentselect` with registry and selection logic.
3. Update all structs, prompts, and main.go files to use the shared selector.
4. Expand and document the capability registry.
5. Enhance heuristic prompt to support capability recommendation output.
6. Update audits/records to capture selection rationale.
7. Test end-to-end flows (direct, capability, default).
8. Revise `SmartModels.md` with completed implementation details.

## Open Questions for Humans

1. **Selection Strategy**: Should capability selection be purely static registry (fast, deterministic) or allow a meta-heuristic LLM call for complex tasks? What is the performance budget?

2. **Capability Taxonomy**: What specific capabilities should be in the initial registry? Please provide prioritized list (e.g. reasoning-depth, tool-use, multimodal, cost-sensitivity, context-window, speed).

3. **Heuristic Self-Selection**: Should `heuristic-request` always run on a fixed high-capability model (e.g. `grok-4.20-reasoning`), or allow it to be selected via the same system?

4. **Conflict Resolution**: Precedence rules when direct params, capability, and heuristic recommendation disagree? How to surface conflicts in audits?

5. **Registry Maintenance**: Should profiles be in code, JSON config, or dynamically updated? Any desire for per-workspace or per-user overrides?

6. **Observability**: What additional fields should `audit.json` and session records capture about the selection decision?

7. **Scope**: Should this also extend to `agent-dispatch` Terraform flows and approval gates? Any constraints from containment/execution modules?

Please review and provide guidance on the questions above before implementation begins.

---
*Last updated: 2026-04-11. References previous SmartModels.md and agent audit work.*
