# Slopspace Adventures

This file is a conversation about 

## Step 1 - Introduce the 'proposals' concept

```prompt

We want slopspaces to now support a concept of 'proposing'. We want to have each slopspace have a 'proposal mode' -- no new arguments are required, and if the optional 'proposal mode' argument is not provided then this mode will default to 'auto-propose'.

The proposal mode of a slopspace is inherited by all writespaces in that slopspace. (In the future we may have individual proposal modes but for now we'll stick with the single slopspace level)

Each type of writespace proposes its changes differently. A writespace repo pushes its changes to its branch as part of the 'sync' phase which happens before proposing but after returning the slopspace. In order to propose the change it will use its 'interaction surface'. Different writespaces allow different interaction surfaces - the two types we will support in our initial implementation are the 'filesystem' and 'github PR'.R'. 

Respond first with a proposed implementation with all gaps filled in by assumptions (which are stated), and then respond with a series of clarifying questions so we can refine this implementation.on. 

Assume we want to support all existing content with only minor change to behaviour and that we will also create a new example for claudomation and new dist
inct ridealongs for both dungeon-keeper and claudomation to explore this new functionality.

```

## Step 1 - Introduce the 'sync' concept

```prompt

We want slopspaces to now support a concept of 'syncing'. This is the process by which slopspaces 'expose their potential' to the outside world. In the case of a repo read or writespace this means pushing and pulling to/from the remote.

We want to have each slopspace have a 'proposal mode' -- no new arguments are required, and if the optional 'proposal mode' argument is not provided then this mode will default to 'auto-propose'.

```