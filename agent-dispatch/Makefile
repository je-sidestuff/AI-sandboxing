.PHONY: build build-poller run run-poller clean test fmt vet lint deps all

BINARY_NAME=agent-dispatch
POLLER_NAME=prpoller-cli

all: build build-poller

build: deps
	go build -o $(BINARY_NAME) .

build-poller: deps
	go build -o $(POLLER_NAME) ./cmd/prpoller

run: build
	./$(BINARY_NAME)

run-poller: build-poller
	./$(POLLER_NAME)

clean:
	go clean
	rm -f $(BINARY_NAME) $(POLLER_NAME)

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

install: all
	cp $(BINARY_NAME) /usr/local/bin/
	cp $(POLLER_NAME) /usr/local/bin/
