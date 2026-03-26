# Sessions

Planned functionality for agent session management.

## Switching

Sessions can be switched between, allowing a user or orchestrator to move focus from one active session to another. Switching preserves the state of the session being left so it can be resumed later.

## Joining with Multiple Heads

Multiple agents or users can join the same session simultaneously. Each participant ("head") shares the same session context, enabling collaborative or parallel operation within a single session.

## Naming

Sessions can be assigned human-readable names. Named sessions are easier to reference, switch to, and reason about compared to sessions identified only by opaque IDs.

## Driving with External Input

Sessions can be driven by external input sources, not just interactive user messages. This allows automated systems, pipelines, or other agents to supply input to a running session, enabling programmatic control and integration with external workflows.
