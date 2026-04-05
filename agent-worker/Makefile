.PHONY: build run clean test fmt vet lint deps \
	setup-user check-user setup-dirs setup verify-setup

BINARY_NAME=agent-worker
AGENT_USER=agent-worker
AGENT_DIR=/agent/agent-worker

# ============================================================================
# Build targets
# ============================================================================

build:
	go build -o $(BINARY_NAME) .

# Run the agent worker
# By default, uses the restricted agent-worker user if available
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
# - agent-worker binary runs as a privileged user (or root)
# - It prepares files in /agent/agent-worker/read/ and /agent/agent-worker/write/
# - The AI itself runs as the restricted 'agent-worker' user
# - The restricted user only has access to /agent/agent-worker/

# Create the restricted agent-worker user (requires root/sudo)
setup-user:
	@echo "Setting up agent-worker user..."
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

# Check if the agent-worker user exists
check-user:
	@if id $(AGENT_USER) >/dev/null 2>&1; then \
		echo "User '$(AGENT_USER)' exists (uid=$$(id -u $(AGENT_USER)))"; \
	else \
		echo "ERROR: User '$(AGENT_USER)' does not exist"; \
		echo "Run 'make setup-user' to create it (requires sudo)"; \
		exit 1; \
	fi

# Create the agent workspace directory structure (requires root/sudo)
# The directories use permissive mode (777) because:
# - Security comes from the restricted agent user, not directory permissions
# - The read/ and write/ subdirectories are temporary (cleaned up after each work unit)
# - This allows any user to run 'make run' and clean up previous workspace contents
setup-dirs:
	@echo "Setting up agent workspace directories..."
	@sudo mkdir -p $(AGENT_DIR)/read
	@sudo mkdir -p $(AGENT_DIR)/write
	@if id $(AGENT_USER) >/dev/null 2>&1; then \
		sudo chown $(AGENT_USER):$(AGENT_USER) $(AGENT_DIR); \
		echo "Directory $(AGENT_DIR) owned by $(AGENT_USER)"; \
	else \
		echo "Warning: User $(AGENT_USER) not found, directories created but not chowned"; \
	fi
	@sudo chmod 755 $(AGENT_DIR)
	@sudo chmod 1777 $(AGENT_DIR)/read $(AGENT_DIR)/write
	@echo "Workspace directories ready at $(AGENT_DIR)"
	@echo "Note: read/ and write/ use permissive mode for cleanup flexibility"

# Full setup: user + directories
setup: setup-user setup-dirs
	@echo ""
	@echo "Setup complete. The agent-worker can now run with user isolation."
	@echo "Run 'make run' to start the worker."

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

# Remove the agent-worker user (requires root/sudo)
remove-user:
	@echo "Removing agent-worker user..."
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
