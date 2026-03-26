package main

import (
	"fmt"

	"agent-recorder"
)

func main() {
	fmt.Println("=== agent-executor: emitting execution records ===")

	det := recorder.New(
		"det-agent-001",
		recorder.Deterministic,
		"cmd.exec",
		map[string]string{"status": "pass", "checks": "3"},
	)
	recorder.Emit(det)

	heu := recorder.New(
		"heu-agent-002",
		recorder.Heuristic,
		"cmd.exec",
		map[string]string{"status": "pass", "score": "0.97"},
	)
	recorder.Emit(heu)
}
