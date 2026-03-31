#!/bin/bash

# Description: This script handles REVISE: comments from a containment PR.
#              For each instruction in REVISE_INSTRUCTIONS_JSON, it clones the
#              repo at the HEAD of the working branch, dispatches an AI work unit
#              with the instruction, waits for completion, and pushes results back
#              to the same branch.
#
# Required environment variables:
#   REVISE_INSTRUCTIONS_JSON - JSON array of instruction strings (from fetch_pr_comments.py)
#   BRANCH_NAME              - The working branch to clone and push to
#   SOURCE_REPO_URL          - The authenticated HTTPS URL of the source repository
#   SLOPSPACES_WORK_DIR      - The SLOPSPACES working directory (e.g., /workspaces/slopspaces/working/)
#   DISPATCHER_NAME          - The name of the dispatcher (used for directory naming)
#   UNIX_TIMESTAMP           - Unix seconds timestamp for unique directory naming

set -e

echo "========================================"
echo "Handle REVISE Comments"
echo "========================================"
echo "Dispatcher:  ${DISPATCHER_NAME}"
echo "Branch:      ${BRANCH_NAME}"
echo "Timestamp:   ${UNIX_TIMESTAMP}"
echo "========================================"

# Parse the number of instructions from the JSON array using python3
INSTRUCTION_COUNT=$(python3 -c "import json,sys; data=json.loads(sys.argv[1]); print(len(data))" "$REVISE_INSTRUCTIONS_JSON")

if [ "$INSTRUCTION_COUNT" -eq 0 ]; then
    echo "No REVISE instructions found. Nothing to do."
    exit 0
fi

echo "Found ${INSTRUCTION_COUNT} REVISE instruction(s). Processing..."

clean_up_and_report_failure() {
    local outer="$1"
    local worker="$2"
    rm -rf "$outer"
    rm -rf "$worker"
    echo "ERROR: $3"
    exit 1
}

# Iterate over each instruction
for i in $(seq 0 $((INSTRUCTION_COUNT - 1))); do
    INSTRUCTION=$(python3 -c "import json,sys; data=json.loads(sys.argv[1]); print(data[int(sys.argv[2])])" "$REVISE_INSTRUCTIONS_JSON" "$i")

    REVISE_ID="revise_${DISPATCHER_NAME}_${UNIX_TIMESTAMP}_${i}"
    OUTER_DIR="${SLOPSPACES_WORK_DIR}/${REVISE_ID}"
    WORKER_DIR="/workspaces/slopspaces/input/any/${REVISE_ID}"
    OUTPUT_DIR="/workspaces/slopspaces/output/content/${REVISE_ID}"

    echo ""
    echo "----------------------------------------"
    echo "Processing REVISE instruction $((i + 1)) of ${INSTRUCTION_COUNT}"
    echo "ID: ${REVISE_ID}"
    echo "Instruction: ${INSTRUCTION}"
    echo "----------------------------------------"

    # Step 1: Create the outer working directory
    echo "Step 1: Creating outer working directory..."
    mkdir -p "${OUTER_DIR}/git_state"

    # Step 2: Clone the repo at the HEAD of the working branch
    echo "Step 2: Cloning repository at branch ${BRANCH_NAME}..."
    echo "Sleeping for 5 seconds to allow for any potential push-propagation delays..."
    sleep 5
    git clone --branch "$BRANCH_NAME" --single-branch "$SOURCE_REPO_URL" "$OUTER_DIR/repo" \
        || clean_up_and_report_failure "$OUTER_DIR" "$WORKER_DIR" "Failed to clone repository at branch ${BRANCH_NAME}"

    # Step 3: Verify branch
    echo "Step 3: Verifying branch..."
    cd "$OUTER_DIR/repo" || clean_up_and_report_failure "$OUTER_DIR" "$WORKER_DIR" "Failed to cd into cloned repo"
    CURRENT_BRANCH=$(git branch --show-current)
    if [ "$CURRENT_BRANCH" != "$BRANCH_NAME" ]; then
        clean_up_and_report_failure "$OUTER_DIR" "$WORKER_DIR" "Expected branch ${BRANCH_NAME} but found ${CURRENT_BRANCH}"
    fi
    echo "Verified on branch: ${BRANCH_NAME}"

    # Save branch name for later reference
    echo "$BRANCH_NAME" > "${OUTER_DIR}/branch_name"

    # Step 4: Isolate .git state
    echo "Step 4: Isolating git state..."
    mv "$OUTER_DIR/repo/.git" "${OUTER_DIR}/git_state/.git"

    # Step 5: Write INSTRUCTION.json with the REVISE prompt
    echo "Step 5: Writing INSTRUCTION.json..."
    python3 - <<PYEOF
import json
instruction = """${INSTRUCTION}"""
payload = {"mode": "execute", "instruction": instruction}
with open("${OUTER_DIR}/repo/INSTRUCTION.json", "w") as f:
    json.dump(payload, f, indent=2)
PYEOF
    echo "Created: ${OUTER_DIR}/repo/INSTRUCTION.json"

    # Step 6: Move repo to slopspaces worker input
    echo "Step 6: Moving repo to worker directory..."
    mkdir -p "$(dirname "$WORKER_DIR")"
    mv "$OUTER_DIR/repo" "$WORKER_DIR"
    echo "Moved to: ${WORKER_DIR}"

    # Step 7: Wait for the AI to process the work unit (up to 10 minutes)
    echo "Step 7: Waiting for AI processing (up to 10 minutes)..."
    TIMEOUT=600
    ELAPSED=0
    while [ ! -d "${OUTPUT_DIR}" ]; do
        sleep 5
        ELAPSED=$((ELAPSED + 5))
        if [ $ELAPSED -ge $TIMEOUT ]; then
            clean_up_and_report_failure "$OUTER_DIR" "$WORKER_DIR" "Timed out waiting for AI to process instruction ${i}"
        fi
    done
    echo "Processing complete for instruction ${i}."

    # Step 8: Restore .git and push changes back to the working branch
    echo "Step 8: Restoring git state and pushing to ${BRANCH_NAME}..."
    mv "${OUTER_DIR}/git_state/.git" "${OUTPUT_DIR}/.git"
    cd "${OUTPUT_DIR}" || clean_up_and_report_failure "$OUTER_DIR" "" "Failed to cd into output directory"

    # Debug: show current git state before any changes
    echo "Current git HEAD before fix:"
    cat .git/HEAD
    echo "Branches available:"
    git branch -a 2>/dev/null || echo "(git branch failed)"

    # After restoring .git, force checkout the target branch to ensure git recognizes it.
    echo "Force checking out branch: ${BRANCH_NAME}..."
    git checkout -f "${BRANCH_NAME}" || {
        echo "Checkout failed, trying alternative approach..."
        git symbolic-ref HEAD "refs/heads/${BRANCH_NAME}"
    }

    echo "Current branch after checkout:"
    git branch --show-current || echo "(no branch)"

    # Reset the index and stage working tree changes
    echo "Resetting git index..."
    git reset --mixed

    git add --all
    git commit -m "REVISE: AI-applied changes from dispatcher ${DISPATCHER_NAME} (instruction $((i + 1)))"
    git push --set-upstream origin "$BRANCH_NAME"
    echo "Pushed changes to branch: ${BRANCH_NAME}"

    # Cleanup
    rm -rf "${OUTER_DIR}"
    echo "Cleaned up outer directory for instruction ${i}."
done

echo ""
echo "========================================"
echo "All REVISE instructions processed successfully."
echo "========================================"
