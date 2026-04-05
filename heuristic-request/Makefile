.PHONY: build run clean test fmt vet lint deps \
	setup-user check-user setup-dirs setup verify-setup

BINARY_NAME=heuristic-request
AGENT_USER=heuristic-request
AGENT_DIR=/agent/heuristic-request

# ============================================================================
# Build targets
# ============================================================================

build:
	go build -o $(BINARY_NAME) .

# Run the heuristic request watcher
# By default, uses the restricted heuristic-request user if available
# Set AGENT_USER= to disable user isolation (for debugging)
run: build verify-setup
	./$(BINARY_NAME)

# Run without user isolation (for development/debugging)
run-dev: build
	AGENT_USER= ./$(BINARY_NAME)

clean:
	go clean
	rm -f $(BINARY_NAME)

test:
	go test -v ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet

deps:
	go mod tidy
	go mod download

# ============================================================================
# User and directory setup for host mode isolation
# ============================================================================
# These targets manage the restricted user and directory structure needed
# for running the AI agent with least-privilege isolation.
#
# The model:
# - heuristic-request binary runs as a privileged user (or root)
# - It prepares files in /agent/heuristic-request/read/ and /agent/heuristic-request/write/
# - The AI itself runs as the restricted 'heuristic-request' user
# - The restricted user only has access to /agent/heuristic-request/

# Create the restricted heuristic-request user (requires root/sudo)
setup-user:
	@echo "Setting up heuristic-request user..."
	@if id $(AGENT_USER) >/dev/null 2>&1; then \
		echo "User '$(AGENT_USER)' already exists"; \
	else \
		echo "Creating user '$(AGENT_USER)'..."; \
		sudo useradd --system --shell /usr/sbin/nologin \
			--home-dir $(AGENT_DIR) \
			--no-create-home \
			--comment "Restricted user for AI agent execution" \
			$(AGENT_USER); \
		echo "User '$(AGENT_USER)' created"; \
	fi

# Check if the heuristic-request user exists
check-user:
	@if id $(AGENT_USER) >/dev/null 2>&1; then \
		echo "User '$(AGENT_USER)' exists (uid=$$(id -u $(AGENT_USER)))"; \
	else \
		echo "ERROR: User '$(AGENT_USER)' does not exist"; \
		echo "Run 'make setup-user' to create it (requires sudo)"; \
		exit 1; \
	fi

# Create the agent workspace directory structure (requires root/sudo)
#
# Permission model:
# - The agent sees everything under /agent/<type>/ as its filesystem world
# - OS-level user permissions enforce read vs write access:
#   - /agent/<type>/        : root-owned, 755 (agent can traverse)
#   - /agent/<type>/read/   : root-owned, 1777 (sticky bit - anyone can create subdirs,
#                             but only owner can delete; agent can read all content)
#   - /agent/<type>/write/  : agent-owned, 1777 (sticky bit - agent can write here)
# - The binary (running as vscode or any user) can create subdirs in read/ and write/
# - The agent user can only write within write/ (read/ subdirs are owned by binary user)
# - Sticky bit (1777) allows the binary user to create/cleanup workspace without root
setup-dirs:
	@echo "Setting up agent workspace directories..."
	@sudo mkdir -p $(AGENT_DIR)/read
	@sudo mkdir -p $(AGENT_DIR)/write
	@# Root owns the agent dir (agent can traverse)
	@sudo chown root:root $(AGENT_DIR)
	@sudo chmod 755 $(AGENT_DIR)
	@# Read space: sticky bit allows binary user to create subdirs, agent can read all
	@sudo chown root:root $(AGENT_DIR)/read
	@sudo chmod 1777 $(AGENT_DIR)/read
	@# Write space: sticky bit allows binary user to create subdirs, agent can write
	@if id $(AGENT_USER) >/dev/null 2>&1; then \
		sudo chown $(AGENT_USER):$(AGENT_USER) $(AGENT_DIR)/write; \
		echo "Write space $(AGENT_DIR)/write owned by $(AGENT_USER)"; \
	else \
		echo "Warning: User $(AGENT_USER) not found, write space owned by root"; \
	fi
	@sudo chmod 1777 $(AGENT_DIR)/write
	@echo "Workspace directories ready at $(AGENT_DIR)"
	@echo "Permission model:"
	@echo "  $(AGENT_DIR)/       - root:root 755 (traverse)"
	@echo "  $(AGENT_DIR)/read/  - root:root 1777 (sticky - binary creates, agent reads)"
	@echo "  $(AGENT_DIR)/write/ - $(AGENT_USER) 1777 (sticky - agent writes here)"

# Full setup: user + directories
setup: setup-user setup-dirs
	@echo ""
	@echo "Setup complete. The heuristic-request watcher can now run with user isolation."
	@echo "Run 'make run' to start the watcher."

# Verify that setup is complete (used as prerequisite for 'make run')
verify-setup:
	@if [ ! -d "$(AGENT_DIR)" ]; then \
		echo "ERROR: Agent directory $(AGENT_DIR) does not exist"; \
		echo "Run 'make setup' first (requires sudo)"; \
		exit 1; \
	fi
	@if ! id $(AGENT_USER) >/dev/null 2>&1; then \
		echo "ERROR: User '$(AGENT_USER)' does not exist"; \
		echo "Run 'make setup' first (requires sudo)"; \
		exit 1; \
	fi
	@echo "Setup verified: user '$(AGENT_USER)' and directory '$(AGENT_DIR)' exist"

# Remove the heuristic-request user (requires root/sudo)
remove-user:
	@echo "Removing heuristic-request user..."
	@if id $(AGENT_USER) >/dev/null 2>&1; then \
		sudo userdel $(AGENT_USER); \
		echo "User '$(AGENT_USER)' removed"; \
	else \
		echo "User '$(AGENT_USER)' does not exist"; \
	fi

# Remove workspace directories (requires root/sudo)
remove-dirs:
	@echo "Removing agent workspace directories..."
	@if [ -d "$(AGENT_DIR)" ]; then \
		sudo rm -rf $(AGENT_DIR); \
		echo "Directory $(AGENT_DIR) removed"; \
	else \
		echo "Directory $(AGENT_DIR) does not exist"; \
	fi

# Full cleanup: remove user and directories
teardown: remove-dirs remove-user
	@echo "Teardown complete."
