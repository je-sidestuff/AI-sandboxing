# Evolution 1

A first-iteration heavy refactoring of the AI-sandboxing capability set.

This project will mainly be deprecated and used for a better-structured reference.

The destination will be primarily the 'AI-evo1' monorepo but will be supplemented by a small number of independent repos.

## Recorded steps

This AI-sandboxing repository will externally track the prompts used to move to the evo1 state in this section.

### Step 1 - Begin with the inner components first, starting with clod - the dummy test double for claude - to be first used by clauditable

```prompt

We are creating a new test-double for claude.

Start in 'research/AI-evo1' and create a sub-project 'clod'. Look at sandbox/AI-sandboxing for guidance on structure of sub-projects. We want something similar for the sub-projects in evo1 but also with a Dockerfile. We can look at productivity/orgonization/Dockerfile for a simple example to use as a baseline there, and we want a makefile target for docker-build like in research/terraform-example-helpers.

The clod sub-project is a go binary that mimics 'claude' (claude code) to stand in as a test-double -- but only for non-interactive prompts.

We want to be able to use very specific strings to trigger actions from clod. In general we use a lorum-ipsum-cat trope to create mock content with clod.

We want the exact strings, embedded anywhere in the prompt, to trigger the expected actions:

'Our nice agent should create the file <full-file-path>' should result in a new text file being created that has a few sentences about cats if the path does not appear to be an image or video. (not implemented stub for now if it is a non-text type)
'Our nice agent should modify the file <full-file-path>' should result in a few sentences being added about cats if the file appears to be a text file. (not implemented stub for now if it is a non-text type)

We will make sure that our makefile can build, docker-build, run (which automatically passes sample args prompting to create cat_story.txt after echoing to the user that it will do this), clean, or test (a simple test will confirm that the file instructed for creation has been created).

We will make sure that the binary is dockerignored.

```

### Step 2 - Move to 'clauditable' next.

```prompt

The next sub-project to build is 'clauditable'. This is the sub-project that will replace parts of agent-recorder and agent-events in 'sandbox/AI-sandboxing'. The project will run with a paradigm similar to the linux 'time' binary. (So we would call 'clauditable claude -p <prompt>' or 'clauditable clod -p <prompt>)

The clauditable sub-project is also a go binary.

The project should have the same 'bootstrapping files' as 'clod' (docker and gitignore for binaries and test files, Makefile with the same targets, an accompanying .github/workflows/ file for CI with the same setup as the one for 'clod', Dockerfile)

The concept of records and reports will be used throughout the projects to refer to directly collected signals as records and significantly processed/composed/adjusted records as reports.

The purpose of the overall record keeping utility is to wrap arbitrary calls - often an agent, but not always, and to organize the records of these calls in a way which is effective for agents and humans to use together. The system, in the longer term, will organize, refine, and augment the collected records.

The 'clauditable' binary will use input from environment variables to drive some known locations like the 'agent records path' in AGENT_RECORDS_PATH, defaulting to '/host-agent-files/agent-records', where records will be sent automatically. The AGENT_SESSION will define the session that the logs will be added to - the current date in the form 2026-12-13 will be used if none is provided, and will automatically update as the day ticks. The AGENT_CONSOLIDATE_RECORDS will be true by default.

For its first functionality 'clauditable' will:
 - Wrap call passed after its binary name, without interfering at all (similar to 'time'), and call the passed call while teeing both input and error stream, and relaying input stream.
 - Record the command
 - Record the length of time the command took
 - Add an event marker and the raw record of the interaction in a session log. This will use a concise simple event schema inspired by the one in 'sandbox/AI-sandboxing'.
 - The raw record will go to <records-path>/<session>/<unix-timestamp>
 - If AGENT_CONSOLIDATE_RECORDS is true the recorder will 'collect' any '<records-path>/<session>/<unix-timestamp>' format logs when the record it writes is complete, and will append the data from them to '<records-path>/<session>/session.log' before deleting them.
 - The tests should be in go and we want only a very basic couple to begin.

In general this is a spiritual successor to the records functionality found in 'sandbox/AI-sandboxing', so feel free to use patterns there to eliminate ambiguities that arise.

```

### Step 3 - Move to 'ambiguous-agent' next.

```prompt

Next we will create 'research/AI-evo1/ambiguous-agent'.

The 'ambiguous-agent' sub-project is a go binary that is the spiritual successor to 'sandbox/AI-sandboxing/ambiguous-agent/invoke-agent.sh'. It is NOT responsible to cover any interactive-shell-like-CLI any longer (like the old 'sandbox/AI-sandboxing/ambiguous-agent/' go functionality or ambiguous-shell).

Ambiguous agent will provide a generic interface for a call to be made to an agent without knowing which agent/model type will fulfill it. It will be session-aware and will always wrap calls with 'clauditable'. It will optionally provide access to agent records for one or more sessions with 'add dir' style functionality. Unlike the previous implementation which accepted flags p/e for prompt/execute - this time we accept p/r/w/x for prompt/read/write/execute. This corresponds to passing config to the underlying agent to (only chat without even reading files/read files only/read and write files/read and write files, and execute commands). If no 'mode' flag is passed it will default to 'read'.

The project should have the same 'bootstrapping files' as 'clod' (docker and gitignore for binaries and test files, Makefile with the same targets, an accompanying .github/workflows/ file for CI with the same setup as the one for 'clod', Dockerfile) In this case we need to consider that our Docker file must include 'clauditable' as it is a runtime dependency.

For this increment we will support agent listing and selection, but we will not yet support models other than the default. We will use visual flare similar to 'sandbox/AI-sandboxing/ambiguous-agent/main.go' to reflect which agent we are using. We will support the same set of agents we supported in 'sandbox/AI-sandboxing'.

The test suite should be very simple to begin and should leverage 'clod' as the harnessed agent.

```

### Step 4 - Move to 'federation-command' next.

```prompt

We will create the 'research/AI-evo1/federation-command' sub-project next.

The 'federation-command' sub-project is a go binary that is the spiritual successor to 'sandbox/AI-sandboxing/ambiguous-agent/ambiguous-shell'. It is responsible for covering the interactive-shell-like-CLI functionality (the old 'sandbox/AI-sandboxing/ambiguous-agent/' go functionality).

Federation command is a CLI with the same appearance as the previous ambiguous-shell and roughly the same functionality. It now wraps all commands with 'clauditable' - setting the agent to 'none' when the human/keyboard driver is using non-agentic commands. We will add a functionality to clauditable where we prevent double-wrapping by setting an environment variable 'IS_CLAUDITABLE' within the scope of clauditable and detecting it on a subsequent invocation so we can prevent double-logging.

For this first increment we will replicate the visual appearance, the agent-selection, and the 'agent' invocation command (now with -p/r/w/x instead of -p/e). We will also add a NOT_YET_IMPLEMENTED.md describing the functions not yet brought over from legacy.

Make sure the visual style is consistent with 'research/AI-evo1/ambiguous-agent'.

The project should have the same 'bootstrapping files' as 'clod' (docker and gitignore for binaries and test files, Makefile with the same targets, an accompanying .github/workflows/ file for CI with the same setup as the one for 'clod', Dockerfile) In this case we need to consider that our Docker file must include 'clauditable' as it is a runtime dependency. We'll want a similar Makefile capability to deploy dependencies locally, like we have in ambiguous-agent.

For our test suite we will simply support a test of a 'version' entrypoint for now, not the interative mode. In the reply please discuss possible options for testing the interactive mode.

```

### Step 5 - Tighten up ring-0 and the outer ring before moving to async functions

```prompt

Now that the ring-0 sub-projects (clod, clauditable, ambiguous-agent) and the first outer-ring (federation-command) are completed in 'research/AI-evo1' - we will tighten up some functionality and add some additional documentation before proceeding to the async functions.

We will add content for a type of documentary called 'tours'. These documents are similar to system smoke check manuals, but are slightly verbose as they explain more of what is happening with each step. The most important part of these docs is the set of steps - each surrounded by triple-backtics - that if executed would run successfully and 'tell a story'. The 'brief tour' is the less verbose tour, it accompanies a much more thorough and slightly permutative alternative. For now the 'brief-tour.md' will link to a 'full-tour.md' that is just an empty stub.

We will create a CI routine that when pushed to release/* all tests will be run for all sub-projects, and a container will be built and pushed to a ghcr (similar to research/terraform-example-helpers). We would expect to be able to run this container and have full federation-command functionality. (Standalone or as a devcontainer base)

We will also add the capability to provide records to an agent for an invocation. We will accomplish this by copying all the files for a session to a tempdir which will be immediately deleted afterward. We'll add-dir or similar to give the agent access to these files. The default session will be selected by default, but zero or more may be specified. The functionality will be present in ambiguous-agent and federation-command. (Core functionality in AA, an ability to call it with a smart interface in FC)

```

potentially soon:
Thinking...
- Conversation files? (Condocs)
- Ride-alongs? 
- Start visualization?
- Start record processing?
- Look at universal syntax for invoking commands from FC?

### Step 6 - Create 'heuristic-agent' next.

Note: See Step6PromptAttempt1Reference.md for a poor first implementation attempt.

```prompt

We will create the 'research/AI-evo1/heuristic-agent' sub-project next.

The 'heuristic-agent' sub-project is a go binary that is the spiritual successor to 'sandbox/AI-sandboxing/agent-worker' and 'sandbox/AI-sandboxing/heuristic-request'. It is responsible for managing asynchronous invocations of our AI agents through ambiguous-agent.

The heuristic-agent now handles the creation of 'slopspaces' as a first-class concern. It has one entrypoint which is a watch-loop, like the legacy agent-worker and heuristic-request, and another to manage read-spaces, write-spaces, and slopspaces.

Compared to the legacy implementation the separation of work and work signals is also more distinct. Work signals will denote the location of work (which may be a free directory on the host or a slopspace. In practice we will only deploy in-place work locally, remote dispatches will always be in slopspaces) as well as the agent's role and prompt. The work signals will be used as a lock file and progress report as well as the instruction itself, but will not contain *content* that is worked on. The heuristic-agent will work with a one-per-host model, but will be scalable via containerization.

The heuristic-agent will be capable of taking the slopspace from the slopspaces directory and deploying it to the '/agent' dir before work and returning it afterward. We can keep sensitive bits like the .git directory in the slopspace to isolate sensitive operation capability.

Both the work signals and the slopspaces directory will be drivable through environment variables and default to different subdirectories of /host-agent-files.

Remember to use the legacy 'sandbox/AI-sandboxing' as a reference to fill in some blanks.

The project should have the same 'bootstrapping files' as other sub-projects like  'clod' (docker and gitignore for binaries and test files, Makefile with the same targets, an accompanying .github/workflows/ file for CI with the same setup as the one for 'clod', Dockerfile) In this case we need to consider that our Docker file must include its runtime dependencies. We'll want a similar Makefile capability to deploy dependencies locally, like we have in ambiguous-agent.

We will follow the same paradigm with making our calls clauditable (unless they already are by virtue of nesting).

If anything is too ambiguous in these instructions please document the decision paths rather than proceeding blindly and we will perform the work across multiple iterations.

```

NOTE: We take a quick break here to switch FC to be a bubbletea based implementation.

We also added a special 'dynapane' dynamic panel which we will use for interactive functionality.

### Step 7 - Make another stop on 'federation-command' to add the first interactive functionality - the 'ridealong'.

```prompt

Next we will add the first interactive functionality to 'research/AI-evo1/federation-command'.

We will add this functionality in a highly decoupled way, minimizing the intertwining in the main code file as much as we can.

The first thing we want to do is add a 'blinker slot' to the far left of our command prompt.

Where our command prompt used to look like this:
[127] [claude] .../AI-evo1/federation-command > 

It should now look like this:
[ ][127] [claude] .../AI-evo1/federation-command > 

The blinker edges should be a light blue (like the info commands displaying local binaries being used).

Before the user has entered any text the blinker should, by default, blink with a hollow grey block at a 'standard cursor frequency'.

If the user presses right arrow or types any characters that persist in the pending command prompt entry then this blinker should become blank and stop blinking.

If the user removes all text from the pending command entry and presses down, backspace, or left, then the blinker should begin blinking again.

If the user presses left while the blinker is blinking with a hollow grey block the cursor will then disappear from the entry prompt and the blinker will blink with a solid grey block. This is called 'blinker select' mode.

The user may exit 'blinker select' mode by pressing right. If any other keys are pressed while in this mode the blinker slot will flash but nothing will happen. This will alert the user to the fact that they are in 'blinker select' mode.

```

### Step 8 - Add the functionality to 'heuristic-agent' to support testing a repo-isolation or branch-isolation workflow

```prompt

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
```
