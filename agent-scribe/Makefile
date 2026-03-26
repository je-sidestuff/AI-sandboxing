.PHONY: all install run clean dev help

# Default target
all: install

# Install dependencies
install:
	npm install

# Run the application
run: install
	npm start

# Development mode - same as run for Node.js
dev: install
	npm start

# Clean build artifacts and dependencies
clean:
	rm -rf node_modules

# Show help
help:
	@echo "agent-scribe Makefile targets:"
	@echo "  make         - Install dependencies (default)"
	@echo "  make install - Install npm dependencies"
	@echo "  make run     - Run the application"
	@echo "  make dev     - Run in development mode"
	@echo "  make clean   - Remove node_modules"
	@echo "  make help    - Show this help message"
