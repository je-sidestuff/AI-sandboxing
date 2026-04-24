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

The 'ambiguous-agent' sub-project is a go binary that is the spiritual successor to 'sandbox/AI-sandboxing/ambiguous-agent/ambiguous-agent.sh'. It is NOT responsible to cover any interactive-shell-like-CLI any longer (like the old 'sandbox/AI-sandboxing/ambiguous-agent/' go functionality).

Ambiguous agent will provide a generic interface for a call to be made to an agent without knowing which agent/model type will fulfill it. It will be session-aware and will always wrap calls with 'clauditable'. It will optionally provide access to agent records for one or more sessions with 'add dir' style functionality. Unlike the previous implementation which accepted flags p/e for prompt/execute - this time we accept p/r/w/x for prompt/read/write/execute. This corresponds to passing config to the underlying agent to (only chat without even reading files/read files only/read and write files/read and write files, and execute commands)

The project should have the same 'bootstrapping files' as 'clod' (docker and gitignore for binaries and test files, Makefile with the same targets, an accompanying .github/workflows/ file for CI with the same setup as the one for 'clod', Dockerfile) In this case we need to consider that our Docker file must include 'clauditable' as it is a runtime dependency.

```

### Step 4 - Move to 'heuristic-agent' next.

```prompt



```