#!/bin/bash

# Description: This script pushes a branch from the isolation repo to the target repo
#              for re-integration after the isolation PR has been merged.
#
# Required environment variables:
#   BRANCH_NAME         - The branch name to push
#   SOURCE_REPO_URL     - The authenticated HTTPS URL of the isolation repository (source)
#   TARGET_REPO_URL     - The authenticated HTTPS URL of the target repository (destination)
#   SLOPSPACES_WORK_DIR - The SLOPSPACES working directory for temporary clone
#   UNIX_TIMESTAMP      - Unix seconds timestamp for unique directory naming

set -e

WORK_DIR="${SLOPSPACES_WORK_DIR}/reintegration_${UNIX_TIMESTAMP}"

echo "========================================"
echo "Re-integration: Push to Target Repo"
echo "========================================"
echo "Branch: ${BRANCH_NAME}"
echo "Work directory: ${WORK_DIR}"
echo "========================================"

clean_up_and_report_failure() {
    rm -rf "$WORK_DIR"
    echo "ERROR: $1"
    exit 1
}

# Step 1: Create temporary work directory
echo ""
echo "Step 1: Creating temporary work directory..."
mkdir -p "${WORK_DIR}"
if [ ! -d "$WORK_DIR" ]; then
    echo "Failed to create work directory $WORK_DIR."
    exit 1
fi
echo "Created: ${WORK_DIR}"

# Step 2: Clone the branch from the isolation repo
echo ""
echo "Step 2: Cloning branch ${BRANCH_NAME} from isolation repo..."
git clone --single-branch --branch "$BRANCH_NAME" "$SOURCE_REPO_URL" "$WORK_DIR/repo"
if [ $? -ne 0 ]; then
    clean_up_and_report_failure "Failed to clone branch from isolation repository"
fi
echo "Cloned branch to: ${WORK_DIR}/repo"

# Step 3: Push the branch to the target repo
echo ""
echo "Step 3: Pushing branch to target repo..."
cd "$WORK_DIR/repo" || clean_up_and_report_failure "Failed to change directory"
git remote set-url origin "$TARGET_REPO_URL"
git push -u origin "$BRANCH_NAME"
if [ $? -ne 0 ]; then
    clean_up_and_report_failure "Failed to push branch to target repository"
fi
echo "Successfully pushed branch ${BRANCH_NAME} to target repository"

# Cleanup
echo ""
echo "Step 4: Cleaning up..."
rm -rf "$WORK_DIR"
echo "Cleaned up temporary directory."

echo ""
echo "========================================"
echo "RE-INTEGRATION PUSH COMPLETE"
echo "========================================"
echo "Branch ${BRANCH_NAME} is now available in the target repository."
echo "========================================"
