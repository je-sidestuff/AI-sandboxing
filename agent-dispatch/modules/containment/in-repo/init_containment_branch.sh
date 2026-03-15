#!/bin/bash

# Description: This script creates an in-repo containment environment for AI processing.
#              It clones a repository, creates a branch for AI work, separates the .git
#              state for security, and places the work unit in the SLOPSPACES input
#              directory for the agent-worker to process.
#
# Required environment variables:
#   RANDOM_SUFFIX        - A random string to ensure unique directory names
#   SLOPSPACES_WORK_DIR  - The SLOPSPACES working directory (e.g., /workspaces/slopspaces/working/)
#   SOURCE_REPO_URL      - The authenticated HTTPS URL of the source repository to clone
#   DISPATCHER_NAME      - The name of the dispatcher (used for branch naming)

set -e

# Directory structure:
#   OUTER_DIR (containment folder in SLOPSPACES working):
#     - git_state/.git    (isolated git metadata, kept away from agent)
#     - branch_name       (file containing the branch name for later reference)
#   WORKER_DIR (in SLOPSPACES input for agent-worker pickup):
#     - <repo contents>   (working copy without .git)
#     - INSTRUCTION.md    (instructions for the agent)

OUTER_DIR="${SLOPSPACES_WORK_DIR}/containment_${DISPATCHER_NAME}_${RANDOM_SUFFIX}"
WORKER_DIR="/workspaces/slopspaces/input/any/containment_${DISPATCHER_NAME}_${RANDOM_SUFFIX}"
BRANCH_NAME="dispatch-${DISPATCHER_NAME}-${RANDOM_SUFFIX}"

echo "========================================"
echo "In-Repo Containment Setup"
echo "========================================"
echo "Dispatcher: ${DISPATCHER_NAME}"
echo "Random suffix: ${RANDOM_SUFFIX}"
echo "Outer directory: ${OUTER_DIR}"
echo "Worker directory: ${WORKER_DIR}"
echo "Branch name: ${BRANCH_NAME}"
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

# Step 2: Clone the repository into the outer folder
echo ""
echo "Step 2: Cloning repository..."
echo "Source: $SOURCE_REPO_URL"
echo "Sleeping for 5 seconds to allow for any potential setup delays..."
sleep 5
git clone "$SOURCE_REPO_URL" "$OUTER_DIR/repo"
if [ $? -ne 0 ]; then
    clean_up_and_report_failure "Failed to clone repository"
fi
echo "Cloned repository to: ${OUTER_DIR}/repo"

# Step 3: Create and checkout the work branch
echo ""
echo "Step 3: Creating work branch..."
cd "$OUTER_DIR/repo" || clean_up_and_report_failure "Failed to change directory to cloned repository"
git switch -c "$BRANCH_NAME"
echo "Created branch: ${BRANCH_NAME}"

# Save the branch name to the outer directory for later reference
echo "$BRANCH_NAME" > "${OUTER_DIR}/branch_name"

# Step 4: Extract .git and hold it in the outer folder (security isolation)
echo ""
echo "Step 4: Isolating git state..."
mv "$OUTER_DIR/repo/.git" "${OUTER_DIR}/git_state/.git"
echo "Moved .git to: ${OUTER_DIR}/git_state/.git"

# Step 5: Create the INSTRUCTION file in the repo
echo ""
echo "Step 5: Creating INSTRUCTION file..."
cat > "$OUTER_DIR/repo/INSTRUCTION.md" << 'INSTRUCTION_EOF'
You are working on a contained branch for testing AI code generation.

Please make meaningful improvements to this repository. Some suggestions:
- Add useful documentation
- Improve code quality
- Add helpful comments
- Fix any obvious issues

When you are done, save your changes. The containment system will handle committing and pushing.
INSTRUCTION_EOF
echo "Created: ${OUTER_DIR}/repo/INSTRUCTION.md"

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

# --- TEMPORARY EXIT FOR VERIFICATION ---
echo ""
echo "Exiting early for manual verification."
exit 0
# --- END TEMPORARY EXIT ---

# TODO: The following code handles post-processing after the agent completes.
# This should be triggered separately, possibly by a watcher or the worker itself.

# Wait for up to 10 minutes for the AI to process the work unit
# The agent-worker moves completed work to /workspaces/slopspaces/output/
OUTPUT_DIR="/workspaces/slopspaces/output/containment_${DISPATCHER_NAME}_${RANDOM_SUFFIX}"
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
