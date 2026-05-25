# Prompt

We are creating a new capability for continuity in interactive AI collaboration called 'condocs', or 'conversational documents'.

These are documents which provide an interaction surface for a human and an agent. They are an 'md' based format where there is a top level document and one or more layers of nested child documents.

The top level document is divided into coarse 'steps' which represent stages of collaborative development being performed by the human-AI pair (henceforth 'the pair').

Each 'step' heading optionally contains comments and must contain triple-backtic 'prompt' block. This is the major initiating prompt which starts a new logical division of work.

When we use the handler to instruct the AI to begin a new step it automatically creates a child discussion document and metadata document, populates the header metadata and the parent link back, copies the prompt into the child discussion under the heading '## Prompt', and instructs the AI to execute the prompt.

The output of the AI is captured and placed in the discussion child doc after the heading '## Reply'.

In order to tell the agent that the condoc is in a state where it is time for it to act we can add the line !HANDOFF! to the end of the document. (As long as only whitespace is following the '!HANDOFF!' string the handler wiull recognize this.) When the agent picks up the handoff it will replace the '!HANDOFF!' string with <>

We want to implement condocs in federation-command in a very decoupled and non-invasive way. This implementation is an early iteration and we want to be able to cleanly remove everything without being disruptive if we want to try again.

For our condocs implementation we will look at the 'AI-sandboxing/discussion/Evolution1.md'/'AI-sandboxing/discussion/evolution1impls' and 'AI-sandboxing/discussion/SlopspaceAdventures.md'/'AI-sandboxing/discussion/slopspaceadventuresimpls' for inspiration. These capture the idea of the interaction but are not character-for-character examples. Also look at 'AI-sandboxing/discussion/condocs-examples' for character-for-character examples -- beginning with 'AI-sandboxing/discussion/condocs-examples/simple'. (Note that explanation.md is not part of an interaction - iut is there to explain what is happening)

Note that initially we WILL NOT IMPLEMENT THE RETRY PATH, ONLY THE REVISE.

We can look at our riealong handler dynapane for inspiration on federation-command UI design. When our UI is in 'condoc' mode it has a similar appearance to the ridealong handler but the flashing dot is green and yellow instead of blue and red, and the emoji is not a cop-car.

We will now implement enough of the condoc handler in 'AI-evo1/federation-command' to perform the steps we see in 'AI-sandboxing/discussion/condocs-examples/simple'.
