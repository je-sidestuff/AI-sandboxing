.PHONY: all build run clean dev help

# Binary name
BINARY := agent-viewer

# Default target
all: build

# Build the binary
build:
	go build -o $(BINARY) .

# Run the application (builds first if needed)
run: build
	./$(BINARY)

# Development mode - run without building a binary
dev:
	go run .

# Clean build artifacts
clean:
	rm -f $(BINARY)

# Show help
help:
	@echo "agent-viewer Makefile targets:"
	@echo "  make        - Build the binary (default)"
	@echo "  make build  - Build the binary"
	@echo "  make run    - Build and run the application"
	@echo "  make dev    - Run in development mode (go run)"
	@echo "  make clean  - Remove build artifacts"
	@echo "  make help   - Show this help message"
