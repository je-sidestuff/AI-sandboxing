# Simple

<!--
```condoc-yaml
condoc:
  startTime: 1779734463
  controlScheme: same-repo
  branch: condoc/Simple-1779734463/main
  callerPath: ../../../..
```
-->

A simple conversational document where we create great content about cats.


### Step 1 - Quick Nice Little Cat Story

We'll tell the AI to write a little story here...

```prompt
Create a short story about cats in the file 'AI-sandboxing/discussion/condocs-examples/simple/CatStory.md'.
```

these notes won't be transposed into our step document.


### Step 2 - <REPLACE-TITLE>

```prompt
<REPLACE-PROMPT>
```


## Human-Prompt

The first step has bee ncompleted and we have templated our second step.

Note that everything after the 'Human-Prompt' header will be removed for our next interaction.

The second step has been templated. Please replace the title and the prompt with the desired input for our AI.

When you are done add the '!HANDOFF!' directive to the end of the file followed by only whitespace, and the handler will instruct the AI to execute the first step.

Alternatively, add the '!COMPLETED!' directive to the end of the file to consider this condoc a success and conclude it.
