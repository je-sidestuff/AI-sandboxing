#!/bin/bash

# Description: This script sets up an in-repo containment environment for AI processing.
#              It clones a repository at a pre-created branch, separates the .git
#              state for security, and places the work unit in the SLOPSPACES input
#              directory for the agent-worker to process.
#
# Required environment variables:
#   UNIX_TIMESTAMP       - Unix seconds timestamp for unique directory naming
#   BRANCH_NAME          - The dispatch branch name to create in the isolation repo
#   SLOPSPACES_WORK_DIR  - The SLOPSPACES working directory (e.g., /workspaces/slopspaces/working/)
#   SOURCE_REPO_URL      - The authenticated HTTPS URL of the target repository (baseline source)
#   ISOLATION_REPO_URL   - The authenticated HTTPS URL of the isolation repository
#   DISPATCHER_NAME      - The name of the dispatcher (used for directory naming)

set -e

# Directory structure:
#   OUTER_DIR (containment folder in SLOPSPACES working):
#     - git_state/.git    (isolated git metadata, kept away from agent)
#     - branch_name       (file containing the branch name for later reference)
#   WORKER_DIR (in SLOPSPACES input for agent-worker pickup):
#     - <repo contents>   (working copy without .git)
#     - INSTRUCTION.md    (instructions for the agent)

OUTER_DIR="${SLOPSPACES_WORK_DIR}/containment_${DISPATCHER_NAME}_${UNIX_TIMESTAMP}"
WORKER_DIR="/workspaces/slopspaces/input/any/containment_${DISPATCHER_NAME}_${UNIX_TIMESTAMP}"
# BRANCH_NAME is now passed from terraform (already created)

echo "========================================"
echo "In-Repo Containment Setup"
echo "========================================"
echo "Dispatcher: ${DISPATCHER_NAME}"
echo "Unix timestamp: ${UNIX_TIMESTAMP}"
echo "Outer directory: ${OUTER_DIR}"
echo "Worker directory: ${WORKER_DIR}"
echo "Branch name: ${BRANCH_NAME} (created by script in isolation repo)"
echo "========================================"

clean_up_and_report_failure() {
    rm -rf "$OUTER_DIR"
    rm -rf "$WORKER_DIR"
    echo "ERROR: $1"
    exit 1
}

# Step 1: Create the outer working directory in SLOPSPACES
echo ""
echo "Step 1: Creating outer working directory..."
mkdir -p "${OUTER_DIR}"
mkdir -p "${OUTER_DIR}/git_state"
if [ ! -d "$OUTER_DIR" ]; then
    echo "Failed to create outer directory $OUTER_DIR."
    exit 1
fi
echo "Created: ${OUTER_DIR}"

# Step 2: Clone the target repo for baseline content
echo ""
echo "Step 2: Cloning target repo for baseline content..."
echo "Source: $SOURCE_REPO_URL"
echo "Sleeping for 5 seconds to allow for any potential setup delays..."
sleep 5
git clone --single-branch "$SOURCE_REPO_URL" "$OUTER_DIR/repo"
if [ $? -ne 0 ]; then
    clean_up_and_report_failure "Failed to clone target repository"
fi
echo "Cloned target repository to: ${OUTER_DIR}/repo"

# Step 3: Push baseline to isolation repo and create the dispatch branch
echo ""
echo "Step 3: Pushing baseline to isolation repo and creating branch ${BRANCH_NAME}..."
cd "$OUTER_DIR/repo" || clean_up_and_report_failure "Failed to change directory to cloned repository"
git remote set-url origin "$ISOLATION_REPO_URL"
git push origin main
git checkout -b "$BRANCH_NAME"
git push origin "$BRANCH_NAME"
echo "Pushed baseline to isolation repo and created branch: ${BRANCH_NAME}"

# Save the branch name to the outer directory for later reference
echo "$BRANCH_NAME" > "${OUTER_DIR}/branch_name"

# Step 4: Extract .git and hold it in the outer folder (security isolation)
echo ""
echo "Step 4: Isolating git state..."
mv "$OUTER_DIR/repo/.git" "${OUTER_DIR}/git_state/.git"
echo "Moved .git to: ${OUTER_DIR}/git_state/.git"

# Step 5: Create the INSTRUCTION file in the repo (using execute mode)
echo ""
echo "Step 5: Creating INSTRUCTION.json file (execute mode)..."
cat > "$OUTER_DIR/repo/INSTRUCTION.json" << 'INSTRUCTION_EOF'
{
  "mode": "execute",
  "instruction": "You are working on a contained branch for testing AI code generation.\n\nPlease make meaningful improvements to this repository. Some suggestions:\n- Add useful documentation\n- Improve code quality\n- Add helpful comments\n- Fix any obvious issues\n\nWhen you are done, save your changes. The containment system will handle committing and pushing."
}
INSTRUCTION_EOF
echo "Created: ${OUTER_DIR}/repo/INSTRUCTION.json"

# Step 6: Move the repo to the worker input directory
echo ""
echo "Step 6: Moving repo to worker directory..."
mkdir -p "$(dirname "$WORKER_DIR")"
mv "$OUTER_DIR/repo" "$WORKER_DIR"
echo "Moved repo to: ${WORKER_DIR}"

# Verification
echo ""
echo "========================================"
echo "CONTAINMENT SETUP COMPLETE"
echo "========================================"
echo "Outer folder (git state): ${OUTER_DIR}"
echo "  - Git state: ${OUTER_DIR}/git_state/.git"
echo "  - Branch name file: ${OUTER_DIR}/branch_name"
echo ""
echo "Worker folder (agent input): ${WORKER_DIR}"
echo "  - Repository contents (without .git)"
echo "  - INSTRUCTION.md"
echo ""
echo "The agent-worker will pick up this work unit from the input directory."
echo "========================================"

# Wait for up to 10 minutes for the AI to process the work unit
# The agent-worker moves completed work to /workspaces/slopspaces/output/
OUTPUT_DIR="/workspaces/slopspaces/output/containment_${DISPATCHER_NAME}_${UNIX_TIMESTAMP}"
echo "Waiting for the AI to process the repository..."
echo "Expected output location: ${OUTPUT_DIR}"
TIMEOUT=600 # 10 minutes in seconds
ELAPSED=0
while [ ! -d "${OUTPUT_DIR}" ]; do
    sleep 5
    ELAPSED=$((ELAPSED + 5))
    if [ $ELAPSED -ge $TIMEOUT ]; then
        clean_up_and_report_failure "Timed out waiting for the AI to process the repository."
    fi
done

# Bring the git state back and push to our branch
echo "Processing complete. Restoring git state and pushing..."
mv "${OUTER_DIR}/git_state/.git" "${OUTPUT_DIR}/.git"
cd "${OUTPUT_DIR}" || clean_up_and_report_failure "Failed to change directory to output repository"
git add --all
git commit -m "AI-generated changes from dispatcher ${DISPATCHER_NAME}"
git push --set-upstream origin "$BRANCH_NAME"

echo "Successfully pushed changes to branch: ${BRANCH_NAME}"

# Cleanup outer directory
rm -rf "${OUTER_DIR}"
echo "Cleaned up outer directory."
