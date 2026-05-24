# Slopspace Adventures

This file is a conversation about upgrade being performed on our slopspaces.

## Step 1 - Introduce the 'sync' concept

```prompt

We want slopspaces (in AI-evo1) to now support a concept of 'syncing'. This is the process by which slopspaces 'expose their potential' to the outside world. In the case of a repo read or writespace this means pushing and pulling to/from the remote. In the case of a dtt read or writespace this means doing a read on any resources referenced (ones for a query or ones that are about to be written) and projecting what the changes would be to arrive at the outcome state. In each case the point is that the content is brought to a state where it is compared against the outside world so the outcome can be predicted in the case of acceptance (conceptually similar to a 'terraform plan').

Syncing in slopspaces nomrally occurs at two points: immediately before a deploy occurs, and immediately after a return occurs. Slopspaces have a 'sync mode' -- no new arguments are required compared to our previous increment, and if the optional 'sync mode' argument is not provided then this mode will default to 'auto-sync'. In this case a sync occurs after the deploy is called before the deploy happens, and also after return is called, after the return occurs but before the overall return process ends.

The sync mode of a slopspace is inherited by all writespaces in that slopspace. (In the future we may have individual sync modes but for now we'll stick with the single slopspace level)

In the future slopspaces will sync in order to make themselves ready to propose their changes. In order to propose the change they will use their 'interaction surface'. Different writespaces allow different interaction surfaces - the two types we will support in our initial implementation are the 'filesystem' and 'github PR'. We will worry about these implementations later - they will be outside the view of dungeon-keeper but will be within the view of claudomation.

For our initial increment we will implement only 'sync' functionality, with only the 'auto-sync' mode, and only for repo-type read and writespaces. We will plumb this functionality into dungeok-keeper, and it should come into claudomation for free and should not impose additional baggage.

Respond first with a proposed implementation with all gaps filled in by assumptions (which are stated), and then respond with a series of clarifying questions so we can refine this implementation. 

Assume we want to support all existing content with only minor change to behaviour and that we will also create a new example for claudomation and new distinct ridealongs for both dungeon-keeper and claudomation to explore this new functionality.

```

## Step 2 - Introduce the 'assignment' concept along with approvals and writes

```prompt

Now we need to think about how the process of a slopspace syncing and 'showing the potential of its changes on the outside world' flows into the concept of 'assignments' and the process of 'approval' and/or local and/or remote 'applying' of writes.

An 'assignment' is the high level encapsulation of a sequence of operations managed through slopspaces. Each assignment is broken down into 'phases', which represent distinct sub-goals of the overall operation, which are in turn broken down into 'steps' which represent the smallest pieces of work which are worth encapsulating. Phases and steps progress linearly (we can assume serially now but we will have parallel capbilities in the future) and when we complete one we cannot go back. A step is not necessarily a single invocation and may entail multiple 'retry' or 'revise' invocations.

(A 'revise' is another invocation stacked on top which maintains the previously completed work. A 'retry' discards the previous work but keeps it visible to the agent working on the next attempt for reference.)

We'll describe hypothetical scenarios for reference as we model the next increments of development:


1. A PR is raised by claudomation after an AI performs an increment of work on the single repo writespace. The PR is raised on the target repo itself. The github user merges the PR. This is considered an externally applied write as well as an approval of the work.

This was an assignment with the shorthand 'branch isolation flow'.
- It has a repo as a target which becomes a writespace
- It has one 'step' to execute a prompt on that target repo
  - We can have the option of 'revise' requests or 'retry' requets
  - Should we present 'retry' commits in a readspace?
- Assume we do not allow other writespaces in our simplest case
- Assume we will start with in-repo PR interaction surface
- Assume we will also allow filesystem interaction surface
- Assignment completes on the single 'step' completion
- We do not need sloppo (an AI tracker repo) for this
- Further considerations:
  - If we used the filesystem interaction surface to approve the proposed changes then our PR would be merged by claudomation the next time we did a 'terraform apply' (approval followed by internal apply)


2. A PR is raised in the sloppo repo after an AI performs an increment of work that it intends to write as content to a new github issue. The github user merges the PR - this signals the write to occur. This is considered an approval of the work followed by an internally applied write.

This is a new type of assignment we can shorthand as a 'simple dtt write flow'.
- It has one writespace dtt canvas in this case (but probably could have many)
- It has one 'step' to execute a prompt and create as-code changes to be written to the dtt target
  - We can support 'revise' and 'retry' here as well
- Assume we will use sloppo PR interaction surface
- Assume we will also allow filesystem interaction surface
- Assignment completes on the single 'step' completion
- Further considerations:
  - If we used the filesystem interaction surface to approve the proposed changes then our PR would be merged by claudomation the next time we did a 'terraform apply' (approval followed by internal apply)


3. A PR is raised in the sloppo repo after an AI performs an increment of work that declaratively describes the shape that this assignment will take. This heuristic request will spell out the type of flow to be used for the remainder of the assignment - at this point we will constrain it to one of the aforementioned types. The github user merges the sloppo PR to signal that the proposed assignment continuation proposal is accepted. The declared assignment details contain the writespace and other configuration details, once accepted this proposal will result in the slopspace being reconfigured and the assignment execution to continue. The remaining steps will be akin to what we saw in (1.) and (2.).

This was an assignment with the shorthand 'heuristic request to <x-flow>'.
- The heuristic request 'phase' has a write surface for declaring the assignment details
- The heuristic request (HR) 'phase' may be subject to 'revise' or 'retry' increments as well
- Assume we will use sloppo PR interaction surface for the HR 'phase'
- Assume we will also allow filesystem interaction surface
- The HR 'phase' completes on the single 'step' completion
- The second phase will be basically identical to the single phase in (1.) or (2.)

We will assume that the knowledge of these interactions is distributed as follows:
- dungeon-keeper (and the harnessed agent) only knows about slopspaces and the context of using declarative tools to create effects on the world. Does not *need to* know about its work in the context of an assignment. The work is always handed down in a focused way. Sometimes dungeon-keeper's invocation goals involve crafting new steps, phases, or assignments - in which case it will have the context it needs then -- but only in these cases.
- claudomation has full knowledge of how the high level structures of assignments, phases, and steps translate into the low level details of slopspaces, sessions, and executions. Although claudomation has no continuous behaviour (it is always externally invoked) it is able to complete all operations if it is repeatedly invoked - even a 'while true; terraform apply --auto-approve' would bring all activity to conclusion.
- agent-dispatch has awareness over the high level structures of assignments, phases, and steps, and has the ability to observe signals that indicate it is time to trigger claudomation and have assignments progress. It does not reach into assignments and modify them directly, it always behaves as an orchestrator.

Let's start by crating a plan for an incremental implementation which covers:
- The minimum amount of content necessary to create a 'step' object in a new 'assignment' claudomation module and example
  - The assignment should not be integrated with execution YET - we'll put a fake stub in place instead of the execution for now
  - We won't model retry or revise yet
  - We WILL create the initial ledger implementation
  - We WILL create and read our interaction surface (with a sloppo PR first)

Be as minimalist as possible........

--- take 1 ---

Let's start by crating a plan for an incremental implementation which covers:
- Upgrades to dungeon-keeper and claudomation as-needed to accomplish scenario (1.)
  - Modeling of the assignment, phase, and step data structures
  - Modeling of the revise and retry processes
- No creation of agent-dispatch just yet

In order to plan this implementation we will first create a 'Proposed Implementation' section where we create a specific implementation by making as many assumptions (and stating them) as we need to without stopping to ask for further instructions. After the 'Proposed Implementation' section we will follow up with a 'Clarifying Questions' section where we ask as many questions as are needed to iron out details which were uncertain.

```

Questions:
- Should dungeon keeper know about 'the juggling of repos'? (Creation of new temporary repo for isolation, duplicating a repo as part of a flow or creating a new repo) Thinking no, but not highly certain. If we said yes we could keep the process wrapped up more tightly, and if we said no we'd have to have 'special tricks' to reintegrate the commits from an isolation repo. If we say no then we can keep dungeon-keeper simpler though, and we'll probably want to be able to extract the diff/commits to work with directly anyhow.

- For (3.) 
  - should we consider using a single sloppo PR for a continued interaction surface between the HR stage and the AW stage? For now let's assume not. Let's assume we have 'phase dividers' as a tool.
  - should we consider switching slopspaces? Probably not at this juncture.
  - is the initial write surface a dtt canvas or is it a special type? Let's assume it's a special type - 'ufa-ops' or similar

# Minimum assignment processing in claudomation

## Step:

What do we need to make an assignment work?
- It needs to know when it executes.
- It needs to know when an execution has been finished (keep this in synchronous blocks locked by state)
  - Needs to know when an execution worked (is this just in the success or failure of the resource, or is the completion of an attempt always success? probably latter)
  - Needs to re-execute at the right time when executions fail
- Needs to put its state into a ledger so it can be viewed/state-retrieved (How does the ledger work?)
  - Records IDs and timestamps in a latching way for work completed
  - Idempotent
  - Contains metadata, no work output
  - (Do we need timeouts?)
  - (Do we want to state start and completion times?)
  - We want to reflect everything needed for importing an ongoing assignment in the ledger
  - (Will we use a file and/or github file resource?)
- Needs to see input from the interaction surface
  - (Do we reflect on other interaction surfaces? Assume yes, like read-replicas)
- Needs to know when approved
- Needs to know when rejected

- Don't need revise/retry but when we do...
  - We need to keep track of when something has been 'tried' (unambiguously) so that it can be 'retried'/'revised'
