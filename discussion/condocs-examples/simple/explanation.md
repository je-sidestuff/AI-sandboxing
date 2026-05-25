# Explanation

This directory contains snapshots of a condoc interaction at different phases.

The syntax of our snapshot files is <file>.snpN.<ext> to denote what is happening at a particular point in time (ie: Simple.snp1.md is the file ).

If we are missing a <file>.snpX.<ext> but we have other numbers with gaps, this simply means the file did not change between those times.

This file explains what is happening in the '## Storyline' section, and each '### Snap N' subsection indicates a point in time.

## Storyline

This interaction begins when the user uses the 'condoc AI-sandboxing/discussion/condocs-examples/simple/Simple.md "A simple conversational document where we create great content about cats."' into the federation-command CLI.

The federation-command enters the condoc mode with that file location in context.

Because there was no valid ongoing condoc at that location the handler recognizes that it should create a new file at that location.

The condoc handler finds that the 'AI-sandboxing/discussion/condocs-examples/simple/' directory has the repository root 'AI-sandboxing'.

Because the repo is in the main branch the handler recognizes that it should create a new branch following the pattern condoc/Simple-<timestamp>/main -- it does not do this right away, but we see in the 'snp0' doc that it proposes this.

The 'same-repo' controlScheme means that the handler will be operating directly on the filesystem through that repository, working on the branch specified. When the working tree is clean it fetchs from the remote every 10 seconds to 'pull --rebase' the changes from its target branch.

The condoc handler also watches for changes to the condoc main file and child files. When it sees they have been changed it checks to see if it has received the '!HANDOFF!'.

### Snap 0

The handler has created the file now and templated it with the header material so that the human can see what is proposed.

The human is working locally and sees the proposal. The human likes the setup an adds the '!HANDOFF!' directive.

### Snap 1

The human has responded.

The condoc handler sees this and reads the file. The fact that we have a '!HANDOFF!' in this document is the signal that tells the handler the proposal has been accepted. It switches the repo to the new branch, templates out the first step, and commits and pushes.

### Snap 2

The condoc handler (without any AI help yet) has committed and pushed the changes to the condoc branch. The first step has been templated for the human.

The human enters a title and description and adds the '!HANDOFF!' directive again.

### Snap 3

The human has given a title and prompt.

The handler creates the 'step-file' child document for the first step - 'AI-sandboxing/discussion/condocs-examples/simple/simpleImpls/Step1Prompt.md' and copies the prompt into it as a starting point under the '# Prompt' header.

The prompt is fed to our agent with the ferdation-command command 'agent -w <step-file>'. The default mode is write (and we will not get into permutations here yet, we'll assume this is always the case for now) and the agent proceeds with the execution of the prompt.

The prompt is output in the federation-command terminal which is waiting in the busy 'condoc mode' when the handoff occurs. The response from the agent is also output in the terminal.

When the output is received from the agent it is added to the '## Reply' header at the bottom of the document.

The handler then adds the '## Human-Prompt' section to explain what the human should do next.

The handler then commits all changes and pushes them to the remote branch.

### Snap 4

The human sees the results of the AI's first increment of work as well as the reply it gave to the prompt.

The human is partially happy with the work, but wishes it was less anonymous - cats deserve names.

The human adds extra direction in the form of a Revision and triggers the AI again.

### Snap 5

The condoc handler sees that it has beened commanded to trigger the AI with a reprompt.

The UI displays the prompt as it is submitted to the AI and the response as it comes back. The changes are automatically pushed to the branch.

### Snap 6

The human sees that the AI has completed the next prompt.

It has done well this time - the step is complete and the human marks it as such.

### Snap 7

The step has been completed. Things are good.

### Snap 8

The condoc has been completed.

### Snap 9

The condoc is marked as completed and the handler exits the scope of this condoc.