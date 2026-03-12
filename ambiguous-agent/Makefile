BINARY := ambiguous-shell
GO     := go

.PHONY: build run clean install deps

deps:
	$(GO) mod tidy

build: deps
	$(GO) build -o $(BINARY) .

run: build
	./$(BINARY)

install: build
	cp $(BINARY) /usr/local/bin/
	chmod +x invoke-agent.sh
	cp invoke-agent.sh /usr/local/bin/

clean:
	rm -f $(BINARY)
	$(GO) clean
