# Contained AI Operations

This module creates a new private repository, takes the 'operating repository' as a starting point, and populates the current state of the operating repository as the initial state. The initial state is pushed and the .git directory is removed to prevent access.

The module then executes the given input command with the selected agent.

Once the input command is executed the result is checked, and if the result is successful the new state of the repository is pushed to a branch and a PR is raised.

Order of operations:
1. Create a new private repository
1. Clone the operating repository and copy the contents to the new repository
1. Push the initial state to the new repository
1. Remove the .git directory from the new repository (set it aside outside of the agent scope)
1. Execute the input command with the selected agent
1. Check the result of the command execution
1. If the result is successful, push the new state to a branch and raise a PR
