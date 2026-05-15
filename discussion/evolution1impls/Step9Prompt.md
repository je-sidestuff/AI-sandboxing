# Prompt

Now that we have slopspaces working in 'dungeon-keeper' in place we want to add minimal support for branch-isolation flows. We will update 'research/AI-evo1/claudomation' with this functionality. We want to follow a similar high level model to the legacy 'research/AI-sandboxing/agent-dispatch' but with significant changes in implementation.

Compared to the legacy implementation the following will be similar:
- Terraform modules and examples will power the backbone of flow state management and execution triggers
- We will scan repos that may or may not exist for (and potentially branches/PRs/comments that may or may not exist)
- We will trigger executions based on PR comments

Compared to the legacy implementation the following will be different:
- The terraform orchestrated components will manage creation of slopsspaces, readspaces, and writespaces
- For the dispatch interaction (which we will create later, not during the claudomation steps) we will always keep things within the broader context of an assignment
- We will now maintain an irreversible 'ledger' or interactions executed
  - The ledger may be in filesystem or gitfile medium
- We will call the golang content, sometimes wrapped with python, but will avoid bash

Initial iteration - we will use details from this description throughout the boostrabbing but will only complete a minimal increment here. We're going to add functionality to 'research/AI-evo1/claudomation/{modules,examples}/execution' such that:
- In our module we have a variable execution_details, an object containing:
  - slopspaces_dir
  - work_signal_dir
  - slopspace_id
  - prompt
  - agent
  - model
  - (we will have more variables soon, but for now let's see if we can get away with just this one)
- In our module we will have a terraform_data resource which:
  - Uses python and performs a similar act to the 'AI-sandboxing/agent-dispatch/modules/execution/single/execute_single_step.sh' in terms of the *execution* itself but does not interact with git
  - Checks to verify our slopspace is not deployed, deploys it, sends the work signal, and waits (up to 20 minutes) to collect the work
- In our example we:
  - leverage the module and we will have sensible defaults for all values
  - create a ridealong where we will create a new slopspace, execute the example, and validate the result
  - make sure the example has documentation on how to rtun it manually without the ridealong
  - the ridealong detects USE_REAL_CLAUDE='true' environment variable and uses claude, otherwise it uses clod

See 'research/terraform-azure-simple-modules' and 'research/terraform-github-orchestration' for style guidance on how to create terraform files. They should have a near-identical look-and-feel.

When this work is complete I should be able to execute the new 'research/AI-evo1/claudomation/examples/execution/ridealong.md' and have things run. We expect the dungeon-keeper is already running in its watch loop waiting for work.


## Reply

All files are written. Here's a summary of what was created/modified in `research/AI-evo1/claudomation`:

**Module (`modules/execution/`):**
- `variables.tf` — single `execution_details` object variable with the 6 required fields
- `main.tf` — `terraform_data` resource with a `local-exec` that runs `execute.py`; passes env vars including `WORK_SIGNALS_DIR` (matching dungeon-keeper's naming) and triggers re-run when `slopspace_id` changes
- `outputs.tf` — `id` output returning the slopspace_id (keeps the existing test passing)
- `versions.tf` — stripped to just `required_version >= 1.0` (no null provider needed since `terraform_data` is built-in)
- `execute.py` — Python script that: (1) reads `SLOPSPACE.json` and verifies not deployed, (2) calls `dungeon-keeper slopspace deploy` via subprocess, (3) writes a JSONL work signal to `ongoing/`, (4) polls `complete/` for up to 20 minutes

**Example (`examples/execution/`):**
- `variables.tf` — `slopspace_id` required; all others (`slopspaces_dir`, `work_signal_dir`, `prompt`, `agent`, `model`) have sensible defaults defaulting to `clod`
- `main.tf` — passes all vars through to `execution_details`
- `outputs.tf` — unchanged (still `result = module.execution`)
- `ridealong.md` — step-by-step Python code blocks: init terraform, create slopspace, apply, validate (checks slopspace returned), optional cleanup; also includes a "Running Without the Ridealong" manual section


# Reprompt A

Please adjust the ridealong - it was not clear in the initial prompt, but ALL EXECUTABLE CONTENT IN A RIDEALONG MUST BE BASH.

When we talked about using only python we were specifically talking about content that would be executed by the terraform modules.

Please also ensure our .gitignore in 'research/AI-evo1' or subdirs ensures no terraform state is committed.

Ensure the default prompt in the example contains our special string to get a write from 'clod'. Remember we can add more interesting text AFTER that if we want a real agent to do something more interesting.
