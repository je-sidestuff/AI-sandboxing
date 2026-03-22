.PHONY: build clean run test

# Binary name
BINARY_NAME=agent-dispatch-watch

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build the binary
build:
	$(GOBUILD) -o $(BINARY_NAME) -v

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Run the watcher
run: build
	./$(BINARY_NAME)

# Run tests
test:
	$(GOTEST) -v ./...

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Build for Linux (useful for cross-compilation)
build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-linux -v

# Install (copy to /usr/local/bin)
install: build
	cp $(BINARY_NAME) /usr/local/bin/

# Help
help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  clean       - Remove build artifacts"
	@echo "  run         - Build and run the watcher"
	@echo "  test        - Run tests"
	@echo "  deps        - Download and tidy dependencies"
	@echo "  build-linux - Build for Linux"
	@echo "  install     - Install to /usr/local/bin"
