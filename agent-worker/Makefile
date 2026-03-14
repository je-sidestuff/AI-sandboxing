.PHONY: build run clean test fmt vet lint deps

BINARY_NAME=agent-worker

build:
	go build -o $(BINARY_NAME) .

run: build
	./$(BINARY_NAME)

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
