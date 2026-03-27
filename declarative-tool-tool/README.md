# declarative-tool-tool (tool-tool)

A Go CLI for creating, registering, and executing **declarative tools** — known as **T3s** (declarative-tool-tool-tools).

## What is a T3?

A T3 is a tool defined by a simple, flat YAML (or JSON) schema. Think of it as an oversimplified Terraform: a self-describing, schema-backed executable unit. The schema captures:

- **name** and **description** — human-readable identity
- **inputs** — a flat map of named parameters with types and optional defaults
- **outputs** — what the tool produces
- **entrypoint** — how to invoke the tool (command, script path, etc.)

T3s can be written in any language or technology. **Python is the preferred convention** by default.

## Commands

```
tool-tool create    # Scaffold a new T3
tool-tool register  # Register a T3 in the local registry
tool-tool execute   # Execute a registered T3 with an input file
```

## Build

```bash
go build -o tool-tool .
```

## Future increments

> **Next increment:** Add a complete worked example T3 — a Python tool that
> reads a flat YAML file describing a file/directory tree (with file content
> included inline) and materialises that tree on disk. This will serve as the
> canonical reference T3 and exercise the full create → register → execute
> lifecycle.
