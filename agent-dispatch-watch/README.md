# agent-dispatch-watch

> **Deprecated.** This component has been superseded by `agent-dispatch`, which provides both watch mode and single-shot dispatch in a single binary. New deployments should use `agent-dispatch` instead.

## Migration

Replace `agent-dispatch-watch` with `agent-dispatch`:

```bash
# Old
cd agent-dispatch-watch && ./agent-dispatch-watch

# New (equivalent watch mode)
cd agent-dispatch && ./agent-dispatch
```

`agent-dispatch` supports all dispatch types including `repo-isolation`, which `agent-dispatch-watch` does not. It also adds single-shot dispatch, async operations, status checking, and a PR comment poller.

See [`agent-dispatch/README.md`](../agent-dispatch/README.md) for full documentation.

## What This Component Did

`agent-dispatch-watch` monitored `INPUT_DIR/any/` for `DISPATCH.json` files and processed them via terraform-managed containment workflows. It supported two dispatch types:

- `direct` — converted dispatch into an `INSTRUCTION.json` for `agent-worker` pickup
- `in-repo` — created a branch and PR in the target repository via terraform

The compiled binary (`agent-dispatch-watch`) remains in this directory from a prior build.
