#!/usr/bin/env bash
#
# agent-report.sh - Generate summary reports of agent usage
#
# Uses invoke-agent.sh to instruct an AI agent to analyze records
# and create a summary report for a given day.
#

set -uo pipefail

# ============================================================================
# Configuration
# ============================================================================

RECORDS_PATH="${AGENT_RECORDS_PATH:-/workspaces/agent-records/}"
AGENT_REPORT_MODE="${AGENT_REPORT_MODE:-DAILY}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INVOKE_AGENT="${SCRIPT_DIR}/invoke-agent.sh"

# ============================================================================
# Functions
# ============================================================================

usage() {
    cat >&2 <<EOF
Usage: $(basename "$0") [-d DATE] [-a AGENT] [-m MODE] [-c "CUSTOM PROMPT"]

Generate a summary report of agent usage.

Options:
  -d DATE    Date to report on (YYYY-MM-DD format, default: today)
  -a AGENT   Agent preset to use for generating report (default: claude)
  -m MODE    Report mode: DAILY, WEEKLY, MONTHLY, CUSTOM (default: \$AGENT_REPORT_MODE or DAILY)
  -c PROMPT  Custom report prompt (implies -m CUSTOM). Use this to request specific analyses.
  -h         Show this help message

Environment:
  AGENT_RECORDS_PATH   Directory for records storage (default: $RECORDS_PATH)
  AGENT_REPORT_MODE    Default report mode (default: DAILY)

Examples:
  $(basename "$0")                      # Generate today's daily report
  $(basename "$0") -d 2026-03-12        # Generate report for specific date
  $(basename "$0") -m WEEKLY            # Generate weekly report
  $(basename "$0") -a gemini -d 2026-03-10  # Use gemini for March 10th report
  $(basename "$0") -c "Analyze programming languages used in recent work"
  $(basename "$0") -c "What Terraform resources have we been working with?"

EOF
    exit 1
}

get_date_range() {
    local mode="$1"
    local target_date="$2"

    case "$mode" in
        DAILY)
            START_DATE="$target_date"
            END_DATE="$target_date"
            PERIOD_DESC="$target_date"
            ;;
        WEEKLY)
            # Get the Monday of the week containing target_date
            local dow
            dow=$(date -d "$target_date" +%u)
            START_DATE=$(date -d "$target_date - $((dow - 1)) days" +%Y-%m-%d)
            END_DATE=$(date -d "$START_DATE + 6 days" +%Y-%m-%d)
            PERIOD_DESC="Week of $START_DATE"
            ;;
        MONTHLY)
            START_DATE=$(date -d "$target_date" +%Y-%m-01)
            END_DATE=$(date -d "$START_DATE + 1 month - 1 day" +%Y-%m-%d)
            PERIOD_DESC=$(date -d "$target_date" +"%B %Y")
            ;;
        CUSTOM)
            # Custom reports analyze all available records by default
            # Use a wide date range to capture everything
            START_DATE="2020-01-01"
            END_DATE=$(date +%Y-%m-%d)
            PERIOD_DESC="Custom analysis"
            ;;
        *)
            echo "Error: Unknown report mode '$mode'" >&2
            exit 1
            ;;
    esac
}

find_records_for_period() {
    local start="$1"
    local end="$2"
    local records_dir="$3"

    local found_records=()

    # Convert dates to comparable format
    local start_cmp="${start//-/}"
    local end_cmp="${end//-/}"

    for dir in "$records_dir"/*/; do
        [[ -d "$dir" ]] || continue

        local dirname
        dirname=$(basename "$dir")

        # Extract date from directory name (format: YYYY-MM-DD_HH-MM-SS_timestamp or session-YYYY-MM-DD_...)
        local dir_date
        if [[ "$dirname" =~ ^session- ]]; then
            dir_date=$(echo "$dirname" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' | head -1)
        else
            dir_date=$(echo "$dirname" | grep -oE '^[0-9]{4}-[0-9]{2}-[0-9]{2}')
        fi

        [[ -z "$dir_date" ]] && continue

        local dir_cmp="${dir_date//-/}"

        # Check if date falls within range
        if (( dir_cmp >= start_cmp && dir_cmp <= end_cmp )); then
            found_records+=("$dirname")
        fi
    done

    printf '%s\n' "${found_records[@]}"
}

# ============================================================================
# Main
# ============================================================================

TARGET_DATE=$(date +%Y-%m-%d)
AGENT_PRESET="claude"
REPORT_MODE="$AGENT_REPORT_MODE"
CUSTOM_PROMPT=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -d)
            [[ $# -lt 2 ]] && usage
            TARGET_DATE="$2"
            shift 2
            ;;
        -a)
            [[ $# -lt 2 ]] && usage
            AGENT_PRESET="$2"
            shift 2
            ;;
        -m)
            [[ $# -lt 2 ]] && usage
            REPORT_MODE="$2"
            shift 2
            ;;
        -c)
            [[ $# -lt 2 ]] && usage
            CUSTOM_PROMPT="$2"
            REPORT_MODE="CUSTOM"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Error: Unknown option '$1'" >&2
            usage
            ;;
    esac
done

# Validate custom mode has a prompt
if [[ "$REPORT_MODE" == "CUSTOM" && -z "$CUSTOM_PROMPT" ]]; then
    echo "Error: CUSTOM mode requires a prompt via -c flag" >&2
    exit 1
fi

# Validate date format
if ! date -d "$TARGET_DATE" &>/dev/null; then
    echo "Error: Invalid date format '$TARGET_DATE'. Use YYYY-MM-DD." >&2
    exit 1
fi

# Validate invoke-agent.sh exists
if [[ ! -x "$INVOKE_AGENT" ]]; then
    echo "Error: invoke-agent.sh not found or not executable at '$INVOKE_AGENT'" >&2
    exit 1
fi

# Calculate date range based on mode
get_date_range "$REPORT_MODE" "$TARGET_DATE"

# Create reports directory
REPORTS_DIR="${RECORDS_PATH%/}/reports"
mkdir -p "$REPORTS_DIR"

# Find records for the period
mapfile -t PERIOD_RECORDS < <(find_records_for_period "$START_DATE" "$END_DATE" "$RECORDS_PATH")

if [[ ${#PERIOD_RECORDS[@]} -eq 0 ]]; then
    echo "No agent records found for $PERIOD_DESC ($START_DATE to $END_DATE)" >&2
    exit 0
fi

# Generate report filename (use uppercase mode as stored)
if [[ "$REPORT_MODE" == "CUSTOM" ]]; then
    # Custom reports use timestamp-based naming
    TIMESTAMP=$(date +%Y-%m-%d_%H-%M-%S)
    BASE_REPORT_FILE="${REPORTS_DIR}/report_CUSTOM_${TIMESTAMP}.md"
    REPORT_FILE="$BASE_REPORT_FILE"
else
    BASE_REPORT_FILE="${REPORTS_DIR}/report_${REPORT_MODE}_${TARGET_DATE}.md"

    # Check if report already exists and find next revision number
    if [[ -f "$BASE_REPORT_FILE" ]]; then
        rev=2
        while [[ -f "${REPORTS_DIR}/report_${REPORT_MODE}_${TARGET_DATE}-rev${rev}.md" ]]; do
            ((rev++))
        done
        REPORT_FILE="${REPORTS_DIR}/report_${REPORT_MODE}_${TARGET_DATE}-rev${rev}.md"
    else
        REPORT_FILE="$BASE_REPORT_FILE"
    fi
fi

# Build the prompt for the agent
RECORDS_LIST=$(printf '  - %s\n' "${PERIOD_RECORDS[@]}")

# Context preamble for all reports (similar to agent-worker)
CONTEXT_PREAMBLE="You are generating a report based on agent work records and history.

## Context Available to You

You have access to agent records that document previous work sessions. These records can be found in the records directory ($RECORDS_PATH) and include:
- Session directories containing metadata.txt and raw_output.txt files
- Worker records: JSON files documenting completed work units, including timestamps, agents used, and exit codes
- Session transcripts: Previous agent conversations and their outputs
- Output artifacts: Files and documents created during previous work sessions

## Available Records for Analysis
$RECORDS_LIST

## How to Use This Context

1. **Read the metadata.txt files** in each directory to understand what work was done
2. **Examine raw_output.txt files** to see the actual agent responses and outputs
3. **Synthesize information** from multiple sources to create a comprehensive report
4. **Focus on the specific request** outlined below

"

if [[ "$REPORT_MODE" == "CUSTOM" ]]; then
    # Custom report uses the user-provided prompt
    PROMPT="${CONTEXT_PREAMBLE}## Your Task

$CUSTOM_PROMPT

Create a comprehensive markdown report addressing the above request.
Analyze the records to extract relevant information and insights.

IMPORTANT: You MUST use your file write tool to create the report file. Do NOT just output the report to stdout.
The report file path is: $REPORT_FILE
Use your Write tool (or equivalent file creation tool) to write the markdown content to this exact path.

Format the report in clean markdown with headers, bullet points, and tables where appropriate."
else
    # Standard periodic report
    PROMPT="${CONTEXT_PREAMBLE}## Your Task

Generate a summary report of agent usage for: $PERIOD_DESC

Create a comprehensive markdown report that includes:

1. **Overview**: Total number of invocations, agents used, time period covered
2. **Agent Breakdown**: Which agents were used and how many times each
3. **Usage Patterns**:
   - Common prompts/tasks
   - Execution modes (-p vs -e)
   - Session vs standalone invocations
4. **Git Activity**: Repositories and branches that were worked on
5. **Performance Summary**:
   - Average/total duration of invocations
   - Success/failure rates (based on exit codes)
6. **Notable Sessions**: Any interesting patterns or highlights
7. **Recommendations**: Suggestions for improving agent workflow based on usage patterns

Read the metadata.txt files in each directory to gather this information.
Also examine raw_output.txt files for context where helpful.

IMPORTANT: You MUST use your file write tool to create the report file. Do NOT just output the report to stdout.
The report file path is: $REPORT_FILE
Use your Write tool (or equivalent file creation tool) to write the markdown content to this exact path.

Format the report in clean markdown with headers, bullet points, and tables where appropriate."
fi

echo "Generating $REPORT_MODE report for $PERIOD_DESC..."
echo "Found ${#PERIOD_RECORDS[@]} record(s) to analyze"
echo "Report will be saved to: $REPORT_FILE"
echo ""

# Invoke the agent to generate the report
MAX_ATTEMPTS=2
ATTEMPT=1

while [[ $ATTEMPT -le $MAX_ATTEMPTS ]]; do
    if [[ $ATTEMPT -gt 1 ]]; then
        echo ""
        echo "Attempt $ATTEMPT of $MAX_ATTEMPTS: Retrying with more explicit instructions..."
    fi

    "$INVOKE_AGENT" -e -a "$AGENT_PRESET" "$PROMPT"

    # Check if report was created
    if [[ -f "$REPORT_FILE" ]]; then
        echo ""
        echo "Report generated successfully: $REPORT_FILE"
        break
    fi

    if [[ $ATTEMPT -lt $MAX_ATTEMPTS ]]; then
        echo ""
        echo "Warning: Report file was not created. Retrying..." >&2
        # Make the prompt even more explicit for retry
        if [[ "$REPORT_MODE" == "CUSTOM" ]]; then
            PROMPT="CRITICAL: The previous attempt failed to create the report file.

You MUST use your Write tool to create this file: $REPORT_FILE

Do NOT output the report content to stdout. Use your file writing capability.

Your task: $CUSTOM_PROMPT

Analyze records in: $RECORDS_PATH
Directories: $RECORDS_LIST

Read metadata.txt and raw_output.txt files from each directory.
Write the markdown report to: $REPORT_FILE"
        else
            PROMPT="CRITICAL: The previous attempt failed to create the report file.

You MUST use your Write tool to create this file: $REPORT_FILE

Do NOT output the report content to stdout. Use your file writing capability.

Generate a summary report of agent usage for: $PERIOD_DESC
Analyze records in: $RECORDS_PATH
Directories: $RECORDS_LIST

Include: overview, agent breakdown, usage patterns, git activity, performance summary, notable sessions, recommendations.

Read metadata.txt and raw_output.txt files from each directory.
Write the markdown report to: $REPORT_FILE"
        fi
    fi

    ((ATTEMPT++))
done

if [[ ! -f "$REPORT_FILE" ]]; then
    echo ""
    echo "Error: Report file was not created after $MAX_ATTEMPTS attempts: $REPORT_FILE" >&2
    echo "The agent may not support file creation or encountered an issue." >&2
    exit 1
fi
