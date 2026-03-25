# Future Directions for agent-advisor

This document outlines possible directions for evolving the agent-advisor beyond its current skeleton implementation.

## Current State

agent-advisor is a heuristic harness agent that:
- Reads topic-based configs from a config directory (auto-created if absent)
- Runs on a daily schedule per config
- Invokes an agent in prompt-only mode (`-p`) to produce advisory recommendations
- Outputs `INSTRUCTION.json` (mode: `prompt`) into the request directory for human review

---

## Possible Directions

### 1. Context Injection

Feed the advisor real context before generating advice:
- Read recent records from `agent-records/` to summarize recent activity
- Inspect work unit backlogs (`input/any/`) for queue depth
- Pull recent git log or diff summaries for the target repos
- Attach this context to the prompt so recommendations are grounded in actual state

### 2. Multi-Topic Sessions

Currently each advisor config fires one instruction per day. Future options:
- Run each topic in `topics[]` as a separate agent call, producing one instruction per topic
- Aggregate results into a single advisory session summary
- Allow per-topic scheduling overrides (e.g., topic A daily, topic B weekly)

### 3. Advisor Profiles / Personas

Support different advisory styles via config:
- `"style": "conservative"` — only suggest low-risk, well-understood actions
- `"style": "exploratory"` — propose novel or experimental directions
- `"style": "critical"` — focus on identifying problems and gaps
- Customize the prompt template based on style

### 4. Feedback Loop

Let the advisor learn from what happened after its advice:
- After an instruction is processed, annotate the advisor record with the outcome
- Use outcome history to refine future prompts ("last time I suggested X, it resulted in Y")
- Over time, build a preference model for what types of advice tend to get approved

### 5. Escalation and Priority

Add urgency tiers to advisory output:
- `"priority": "high"` — routes to a faster approval or different downstream queue
- `"priority": "low"` — can be batched or deferred
- Let the advisor itself determine priority based on the topic and context

### 6. Cross-Advisor Awareness

When multiple advisor configs are active, they currently operate independently:
- Deduplicate or consolidate advice before placing work units
- Allow advisors to reference each other's recent output
- Implement a coordinator role that synthesizes multiple advisory sessions into one coherent set of recommendations

### 7. Timer-Based Mode

Currently only schedule-based (daily at a fixed time). Add timer-based support:
- `"type": "timer"` with `"interval": "6h"` for higher-frequency advisory
- Useful for monitoring-style advisors that check for anomalies on short cycles

### 8. Output Routing

Currently always writes to `REQUEST_DIR`. Options:
- Route to `heuristic/pending/` for heuristic-request to further process (chained advisory)
- Route directly to `DISPATCH.json` for immediate dispatch orchestration
- Support configurable output targets per advisor

### 9. Dry Run Mode

Add a `--dry-run` flag that:
- Builds and logs the advisory prompt without invoking the agent
- Useful for testing new configs or debugging prompt content

### 10. Config Reload

Currently configs are loaded once at startup. Options:
- Poll the config directory for changes and reload without restart
- Support `SIGHUP` to trigger a config reload
- Useful for adjusting advisor topics or schedules without downtime
