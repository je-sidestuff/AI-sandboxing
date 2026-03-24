package main

import (
	"fmt"

	"agent-recorder"
)

func main() {
	fmt.Println("=== agent-tester: emitting test records ===")

	det := recorder.New(
		"det-agent-001",
		recorder.Deterministic,
		"test.run",
		map[string]string{"status": "pass", "checks": "3"},
	)
	recorder.Emit(det)

	heu := recorder.New(
		"heu-agent-002",
		recorder.Heuristic,
		"test.run",
		map[string]string{"status": "pass", "score": "0.97"},
	)
	recorder.Emit(heu)
}
