package agentselect

import (
	"fmt"
	"regexp"
	"strings"
)

type Selection struct {
	Agent      string
	Model      string
	Capability string
	Rationale  string
}

var capabilityRegistry = map[string]struct {
	Agent string
	Model string
}{
	"image": {Agent: "grok", Model: "grok-code-fast-1"},
	"cheap": {Agent: "opencode", Model: "google/gemini-2.5-flash"},
}

func extractDirectFromText(text string) (agent, model, capability string) {
	text = strings.ToLower(text)
	reAgent := regexp.MustCompile(`we will use agent (\w+)`)
	reModel := regexp.MustCompile(`we will use model ([\w/.-]+)`)
	reCap := regexp.MustCompile(`we will use (?:an? )?agent with (\w+) capability`)

	if matches := reAgent.FindStringSubmatch(text); len(matches) > 1 {
		agent = matches[1]
	}
	if matches := reModel.FindStringSubmatch(text); len(matches) > 1 {
		model = matches[1]
	}
	if matches := reCap.FindStringSubmatch(text); len(matches) > 1 {
		capability = matches[1]
	}
	return
}

func isCompatible(agent, model, capability string) bool {
	if capability == "" {
		return true
	}
	if cfg, ok := capabilityRegistry[capability]; ok {
		if agent != "" && agent != cfg.Agent {
			return false
		}
		if model != "" && model != cfg.Model {
			return false
		}
	}
	return true
}

func Select(taskDesc, directAgent, directModel, directCapability string) (Selection, error) {
	var rationale []string
	parsedAgent, parsedModel, parsedCap := extractDirectFromText(taskDesc)

	agent := directAgent
	if agent == "" {
		agent = parsedAgent
	}
	model := directModel
	if model == "" {
		model = parsedModel
	}
	capability := directCapability
	if capability == "" {
		capability = parsedCap
	}

	if capability == "" && (strings.Contains(strings.ToLower(taskDesc), "image") || strings.Contains(strings.ToLower(taskDesc), "diagram") || strings.Contains(strings.ToLower(taskDesc), "vision") || strings.Contains(strings.ToLower(taskDesc), "draw")) {
		capability = "image"
		rationale = append(rationale, "inferred image capability from task description")
	} else if capability == "" && (strings.Contains(strings.ToLower(taskDesc), "cheap") || strings.Contains(strings.ToLower(taskDesc), "simple") || strings.Contains(strings.ToLower(taskDesc), "fast") || strings.Contains(strings.ToLower(taskDesc), "quick")) {
		capability = "cheap"
		rationale = append(rationale, "inferred cheap capability from task description")
	}

	if capability != "" {
		if cfg, ok := capabilityRegistry[capability]; ok {
			if agent == "" {
				agent = cfg.Agent
			}
			if model == "" {
				model = cfg.Model
			}
			rationale = append(rationale, fmt.Sprintf("capability %s selected %s::%s", capability, cfg.Agent, cfg.Model))
		} else {
			return Selection{}, fmt.Errorf("unknown capability: %s", capability)
		}
	}

	if agent == "" {
		agent = "claude"
		rationale = append(rationale, "defaulted to high-capability claude")
	}

	if !isCompatible(agent, model, capability) {
		return Selection{}, fmt.Errorf("incompatible agent/model/capability combination: %s/%s with capability %s", agent, model, capability)
	}

	if directAgent != "" {
		rationale = append(rationale, fmt.Sprintf("direct agent: %s", directAgent))
	}
	if directModel != "" {
		rationale = append(rationale, fmt.Sprintf("direct model: %s", directModel))
	}
	if directCapability != "" {
		rationale = append(rationale, fmt.Sprintf("direct capability: %s", directCapability))
	}
	if len(rationale) == 0 {
		rationale = append(rationale, "no specific selection criteria")
	}

	return Selection{
		Agent:      agent,
		Model:      model,
		Capability: capability,
		Rationale:  strings.Join(rationale, "; "),
	}, nil
}

func ListCapabilities() []string {
	var caps []string
	for c := range capabilityRegistry {
		caps = append(caps, c)
	}
	return caps
}

func GetCapabilityConfig(capability string) (string, string, bool) {
	if cfg, ok := capabilityRegistry[capability]; ok {
		return cfg.Agent, cfg.Model, true
	}
	return "", "", false
}
