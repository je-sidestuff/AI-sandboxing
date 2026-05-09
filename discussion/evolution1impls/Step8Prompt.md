# Prompt

Now that we have the beginning of 'heuristic-agent' in place we want to add minimal support for branch-isolation flows. We will update 'research/AI-evo1/heuristic-agent' with this functionality.

Unlike our legacy implementation in 'sandbox/AI-sandboxing', we will support the full flow through heuristic-agent functionality with manual user interactions. (heuristic-agent does not automate or respond, it only provides functions to perform the atomic operations needed).

First we will modify all references in AI-evo1 from 'read-spaces' and 'write-spaces' to 'readspaces' and 'writespaces'.

Our repository interactions will all be within 'readspaces' and 'writespaces', the file system areas which we dispatch repository content to.

The lifecycle of a repository readspace is as follows:
- We create a 'repo readspace' with the command 'readspace repo clone <owner>/<repo>'
  - This pulls the readspace into the 'readspaces dir' beside the 'slopspaces dir' (defaults to /host-agent-files/readspaces)
  - We store the repository on the main branch and every time we touch it we will 'pull --rebase' it
  - We always clone the repo in question using the github PAT, which is stored in the TF_VAR_github_pat env var (for global cross-compatibility)
  - We assume 'gh' is available and on the path for our git operations (including clone)
  - The repo is stored unmodified
- We add a repo readspace to a slopspace with the 'slopspace add-readspace repo <slopspace-id> <owner>/<repo>'
  - We may optionally specify a '--ref <branch|tag|commit>' argument
  - When we add the readspace to a slopspace the following takes place:
    - The repo is copied to the slopspace (<slopspace>/readspaces/repos/<owner>/<repo>)
    - Then we switch to the ref in the copied repo
    - We delete the full '.git' directory
- Now we do not care if an agent modifies the files here because they will not be put back anywhere, they are disposable
- We can remove a repo readspace with 'readspace repo delete <owner>/<repo>'

The lifecycle of a repository writespace is as follows:
- We create a 'repo writespace' with the command 'writespace repo clone <owner>/<repo>'
  - This pulls the writespace into the 'writespaces dir' beside the 'slopspaces dir' (defaults to /host-agent-files/writespaces)
  - We store the repository on the main branch and every time we touch it we will 'pull --rebase' it
  - We always clone the repo in question using the github PAT, which is stored in the TF_VAR_github_pat env var (for global cross-compatibility)
  - We assume 'gh' is available and on the path for our git operations
  - The repo is stored unmodified
- We add a repo writespace to a slopspace with the 'slopspace add-writespace repo <slopspace-id> <owner>/<repo>'
  - We may optionally specify a '--ref <branch>' argument (must be a branch for writespace)
  - When we add the writespace to a slopspace the following takes place:
    - The repo is copied to the slopspace (<slopspace>/writespaces/repos/<owner>/<repo>)
    - Then we switch to the ref in the copied repo
    - We move the full '.git' directory to the 'writespaces-secure dir' (<slopspace>/writespaces-secure/repos/<owner>/<repo>) - this will not be copied to '/agent' when the slopspace is deployed
  - When the slopspace is returned we are able to call 'write' with 'slopspace write <slopspace-id> all' or 'slopspace write repo <slopspace-id> <owner>/<repo>' - this will push to the branch

Please also create a dedicated ridealong document to execute this process. Use this repo as the target and clod as the agent. (See federation-command for details on how to create a good ridealong)



# Reply


I have completed the implementation. Here's a summary of what was done:

## Summary of Changes

### 1. Renamed `read-spaces` → `readspaces` and `write-spaces` → `writespaces`

Updated all references in:
- `pkg/slopspace/slopspace.go` - Constants and error messages
- `pkg/slopspace/slopspace_test.go` - Test file comments
- `pkg/executor/executor_test.go` - Test working directory paths
- `docs/brief-tour.md` - Documentation and examples

### 2. Created New Packages

**`pkg/readspace/readspace.go`**
- `Manager` struct with methods for managing repository readspaces
- `CloneRepo(ownerRepo)` - Clone from GitHub using PAT
- `Get(ownerRepo)` - Get metadata for a repo
- `List()` - List all repos
- `Delete(ownerRepo)` - Delete a repo readspace

**`pkg/writespace/writespace.go`**
- Same structure as readspace for managing writespaces

### 3. Extended `pkg/slopspace/slopspace.go`

Added:
- `WriteSpacesSecure` constant for secure `.git` storage
- `AddReadspaceRepo(id, owner, repo, ref, sourcePath)` - Copy repo to slopspace, checkout ref, delete `.git`
- `AddWritespaceRepo(id, owner, repo, ref, sourcePath)` - Copy repo to slopspace, checkout ref, move `.git` to secure location
- `WriteRepoChanges(id, owner, repo, message)` - Restore `.git`, commit, push
- `WriteAllRepoChanges(id, message)` - Push all writespace repos
- Git helper functions: `gitCheckout`, `gitAddAll`, `gitHasChanges`, `gitCommit`, `gitPush`

### 4. Updated `main.go`

Added new CLI commands:
- `readspace repo clone <owner/repo>` - Clone repo to readspaces
- `readspace repo delete <owner/repo>` - Delete from readspaces
- `readspace repo list` - List repos in readspaces
- `writespace repo clone <owner/repo>` - Clone repo to writespaces
- `writespace repo delete <owner/repo>` - Delete from writespaces
- `writespace repo list` - List repos in writespaces
- `slopspace add-readspace repo <id> <owner/repo> [--ref]` - Add repo to slopspace readspaces
- `slopspace add-writespace repo <id> <owner/repo> [--ref]` - Add repo to slopspace writespaces
- `slopspace write <id> all` - Push all writespace repos
- `slopspace write repo <id> <owner/repo>` - Push specific repo

### 5. Created Ridealong Document

**`docs/branch-isolation-ridealong.md`**
- Complete walkthrough of the branch-isolation flow
- Phase 1: Repository Setup (clone to readspaces/writespaces)
- Phase 2: Slopspace Workflow (create, add repos, deploy, work, return)
- Phase 3: Write Changes Back (commit and push)
- Cleanup instructions
- Directory structure diagrams
- Command reference table

Records saved to: /workspaces/agent-records/session-2026-05-08_23-47-41_1778284061/2026-05-08_23-57-49_1778284669
