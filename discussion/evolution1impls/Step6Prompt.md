# Prompt

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


# Reply

## Analysis: Decision Paths for `research/AI-evo1/heuristic-agent`

Based on my review of the prompt, legacy implementations (`agent-worker`, `heuristic-request`), and the AI-evo1 project structure, I've identified several decision paths that need clarification:

---

### **1. Entrypoint Structure**

The prompt mentions **two entrypoints**:
1. A **watch-loop** (like legacy agent-worker/heuristic-request)
2. A **slopspace management** entrypoint (read-spaces, write-spaces, slopspaces)

**Decision needed:** Should these be:
- **Option A**: Two separate binaries in the same sub-project
- **Option B**: A single binary with subcommands (e.g., `heuristic-agent watch`, `heuristic-agent slopspace create/deploy/return`)
- **Option C**: A single binary with flags to select mode

**Recommendation**: Option B - single binary with subcommands. This follows the pattern of `heuristic-request` (which has `--watch` and `--once` flags) while adding explicit slopspace management.

---

### **2. Slopspace Lifecycle**

The prompt states:
> "The heuristic-agent will be capable of taking the slopspace from the slopspaces directory and deploying it to the '/agent' dir before work and returning it afterward."

**Decision needed:** What is the slopspace lifecycle?
- **Creation**: Who/what creates slopspaces? The heuristic-agent directly, or some external process?
- **Structure**: What's inside a slopspace? (Likely: work unit files, optional .git for sensitive ops)
- **Deployment**: Copy to `/agent` or symlink?
- **Return**: Move back to slopspaces dir, or copy? What about cleanup?

**Suggested structure** based on legacy patterns:
```
/host-agent-files/
├── slopspaces/
│   ├── pending/       (slopspaces waiting to be worked on)
│   ├── active/        (slopspaces currently deployed)
│   └── completed/     (slopspaces that finished work)
├── work-signals/
│   └── <signal-id>/
│       └── SIGNAL.json  (work signal defining what to do)
└── agent-records/     (execution records)
```

---

### **3. Work Signal vs Content Separation**

The prompt clarifies:
> "Work signals will denote the location of work (which may be a free directory on the host or a slopspace)... The work signals will be used as a lock file and progress report as well as the instruction itself, but will not contain *content*."

**Proposed SIGNAL.json structure**:
```json
{
  "id": "unique-signal-id",
  "work_location": "/host-agent-files/slopspaces/pending/my-slopspace",
  "work_type": "slopspace",  // or "in-place" for local directories
  "role": "code-implementer",
  "prompt": "Implement the feature described in FEATURE.md",
  "agent": "claude",
  "mode": "execute",
  "status": "pending",  // pending -> processing -> completed/failed
  "created_at": "...",
  "started_at": null,
  "completed_at": null
}
```

**Decision needed:** Should the signal file be updated in-place (like legacy PROCESSING.md pattern) or should status be tracked separately?

---

### **4. Environment Variables and Defaults**

The prompt mentions:
> "Both the work signals and the slopspaces directory will be drivable through environment variables and default to different subdirectories of /host-agent-files."

**Proposed env vars**:
- `SLOPSPACES_DIR` - default: `/host-agent-files/slopspaces`
- `WORK_SIGNALS_DIR` - default: `/host-agent-files/work-signals`
- `AGENT_WORKSPACE_ROOT` - default: `/agent` (where slopspaces are deployed)
- `AGENT_RECORDS_PATH` - default: `/host-agent-files/agent-records`

---

### **5. Clauditable Integration**

The prompt says:
> "We will follow the same paradigm with making our calls clauditable (unless they already are by virtue of nesting)."

The legacy agent-worker uses `agentaudit.Capture()` from `pkg/agentaudit`. For the new project in AI-evo1, should we:
- **Option A**: Import clauditable as a library/package
- **Option B**: Wrap agent invocations with the `clauditable` binary
- **Option C**: Both (library for capture, binary available as fallback)

**Recommendation**: Option C - the clauditable package already exists in AI-evo1 with `pkg/records/`. Use that for recording.

---

### **6. Docker Runtime Dependencies**

The prompt notes:
> "Our Docker file must include its runtime dependencies."

**Likely dependencies**:
- `invoke-agent.sh` (agent invocation script)
- Agent CLIs: `claude`, `copilot`, etc. (or at least the one being used)
- Go runtime for the binary itself
- Potentially git (if .git operations are needed)

**Decision needed:** Should the Dockerfile:
- **Option A**: Include all agent CLIs (large image, self-contained)
- **Option B**: Expect agents to be mounted/available at runtime
- **Option C**: Parameterize which agents to include

---

### **7. One-Per-Host Model with Containerization**

The prompt states:
> "The heuristic-agent will work with a one-per-host model, but will be scalable via containerization."

This implies each container acts as its own "host" with its own `/agent` workspace. The scaling happens by running multiple containers, each appearing as a separate worker.

**Implication**: The Docker image needs:
- Its own `/agent` directory for workspace
- Access to shared volumes for slopspaces and signals (mounted)

---

### **Summary of Questions for Iteration**

Before proceeding, clarification would help on:

1. **Entrypoint design**: Single binary with subcommands? (Recommended)
2. **Slopspace lifecycle details**: Creation, deployment mechanism, return process
3. **Work signal schema**: Exact fields needed, update-in-place or separate status?
4. **Dependency inclusion**: What agents/tools to bundle in Docker image?
5. **Legacy code reuse**: Should we copy/adapt the `pkg/` utilities from `AI-sandboxing` or reimplement fresh?

---

Would you like me to proceed with implementing based on my recommendations, or would you prefer to clarify any of these decision paths first?

Records saved to: /workspaces/agent-records/session-2026-04-29_00-47-14_1777423634/2026-05-02_00-12-25_1777680745


## Reprompt A

We will take the following paths:

### **1. Entrypoint Structure**

Option B.

### **2. Slopspace Lifecycle**

**Decision:** What is the slopspace lifecycle?
- **Creation**: Heurist-agent creates the slopspace directly, from the 'go' portion of the overall runtime, not from within the scope of the agenty invocation.
- **Structure**: Inside a slopspace we find read-spaces and write-spaces. Examine legacy agent-worker for more details.
- **Deployment**: For deployment we MOVE the internals of the slopspace to /agent - not copy or symlink.
- **Return**: Move back to slopspaces dir. We will not be thorough with cleanup on our first implementation, we will handle it subsequently. We'll start by assuming happy path.

**Folder structure**:
```
/host-agent-files/
├── slopspaces/
│   └── <slopspace-id>/
│       ├── read-spaces/       (things the agent should read - if modifiable will not be reflected on the outside world)
│       │   ├── agent-records/ (may contain sessions to read)
│       │   ├── dtt-images/    (may contain read results from declarative-tool-tools to read)
│       │   ├── repos/         (repos which we can see but not impact)
│       │   └── files/         (arbitrary files we can read)
│       └── write-spaces/      (places the agent may write - if modified will be reflected in the outside world)
│           ├── agent-records/ (ALWAYS available for the agent to write)
│           ├── dtt-canvas/    (agent may use declarative-tool-tools to make changes and/or request feedback)

│           ├── repos/         (repos which we can see but not impact)
│           └── files/         (arbitrary files we can read)
├── work/
│   ├── agent/        (slopspaces currently deployed)
│   └── <signal-id>/
│       └── SIGNAL.json  (work signal defining what to do)
└── agent-records/     (execution records)
```

In plain language - the slopspace lifecycle follows the process of:
- Creation of the slopspace -- the slopspace is created in the slopspaces directory with its new unique ID, directories are created
- Read spaces and write spaces are populated in the slop space - usually from the read-spaces and/or write-spaces directory
- The slopspace is deployed to the appropriate directory within /agent tree (the metadata layer and sensitive files remain in slopspaces, the agent-concerning files are moved into place)
- The agent is invoked within the slopspace
- Depending on a variety of factors, the slopspace may be left in-place, or may be returned back to the slopspaces directory (write spaces are moved back, read spaces are discarded and repopulated in slopspace within  slopspaces dir)
- If the slopspace is left in place another agent invocation will eventually ensue
- If the slopspace is returned it will be responded to, and then it will either continue or conclude
- When the slopspace is responded to, deterministic agents will ingest the content and will then execute commands and/or submit proposals for approval
- The response phase will stay open until all pending auto-approved commands have been executed and proposals have been approved/denied, then it will end and the slopspace will continue/conclude
- If the slopspace concludes it will be removed from the directory, having had all actions performed
- If the slopspace continues it will undergo another deployment and begin again at that phase (this may go on for any number of iterations)


