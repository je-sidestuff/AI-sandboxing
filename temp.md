
We are creating a new capability for continuity in interactive AI collaboration called 'condocs', or 'conversational documents'.

These are documents which provide an interaction surface for a human and an agent. They are an 'md' based format where there is a top level document and one or more layers of nested child documents.

The top level document is divided into coarse 'steps' which represent stages of collaborative development being performed by the human-AI pair (henceforth 'the pair').

Each 'step' heading optionally contains comments and must contain triple-backtic 'prompt' block. This is the major initiating prompt which starts a new logical division of work.

When we use the handler to instruct the AI to begin a new step it automatically creates a child discussion document and metadata document, populates the header metadata and the parent link back, copies the prompt into the child discussion under the heading '## Prompt', and instructs the AI to execute the prompt.

The output of the AI is captured and placed in the discussion child doc after the heading '## Reply'.