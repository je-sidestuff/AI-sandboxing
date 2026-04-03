# Driving Model Details

## Can an agent drive the use of a specific model within a sub-project like `ambiguous-agent`?

No, with the current capabilities, an agent operating within a sub-project directory (such as `ambiguous-agent`) cannot directly control or "drive" the use of a specific underlying language model. The model that an agent uses is typically configured and managed at a higher level within the environment or platform where the agent is deployed, not from within the agent's runtime or its specific project directory.

The agent's capabilities are defined by the tools it has access to and the specific model it has been instantiated with by the orchestrating system. There is no tool available to the agent that allows it to select, switch, or otherwise "drive" a different model for its own operation or for other agents.

## How we'd make sure it worked for all agents (if the capability existed)

If the capability to "drive" the use of a specific model were to be introduced, it would likely involve a system-level or platform-level design to ensure it worked consistently for all agents. Here's how such a mechanism might be implemented:

1.  **Centralized Configuration:**
    *   **Environment Variables:** The most common approach would be to use environment variables set at the deployment level. Each agent instance would inherit these variables, which could specify the model to use (e.g., `LLM_MODEL=gemini-1.5-pro`).
    *   **Configuration Files:** A shared, globally accessible configuration file (e.g., in a central `/etc` directory on a Linux system, or a cloud configuration service) could define model preferences that all agents would read upon startup.

2.  **Agent Orchestration Layer:**
    *   **API for Model Selection:** The agent orchestration system (the platform that launches and manages agents) could expose an API or a configuration parameter during agent creation that allows specifying the desired model. When an agent is launched, this parameter would be passed, dictating which model it uses.
    *   **Agent Profiles/Templates:** Different "agent profiles" or templates could be created, each pre-configured to use a specific model. When a user requests a new agent, they would select a profile, and the agent would be instantiated with the corresponding model.

3.  **Tooling/SDK within the Agent Framework:**
    *   If agents were given the ability to dynamically change models, the agent framework itself would need to provide a specialized tool or SDK function. This tool would not be something an agent could create itself, but rather a built-in feature of the platform. For example, an imaginary `model_selection` tool might exist, allowing an agent to call `model_selection.use_model(name='specific-model')`.

In summary, for model selection to work across all agents, it would require a fundamental architectural change in how agents are deployed and managed, focusing on centralized control, environment-level configuration, or platform-provided APIs/tooling, rather than individual agents within sub-projects attempting to manage their own model selection.
