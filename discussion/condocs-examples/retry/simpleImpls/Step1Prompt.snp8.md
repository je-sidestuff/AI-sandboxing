# Prompt

[Simple](../Simple.md)

Create a short story about cats in the file 'AI-sandboxing/discussion/condocs-examples/retry/CatStory.md'.


[`3f7a1c2`](https://github.com/example-user/research/commit/3f7a1c2d4e5b6a7c8d9e0f1a2b3c4d5e6f7a8b9c)

## Reply

I have created a wonderful story for you in the file 'CatStory.md' in the directory above this one.

I chose the theme of a pompous cat attacking an unsuspecting creature.


## Revision A

Don't make the story much longer (if at all) - but PLEASE give the cat a name. Cats need names.


[`7f2a9e3`](https://github.com/example-user/research/commit/7f2a9e3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f)

## Reply A

I have made this story less anonymous.

Cats deserve names - even violent or malevolent ones.


## Retry B (from start)

I've changed my mind on the whole direction. A pompous hunting cat isn't the story I want.
Please write something warmer — perhaps a cat and their person sharing a quiet moment.


[`2d5f8a3`](https://github.com/example-user/research/commit/2d5f8a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f)

## Reply B

I've written a warmer, quieter story this time. I hope the contrast from the first attempt is clear.


## <REPLACE-Revision|Retry> C

<REPLACE-PROMPT>


## Human-Prompt

The AI has responded to the next increment in this step.

Note that everything after the 'Human-Prompt' header will be removed for our next interaction.

To REVISE further, replace '<REPLACE-Revision|Retry>' with 'Revision' to incorporate the AI's current work and add to it.

To RETRY from a previous point replace the '<REPLACE-Revision|Retry>' with 'Retry'. By default this retries from the previous increment. To retry from further back add '(from start)' or '(from X)' where X is a revision letter, for example '## Retry C (from A)'.

Replace the '<REPLACE-PROMPT>' with the new prompt you wish the agent to follow.

When you are done add the '!HANDOFF!' directive to the end of the file followed by only whitespace.

Alternatively, add the '!COMPLETED!' directive to the end of the file to consider this step a success and conclude it.
