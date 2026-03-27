// declarative-tool-tool (tool-tool) — CLI for creating, registering, and executing
// declarative tools (T3s / declarative-tool-tool-tools).
//
// T3s are tools defined by simple, flat YAML/JSON schemas — similar to
// an oversimplified Terraform. The schema describes what a tool does,
// its inputs/outputs, and how to invoke it. The tool-tool manages the
// full lifecycle: scaffolding a new T3, registering it in a local registry,
// and executing a registered T3 against a set of inputs.
//
// TODO(future increment): Add a worked example T3 — a Python tool that
// reads a flat YAML describing a file/directory tree (with file content
// inline) and materialises it on disk. T3s can be written in any
// language/technology, but Python is the preferred convention.
package main

import (
	"fmt"
	"os"
)

const usage = `declarative-tool-tool — tool-tool for T3 lifecycle management

Usage:
  tool-tool <command> [args]

Commands:
  create    Scaffold a new declarative tool (T3)
  register  Register an existing T3 in the local registry
  execute   Execute a registered T3 with a given input file

Run 'tool-tool <command> --help' for command-specific help.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "create":
		runCreate(os.Args[2:])
	case "register":
		runRegister(os.Args[2:])
	case "execute":
		runExecute(os.Args[2:])
	case "--help", "-h", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

// runCreate scaffolds a new T3 in the given directory.
// TODO: prompt for tool name/description, write skeleton schema + entrypoint.
func runCreate(args []string) {
	fmt.Println("create: not yet implemented")
}

// runRegister adds a T3 to the local tool registry.
// TODO: validate schema, write entry to registry index (~/.tool-tool/registry.yaml).
func runRegister(args []string) {
	fmt.Println("register: not yet implemented")
}

// runExecute runs a registered T3 with the supplied input.
// TODO: look up T3 in registry, validate input against schema, invoke entrypoint.
func runExecute(args []string) {
	fmt.Println("execute: not yet implemented")
}
