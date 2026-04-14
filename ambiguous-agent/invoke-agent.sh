#!/usr/bin/env bash
#
# invoke-agent.sh - Universal wrapper for AI coding agents with records keeping
#
# Records each invocation with metadata, git context, and timing information.
# Groups related calls within a 20-minute window for easier review.
#

set -uo pipefail

# ============================================================================
# Configuration
# ============================================================================

RECORDS_PATH="${AGENT_RECORDS_PATH:-/workspaces/agent-records/}"
DEFAULT_AGENT="claude"

# ============================================================================
# Agent Definitions
# ============================================================================
# Each agent defines how to invoke a specific AI CLI tool.
# An "agent" is the CLI tool (claude, opencode, gemini, etc.)
# A "model" is the underlying LLM the agent uses (claude-opus-4-5-20251101, gpt-4o, etc.)
#
# Format: AGENT_<name>_CMD           - the base command to run
#         AGENT_<name>_PROMPT_FLAG   - flag to pass prompts
#         AGENT_<name>_EXEC_ARGS     - extra args for execute mode
#         AGENT_<name>_ADD_DIR_FLAG  - flag to add directories (if supported)
#         AGENT_<name>_MODEL_FLAG    - flag to specify model (if supported)
#         AGENT_<name>_DEFAULT_MODEL - default model for this agent
#         AGENT_<name>_MODELS        - space-separated list of available models

# Copilot CLI
AGENT_copilot_CMD="copilot"
AGENT_copilot_PROMPT_FLAG="-p"
AGENT_copilot_ADD_DIR_FLAG="--add-dir"
AGENT_copilot_EXEC_ARGS="--allow-all-tools"
AGENT_copilot_MODEL_FLAG=""
AGENT_copilot_DEFAULT_MODEL=""
AGENT_copilot_MODELS=""

# Gemini CLI - supports model selection via --model flag
AGENT_gemini_CMD="gemini"
AGENT_gemini_PROMPT_FLAG="-p"
AGENT_gemini_ADD_DIR_FLAG=""
AGENT_gemini_EXEC_ARGS="--sandbox=permissive"
AGENT_gemini_MODEL_FLAG="--model"
AGENT_gemini_DEFAULT_MODEL=""
AGENT_gemini_MODELS="gemini-2.5-pro gemini-2.5-flash gemini-2.0-flash-001 gemini-2.0-flash-lite"

# Claude Code - supports model selection via --model flag
AGENT_claude_CMD="claude"
AGENT_claude_PROMPT_FLAG="-p"
AGENT_claude_ADD_DIR_FLAG="--add-dir"
AGENT_claude_EXEC_ARGS="--permission-mode acceptEdits"
AGENT_claude_EXEC_ARGS_FULL_AUTO="--dangerously-skip-permissions"
AGENT_claude_MODEL_FLAG="--model"
AGENT_claude_DEFAULT_MODEL=""
AGENT_claude_MODELS="opus sonnet haiku claude-opus-4-5-20251101 claude-sonnet-4-20250514 claude-sonnet-4-5-20250929"

# OpenCode - supports model selection via --model flag
AGENT_opencode_CMD="opencode run"
AGENT_opencode_PROMPT_FLAG=""
AGENT_opencode_ADD_DIR_FLAG=""
AGENT_opencode_EXEC_ARGS=""
AGENT_opencode_MODEL_FLAG="--model"
AGENT_opencode_DEFAULT_MODEL=""
AGENT_opencode_MODELS="openai/gpt-4.1 openai/gpt-4.1-mini openai/gpt-4.1-nano openai/o4-mini openai/o3 openai/o3-mini anthropic/claude-sonnet-4-20250514 anthropic/claude-opus-4-5-20251101 google/gemini-2.5-pro google/gemini-2.5-flash"

# Codex
AGENT_codex_CMD="codex"
AGENT_codex_PROMPT_FLAG="-p"
AGENT_codex_ADD_DIR_FLAG=""
AGENT_codex_EXEC_ARGS="--full-auto"
AGENT_codex_MODEL_FLAG=""
AGENT_codex_DEFAULT_MODEL=""
AGENT_codex_MODELS=""

# Grok (xAI)
AGENT_grok_CMD="grok"
AGENT_grok_PROMPT_FLAG="-p"
AGENT_grok_ADD_DIR_FLAG=""
AGENT_grok_EXEC_ARGS=""
AGENT_grok_MODEL_FLAG="--model"
AGENT_grok_DEFAULT_MODEL=""
AGENT_grok_MODELS="grok-4.20-multi-agent-0309 grok-4.20-multi-agent grok-4.20-multi-agent-beta grok-4.20-0309-reasoning grok-4.20-beta-0309 grok-4.20-beta grok-beta grok-4.20-0309-non-reasoning grok-4-1-fast-reasoning grok-4-1-fast grok-4-1-fast-non-reasoning grok-4-fast-reasoning grok-4-fast grok-4-fast-non-reasoning grok-4-0709 grok-code-fast-1 grok-code-fast grok-3 grok-3-mini grok-3-mini-fast"

# ============================================================================
# Capability Definitions
# ============================================================================
# Capabilities map task types to optimal agent/model combinations.
# Format: CAPABILITY_<name>_AGENT  - the agent to use
#         CAPABILITY_<name>_MODEL  - the model to use

# Image capability - uses grok with vision model
CAPABILITY_image_AGENT="grok"
CAPABILITY_image_MODEL="grok-code-fast-1"

# Cheap capability - uses fast/inexpensive model for simple tasks
CAPABILITY_cheap_AGENT="opencode"
CAPABILITY_cheap_MODEL="google/gemini-2.5-flash"

# ============================================================================
# Functions
# ============================================================================

usage() {
    cat >&2 <<EOF
Usage: $(basename "$0") [-e|-p] [-a <agent>] [-m <model>] [-c <capability>] [-s] [-f <file>] <prompt...>
       $(basename "$0") --list-models <agent>
       $(basename "$0") --list-capabilities

Modes:
  -p  Prompt mode: ask questions, read-only context
  -e  Execute mode: allow tool use and file modifications

Options:
  -a <agent>       Select agent CLI tool (default: $DEFAULT_AGENT)
  -m <model>       Select model for agent (if supported by agent)
  -c <capability>  Select capability (auto-selects best agent/model for the task)
  -s               Session context: indicates invocation is from a parent session
  -f <file>        Read prompt from file (use - for stdin)

Commands:
  --list-models <agent>  List available models for an agent
  --list-capabilities    List available capabilities

Available agents:
  copilot   GitHub Copilot CLI
  gemini    Google Gemini CLI
  claude    Claude Code (default)
  opencode  OpenCode (supports model selection)
  codex     OpenAI Codex
  grok      xAI Grok

Environment:
  AGENT_RECORDS_PATH   Directory for records storage (default: $RECORDS_PATH)
  AGENT_NAME           Default agent (overrides built-in default)
  AGENT_MODEL          Default model for agent (if supported)
  AGENT_QUIET_CONTEXT  Set to 1 to use minimal context prefix (path only)
  AGENT_FULL_AUTO      Set to 1 for pre-approved execution (skips permission prompts)
  AGENT_ADD_DIRS       Colon-separated list of additional directories to add via --add-dir

EOF
    exit 1
}

get_agent_var() {
    local agent="$1"
    local var="$2"
    local full_auto="${3:-false}"

    # If full_auto mode and requesting EXEC_ARGS, check for _FULL_AUTO variant first
    if [[ "$full_auto" == "true" && "$var" == "EXEC_ARGS" ]]; then
        local full_auto_varname="AGENT_${agent}_${var}_FULL_AUTO"
        if [[ -n "${!full_auto_varname:-}" ]]; then
            echo "${!full_auto_varname}"
            return
        fi
    fi

    local varname="AGENT_${agent}_${var}"
    echo "${!varname:-}"
}

# Get capability configuration
get_capability_var() {
    local capability="$1"
    local var="$2"
    local varname="CAPABILITY_${capability}_${var}"
    echo "${!varname:-}"
}

# List available capabilities
list_capabilities() {
    echo "Available capabilities:"
    echo "  image   - Image understanding and generation (uses grok::grok-code-fast-1)"
    echo "  cheap   - Fast and inexpensive tasks (uses opencode::google/gemini-2.5-flash)"
}

# List available models for an agent
list_agent_models() {
    local agent="$1"
    local models
    local dynamic_models=""

    if [[ "$agent" == "opencode" ]]; then
        # Query models dynamically from the tool
        if command -v opencode >/dev/null 2>&1; then
            dynamic_models=$(opencode models 2>/dev/null)
            if [[ $? -eq 0 ]]; then
                models="$dynamic_models"
            fi
        fi
    elif [[ "$agent" == "grok" ]]; then
        # Query models dynamically from the tool if supported
        if command -v grok >/dev/null 2>&1; then
            dynamic_models=$(grok models 2>/dev/null)
            if [[ $? -eq 0 ]]; then
                models="$dynamic_models"
            fi
        fi
    fi

    if [[ -z "$models" ]]; then
        models=$(get_agent_var "$agent" "MODELS")
    fi

    local default_model
    default_model=$(get_agent_var "$agent" "DEFAULT_MODEL")

    if [[ -z "$models" ]]; then
        echo "Agent '$agent' does not support model selection" >&2
        return 1
    fi

    echo "Available models for '$agent':"
    echo "$models" | while IFS= read -r model; do
        [[ -z "$model" ]] && continue
        if [[ "$model" == "$default_model" ]]; then
            echo "  $model (default)"
        else
            echo "  $model"
        fi
    done

    if [[ -z "$default_model" ]]; then
        echo ""
        echo "No default model specified - agent uses its built-in default"
    fi
}

format_duration() {
    local seconds="$1"
    if (( seconds >= 3600 )); then
        echo "$((seconds / 3600))h $(( (seconds % 3600) / 60 ))m $((seconds % 60))s"
    elif (( seconds >= 60 )); then
        echo "$((seconds / 60))m $((seconds % 60))s"
    else
        echo "${seconds}s"
    fi
}

gather_git_info() {
    local dir="$1"

    if ! git -C "$dir" rev-parse --is-inside-work-tree &>/dev/null; then
        GIT_REPO="N/A"
        GIT_BRANCH="N/A"
        GIT_DIRTY="N/A"
        return
    fi

    GIT_REPO=$(git -C "$dir" remote get-url origin 2>/dev/null \
               || git -C "$dir" rev-parse --show-toplevel 2>/dev/null \
               || echo "N/A")
    GIT_BRANCH=$(git -C "$dir" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "N/A")

    if [[ -n "$(git -C "$dir" status --porcelain 2>/dev/null)" ]]; then
        GIT_DIRTY="true"
    else
        GIT_DIRTY="false"
    fi
}

find_recent_group_id() {
    local records_dir="$1"
    local current_ts="$2"
    local cutoff=$((current_ts - 1200))

    for dir in "$records_dir"/*/; do
        [[ -d "$dir" ]] || continue

        local dirname
        dirname=$(basename "$dir")

        local dir_ts
        dir_ts=$(echo "$dirname" | grep -oE '[0-9]{10}$' || true)
        [[ -z "$dir_ts" ]] && continue
        (( dir_ts < cutoff )) && continue

        local meta="${dir}/metadata.txt"
        [[ -f "$meta" ]] || continue

        local existing_group
        existing_group=$(grep '^Group ID:' "$meta" 2>/dev/null | sed 's/^Group ID: //' | head -1)

        if [[ -n "$existing_group" ]]; then
            echo "$existing_group"
            return
        fi
    done

    echo "$current_ts"
}

build_agent_command() {
    local agent="$1"
    local mode="$2"
    local call_pwd="$3"
    local records_path="$4"
    local full_auto="$5"
    local model="$6"
    shift 6

    local cmd
    cmd=$(get_agent_var "$agent" "CMD")
    local prompt_flag
    prompt_flag=$(get_agent_var "$agent" "PROMPT_FLAG")
    local add_dir_flag
    add_dir_flag=$(get_agent_var "$agent" "ADD_DIR_FLAG")
    local model_flag
    model_flag=$(get_agent_var "$agent" "MODEL_FLAG")
    local exec_args
    # Use full-auto exec args if full_auto is enabled
    local use_full_auto="false"
    [[ "$full_auto" == "1" ]] && use_full_auto="true"
    exec_args=$(get_agent_var "$agent" "EXEC_ARGS" "$use_full_auto")

    if [[ -z "$cmd" ]]; then
        echo "Error: Unknown agent '$agent'" >&2
        return 1
    fi

    local args=()

    # Add model flag if model is specified and agent supports it
    if [[ -n "$model" && -n "$model_flag" ]]; then
        args+=("$model_flag" "$model")
    fi

    if [[ "$mode" == "-e" && -n "$exec_args" ]]; then
        read -ra exec_args_arr <<< "$exec_args"
        args+=("${exec_args_arr[@]}")
    fi

    if [[ -n "$add_dir_flag" ]]; then
        args+=("$add_dir_flag" "$records_path")
        if [[ "$mode" == "-e" ]]; then
            args+=("$add_dir_flag" "$call_pwd")
        fi
        # Add any additional directories from AGENT_ADD_DIRS (colon-separated)
        if [[ -n "${AGENT_ADD_DIRS:-}" ]]; then
            IFS=':' read -ra extra_dirs <<< "$AGENT_ADD_DIRS"
            for extra_dir in "${extra_dirs[@]}"; do
                if [[ -n "$extra_dir" && -d "$extra_dir" ]]; then
                    args+=("$add_dir_flag" "$extra_dir")
                fi
            done
        fi
    fi

    # Build the prompt - optionally append records path context
    # Note: Context is appended (not prepended) so the actual task is seen first
    local prompt_with_context
    if [[ "${AGENT_QUIET_CONTEXT:-}" == "1" ]]; then
        # Quiet mode: just include the path hint
        prompt_with_context="$*

(Records: ${records_path%/})"
    else
        # Default: include context explaining continuity, appended after the main prompt
        prompt_with_context="$*

[Previous invocation records available at: ${records_path%/}]"
    fi

    if [[ -n "$prompt_flag" ]]; then
        args+=("$prompt_flag" "$prompt_with_context")
    else
        args+=("$prompt_with_context")
    fi

    # Output command and args using NUL separators to preserve multi-line content
    printf '%s\0' "$cmd" "${args[@]}"
}

# ============================================================================
# Main
# ============================================================================

# Use AGENT_NAME env var, fall back to deprecated AGENT_PRESET for backwards compatibility
AGENT="${AGENT_NAME:-${AGENT_PRESET:-$DEFAULT_AGENT}}"
MODEL="${AGENT_MODEL:-}"
CAPABILITY=""
MODE=""
SESSION_CONTEXT="false"
PROMPT_FILE=""
FULL_AUTO="${AGENT_FULL_AUTO:-0}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        -e|-p)
            MODE="$1"
            shift
            ;;
        -a)
            [[ $# -lt 2 ]] && usage
            AGENT="$2"
            shift 2
            ;;
        -m)
            [[ $# -lt 2 ]] && usage
            MODEL="$2"
            shift 2
            ;;
        -c)
            [[ $# -lt 2 ]] && usage
            CAPABILITY="$2"
            shift 2
            ;;
        -s|--session)
            SESSION_CONTEXT="true"
            shift
            ;;
        --full-auto)
            FULL_AUTO="1"
            shift
            ;;
        --list-models)
            [[ $# -lt 2 ]] && usage
            list_agent_models "$2"
            exit $?
            ;;
        --list-capabilities)
            list_capabilities
            exit 0
            ;;
        -f)
            [[ $# -lt 2 ]] && usage
            PROMPT_FILE="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            break
            ;;
    esac
done

[[ -z "$MODE" ]] && usage

# Apply capability override if specified
if [[ -n "$CAPABILITY" ]]; then
    CAP_AGENT=$(get_capability_var "$CAPABILITY" "AGENT")
    CAP_MODEL=$(get_capability_var "$CAPABILITY" "MODEL")
    if [[ -z "$CAP_AGENT" ]]; then
        echo "Error: Unknown capability '$CAPABILITY'" >&2
        echo "Use --list-capabilities to see available capabilities" >&2
        exit 1
    fi
    AGENT="$CAP_AGENT"
    MODEL="$CAP_MODEL"
fi

# Read prompt from file if -f was specified
if [[ -n "$PROMPT_FILE" ]]; then
    if [[ "$PROMPT_FILE" == "-" ]]; then
        PROMPT=$(cat)
    elif [[ -f "$PROMPT_FILE" ]]; then
        PROMPT=$(cat "$PROMPT_FILE")
    else
        echo "Error: File not found: $PROMPT_FILE" >&2
        exit 1
    fi
    set -- "$PROMPT"
fi

[[ $# -lt 1 ]] && usage

CALL_PWD=$(pwd)
NOW_UNIX=$(date +%s)
NOW_HUMAN=$(date '+%Y-%m-%d %H:%M:%S')
NOW_DIR=$(date '+%Y-%m-%d_%H-%M-%S')

mkdir -p "$RECORDS_PATH"

GROUP_ID=$(find_recent_group_id "$RECORDS_PATH" "$NOW_UNIX")

INVOCATION_DIR="${RECORDS_PATH%/}/${NOW_DIR}_${NOW_UNIX}"
mkdir -p "$INVOCATION_DIR"

gather_git_info "$CALL_PWD"

# Use NUL-separated values to properly handle multi-line prompts
mapfile -t -d '' cmd_parts < <(build_agent_command "$AGENT" "$MODE" "$CALL_PWD" "$RECORDS_PATH" "$FULL_AUTO" "$MODEL" "$@")
[[ ${#cmd_parts[@]} -eq 0 ]] && exit 1

# Split the command into words (handles multi-word commands like "opencode run")
read -ra AGENT_CMD_PARTS <<< "${cmd_parts[0]}"
AGENT_ARGS=("${cmd_parts[@]:1}")

START_TIME=$(date +%s)

AGENT_EXIT=0
"${AGENT_CMD_PARTS[@]}" "${AGENT_ARGS[@]}" 2>&1 | tee "$INVOCATION_DIR/raw_output.txt"
AGENT_EXIT="${PIPESTATUS[0]}"

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
DURATION_STR=$(format_duration "$DURATION")

cat > "$INVOCATION_DIR/metadata.txt" <<EOF
Date/Time:  $NOW_HUMAN
Agent:      $AGENT (${cmd_parts[0]})
Model:      ${MODEL:-"(default)"}
Capability: ${CAPABILITY:-"(none)"}
Mode:       $MODE
Full Auto:  $FULL_AUTO
Session:    $SESSION_CONTEXT
Prompt:     $*
PWD:        $CALL_PWD
Git Repo:   $GIT_REPO
Git Branch: $GIT_BRANCH
Git Dirty:  $GIT_DIRTY
Duration:   $DURATION_STR
Group ID:   $GROUP_ID
Exit Code:  $AGENT_EXIT
EOF

echo "" >&2
echo "Records saved to: $INVOCATION_DIR" >&2

exit "$AGENT_EXIT"
