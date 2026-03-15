#!/bin/bash

# Description: This script clones a Git repository from an HTTPS URL provided
#              via the REPO_HTTPS_CLONE_URL environment variable, then copies
#              the content of a frame directory into the cloned repository and pushes.

TMP_DIR="/tmp/git_repo_apply_frames_$RANDOM_SUFFIX"
CONTAINMENT_DIR="${SLOPSPACES_WORK_DIR}/containment_$RANDOM_SUFFIX"

echo "Calling ${0} with action ${ACTION} and random suffix ${RANDOM_SUFFIX}"
echo "Using SLOPSPACES_WORK_DIR: ${SLOPSPACES_WORK_DIR}"
echo "Containment directory: ${CONTAINMENT_DIR}"

clean_up_and_report_failure() {
    rm -rf "$TMP_DIR"
    echo "$1"
    exit 1
}

if [ "$ACTION" == "create" ]; then
  # Create a temporary directory
  mkdir -p "$TMP_DIR"
  if [ ! -d "$TMP_DIR" ]; then
    echo "Failed to create temporary directory $TMP_DIR."
    exit 1
  fi

  # Clone the repository
  echo "Cloning repository from $REPO_HTTPS_CLONE_URL into $TMP_DIR/repo"
  echo "Sleeping for 5 seconds to allow for any potential setup delays..."
  sleep 5
  git clone "$REPO_HTTPS_CLONE_URL" "$TMP_DIR/repo"
  if [ $? -ne 0 ]; then
    clean_up_and_report_failure "Failed to clone repository"
  fi

  # Clone the source repository
  echo "Cloning repository from $REPO_HTTPS_CLONE_URL into $TMP_DIR/source-repo"
  echo "Sleeping for 5 seconds to allow for any potential setup delays..."
  sleep 5
  git clone "$SOURCE_REPO_URL" "$TMP_DIR/source-repo"
  if [ $? -ne 0 ]; then
    clean_up_and_report_failure "Failed to clone repository"
  fi

  # Remove git lineage from source repository
  rm -rf "$TMP_DIR/source-repo/.git"

  # Copy all files from the source repository to the cloned repository (including dotfiles)
  cp -r "$TMP_DIR/source-repo/." "$TMP_DIR/repo/"

elif [ "$ACTION" == "destroy" ]; then
  rm -rf $TMP_DIR
fi

cd "$TMP_DIR/repo" || clean_up_and_report_failure "Failed to change directory to cloned repository"
#gemini --approval-mode auto_edit -p "Create a new hello world application in this repository."

git add --all
git commit -m "Add pre-AI snapshot."
git push
git switch -c "FLAZZERWOOZLE-WAS-HERE"

echo "Add a null resource that lets us know that the FLAZZERWOOZLE was here." > "$TMP_DIR/repo/INSTRUCTION.md"

# Create the containment directory structure
echo "Creating containment directory: ${CONTAINMENT_DIR}"
mkdir -p "${CONTAINMENT_DIR}"
mkdir -p "${CONTAINMENT_DIR}/git_state"

# Move our git state safely to the containment folder
echo "Safely storing .git to ${CONTAINMENT_DIR}/git_state"
mv "$TMP_DIR/repo/.git" "${CONTAINMENT_DIR}/git_state/.git"

# Move the repo working directory to the containment folder
echo "Moving repo to containment directory: ${CONTAINMENT_DIR}/repo"
mv "$TMP_DIR/repo" "${CONTAINMENT_DIR}/repo"

# --- TEMPORARY EXIT FOR VERIFICATION ---
echo ""
echo "========================================"
echo "VERIFICATION CHECKPOINT"
echo "========================================"
echo "Containment folder created at: ${CONTAINMENT_DIR}"
echo "  - Git state stored at: ${CONTAINMENT_DIR}/git_state/.git"
echo "  - Repo working copy at: ${CONTAINMENT_DIR}/repo"
echo ""
echo "Please verify the structure and contents before continuing."
echo "Exiting early for manual verification."
exit 0
# --- END TEMPORARY EXIT ---

# Wait for up to 10 minutes for the AI to put the completed work in slopspaces/processed
echo "Waiting for the AI to process the repository and place the result in ${CONTAINMENT_DIR}/processed/repo..."
TIMEOUT=600 # 10 minutes in seconds
ELAPSED=0
while [ ! -d "${CONTAINMENT_DIR}/processed/repo" ]; do
    sleep 5
    ELAPSED=$((ELAPSED + 5))
    if [ $ELAPSED -ge $TIMEOUT ]; then
        clean_up_and_report_failure "Timed out waiting for the AI to process the repository."
    fi
done

# Move the processed state back to our temporary directory so we can raise a PR
mv "${CONTAINMENT_DIR}/processed/repo" "$TMP_DIR/processed-repo"

# Bring the git state back and push to our branch
mv "${CONTAINMENT_DIR}/git_state/.git" "$TMP_DIR/processed-repo/.git"
cd "$TMP_DIR/processed-repo" || clean_up_and_report_failure "Failed to change directory to processed repository"
git add --all
git commit -m "Add post-AI snapshot."
git push --set-upstream origin "FLAZZERWOOZLE-WAS-HERE"
