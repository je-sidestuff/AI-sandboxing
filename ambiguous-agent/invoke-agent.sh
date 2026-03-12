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
DEFAULT_PRESET="claude"

# ============================================================================
# Agent Presets
# ============================================================================
# Each preset defines how to invoke a specific AI agent.
# Format: PRESET_<name>_CMD contains the base command
#         PRESET_<name>_PROMPT_FLAG contains the prompt flag
#         PRESET_<name>_EXEC_ARGS contains extra args for execute mode
#         PRESET_<name>_ADD_DIR_FLAG contains flag to add directories (if supported)

# Copilot CLI
PRESET_copilot_CMD="copilot"
PRESET_copilot_PROMPT_FLAG="-p"
PRESET_copilot_ADD_DIR_FLAG="--add-dir"
PRESET_copilot_EXEC_ARGS="--allow-all-tools"

# Gemini CLI
PRESET_gemini_CMD="gemini"
PRESET_gemini_PROMPT_FLAG="-p"
PRESET_gemini_ADD_DIR_FLAG=""
PRESET_gemini_EXEC_ARGS="--sandbox=permissive"

# Claude Code
PRESET_claude_CMD="claude"
PRESET_claude_PROMPT_FLAG="-p"
PRESET_claude_ADD_DIR_FLAG="--add-dir"
PRESET_claude_EXEC_ARGS="--dangerously-skip-permissions"

# OpenCode
PRESET_opencode_CMD="opencode"
PRESET_opencode_PROMPT_FLAG="-p"
PRESET_opencode_ADD_DIR_FLAG=""
PRESET_opencode_EXEC_ARGS="--auto-approve"

# Codex
PRESET_codex_CMD="codex"
PRESET_codex_PROMPT_FLAG="-p"
PRESET_codex_ADD_DIR_FLAG=""
PRESET_codex_EXEC_ARGS="--full-auto"

# ============================================================================
# Functions
# ============================================================================

usage() {
    cat >&2 <<EOF
Usage: $(basename "$0") [-e|-p] [-a <agent>] <prompt...>

Modes:
  -p  Prompt mode: ask questions, read-only context
  -e  Execute mode: allow tool use and file modifications

Options:
  -a <agent>  Select agent preset (default: $DEFAULT_PRESET)

Available presets:
  copilot   GitHub Copilot CLI
  gemini    Google Gemini CLI
  claude    Claude Code (default)
  opencode  OpenCode
  codex     OpenAI Codex

Environment:
  AGENT_RECORDS_PATH  Directory for records storage (default: $RECORDS_PATH)
  AGENT_PRESET        Default agent preset (overrides built-in default)

EOF
    exit 1
}

get_preset_var() {
    local preset="$1"
    local var="$2"
    local varname="PRESET_${preset}_${var}"
    echo "${!varname:-}"
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
    local preset="$1"
    local mode="$2"
    local call_pwd="$3"
    local records_path="$4"
    shift 4

    local cmd
    cmd=$(get_preset_var "$preset" "CMD")
    local prompt_flag
    prompt_flag=$(get_preset_var "$preset" "PROMPT_FLAG")
    local add_dir_flag
    add_dir_flag=$(get_preset_var "$preset" "ADD_DIR_FLAG")
    local exec_args
    exec_args=$(get_preset_var "$preset" "EXEC_ARGS")

    if [[ -z "$cmd" ]]; then
        echo "Error: Unknown preset '$preset'" >&2
        return 1
    fi

    local args=()

    if [[ "$mode" == "-e" && -n "$exec_args" ]]; then
        read -ra exec_args_arr <<< "$exec_args"
        args+=("${exec_args_arr[@]}")
    fi

    if [[ -n "$add_dir_flag" ]]; then
        args+=("$add_dir_flag" "$records_path")
        if [[ "$mode" == "-e" ]]; then
            args+=("$add_dir_flag" "$call_pwd")
        fi
    fi

    # Build the prompt with records path context prepended
    local prompt_with_context="[Agent records directory: ${records_path}]

$*"

    args+=("$prompt_flag" "$prompt_with_context")

    echo "$cmd"
    printf '%s\n' "${args[@]}"
}

# ============================================================================
# Main
# ============================================================================

PRESET="${AGENT_PRESET:-$DEFAULT_PRESET}"
MODE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -e|-p)
            MODE="$1"
            shift
            ;;
        -a)
            [[ $# -lt 2 ]] && usage
            PRESET="$2"
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
[[ $# -lt 1 ]] && usage

CALL_PWD=$(pwd)
NOW_UNIX=$(date +%s)
NOW_HUMAN=$(date '+%Y-%m-%d %H:%M:%S')
NOW_DIR=$(date '+%Y-%m-%d_%H-%M-%S')

mkdir -p "$RECORDS_PATH"

GROUP_ID=$(find_recent_group_id "$RECORDS_PATH" "$NOW_UNIX")

INVOCATION_DIR="${RECORDS_PATH}/${NOW_DIR}_${NOW_UNIX}"
mkdir -p "$INVOCATION_DIR"

gather_git_info "$CALL_PWD"

mapfile -t cmd_parts < <(build_agent_command "$PRESET" "$MODE" "$CALL_PWD" "$RECORDS_PATH" "$@")
[[ ${#cmd_parts[@]} -eq 0 ]] && exit 1

AGENT_CMD="${cmd_parts[0]}"
AGENT_ARGS=("${cmd_parts[@]:1}")

START_TIME=$(date +%s)

AGENT_EXIT=0
"$AGENT_CMD" "${AGENT_ARGS[@]}" 2>&1 | tee "$INVOCATION_DIR/raw_output.txt"
AGENT_EXIT="${PIPESTATUS[0]}"

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
DURATION_STR=$(format_duration "$DURATION")

cat > "$INVOCATION_DIR/metadata.txt" <<EOF
Date/Time:  $NOW_HUMAN
Agent:      $PRESET ($AGENT_CMD)
Mode:       $MODE
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
