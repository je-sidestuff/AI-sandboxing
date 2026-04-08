# Smart Models: Agent vs Model Separation

## Overview

This document tracks the implementation of proper separation between "agents" (CLI tools) and "models" (underlying LLMs) in the AI-sandboxing system.

## Problem Statement

Previously, the codebase conflated two distinct concepts:
- **Agent**: A CLI tool for interacting with AI (claude, opencode, gemini, copilot, codex, grok)
- **Model**: The underlying LLM the agent uses (claude-opus-4-5-20251101, gpt-4.1, gemini-2.5-pro, etc.)

The `AGENT_PRESET` environment variable and related code treated these as interchangeable, which prevented:
1. Using the same agent with different models
2. Tracking which specific model was used for an invocation
3. Specifying models independently from agent tools

## Implementation (Completed)

### 1. invoke-agent.sh Updates

- Renamed `PRESET_*` variables to `AGENT_*` for clarity
- Added new fields per agent:
  - `AGENT_<name>_MODEL_FLAG`: CLI flag to specify model (e.g., `--model`)
  - `AGENT_<name>_DEFAULT_MODEL`: Default model for the agent
  - `AGENT_<name>_MODELS`: Space-separated list of available models
- Added `-m <model>` flag for specifying model at invocation
- Added `--list-models <agent>` command to query available models
- Updated metadata.txt to include `Model:` field
- Renamed `AGENT_PRESET` env var to `AGENT_NAME` (backwards compatible)

### 2. ambiguous-agent main.go Updates

- Added `AgentModelConfig` struct to define model support per agent
- Added `agentModelConfigs` map with opencode's supported models
- Added new commands:
  - `list-models`: Show available models for current agent
  - `set-model <name>`: Set explicit model override
  - `clear-model`: Remove model override (use agent default)
- Updated prompt to show `[agent::model]` when model is explicitly set
- Model is automatically cleared when switching agents
- Updated invoking message to show agent name in color and model info
- Added `AGENT_MODEL` environment variable support

### 3. agent-worker Updates

- Updated to use `AGENT_NAME` (with `AGENT_PRESET` fallback for backwards compatibility)
- Added `grok` to available agents list to match invoke-agent.sh

### 4. heuristic-request Updates

- Split `Model` field in HeuristicData into separate `Agent` and `Model` fields
- Updated `executeAgent` to accept both agent and model overrides
- Model override is now passed via `-m` flag to invoke-agent.sh
- Added `AGENT_MODEL` to environment when model is specified

## Currently Supported

### Agents with Model Selection

| Agent | Model Flag | Available Models |
|-------|-----------|------------------|
| claude | `--model` | opus, sonnet, haiku, claude-opus-4-5-20251101, claude-sonnet-4-20250514, claude-sonnet-4-5-20250929 |
| gemini | `--model` | gemini-2.5-pro, gemini-2.5-flash, gemini-2.0-flash, gemini-2.0-flash-lite |
| opencode | `--model` | openai/gpt-4.1, openai/gpt-4.1-mini, openai/gpt-4.1-nano, openai/o4-mini, openai/o3, openai/o3-mini, anthropic/claude-sonnet-4-20250514, anthropic/claude-opus-4-5-20251101, google/gemini-2.5-pro, google/gemini-2.5-flash |
| grok | `--model` | grok-4.20-multi-agent, grok-4.20-reasoning, grok-4-fast, grok-3, grok-3-mini (+ aliases) |

### Agents Without Model Selection (Yet)

- `copilot` - Uses GitHub Copilot's default
- `codex` - Uses Codex's default

## Environment Variables

| Variable | Description |
|----------|-------------|
| `AGENT_NAME` | Specifies the agent CLI to use (new) |
| `AGENT_PRESET` | Deprecated alias for AGENT_NAME (backwards compatible) |
| `AGENT_MODEL` | Specifies the model to use (for agents that support it) |
| `AGENT_RECORDS_PATH` | Directory for agent records |
| `AGENT_FULL_AUTO` | Set to 1 for pre-approved execution |
| `AGENT_ADD_DIRS` | Additional directories to expose to agent |
| `AGENT_QUIET_CONTEXT` | Minimal context mode |

## Next Steps

1. ~~**Add model selection support to other agents** - Research CLI flags for claude, gemini, copilot~~ ✓ Done for claude and gemini
2. **Add model selection support to copilot** - Research CLI flags for GitHub Copilot
3. **Model validation** - Validate model names against available list before invocation
4. **Default model configuration** - Allow setting default models per agent in system config
5. **Model usage tracking** - Add model field to CommandRecord in session.jsonl
6. **Model cost tracking** - Track estimated costs per model for billing/budgeting

## Usage Examples

### Shell Commands

```bash
# List available models for opencode
list-models

# Set explicit model (must include provider prefix)
set-model openai/gpt-4.1

# Prompt now shows [opencode::openai/gpt-4.1]

# Clear model override
clear-model

# Prompt returns to [opencode]
```

### HEURISTIC.json

```json
{
  "message": "Analyze this code",
  "agent": "opencode",
  "model": "anthropic/claude-opus-4-5-20251101"
}
```

### Direct invoke-agent.sh

```bash
# List models
invoke-agent.sh --list-models opencode

# Invoke with specific model (must include provider prefix)
invoke-agent.sh -e -a opencode -m openai/gpt-4.1 "Write a hello world program"
```

## Breaking Changes

- `AGENT_PRESET` is deprecated in favor of `AGENT_NAME`
- The old behavior of using `model` field in HEURISTIC.json as an agent override no longer works; use `agent` field instead
- Environment variable semantics have changed (but backwards compatibility is maintained)
