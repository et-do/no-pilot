BINARY  := no-pilot
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"
OUTDIR  := bin

.PHONY: build build-all install lint test smoke clean run ci help

## help: show this help
help:
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## //'

## build: compile for the current platform
build:
	go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY) .

## install: build and install to $GOPATH/bin and /usr/local/bin
install:
	go install $(LDFLAGS) .
	sudo cp $(GOPATH)/bin/$(BINARY) /usr/local/bin/$(BINARY)

## build-all: cross-compile for all distribution targets
build-all:
	GOOS=linux   GOARCH=amd64  go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-linux-amd64   .
	GOOS=linux   GOARCH=arm64  go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-linux-arm64   .
	GOOS=darwin  GOARCH=amd64  go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-darwin-amd64  .
	GOOS=darwin  GOARCH=arm64  go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-darwin-arm64  .
	GOOS=windows GOARCH=amd64  go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-windows-amd64.exe .

## lint: run golangci-lint on all packages
lint:
	golangci-lint run ./...

## test: run all tests with race detector
test:
	go test -race -count=1 ./...

## smoke: run integration smoke test (server startup + core tools; ~30s)
smoke:
	go test -v ./internal/server -run TestSmoke -timeout 60s

## ci: run smoke + full test suite locally (mimics GitHub Actions)
ci: smoke test
	@echo "✓ CI pipeline passed"

## run: build and start the MCP server (stdio) — for manual smoke-testing; pipe JSON-RPC to stdin
run: build
	./$(OUTDIR)/$(BINARY)

## clean: remove build artifacts
clean:
	rm -rf $(OUTDIR)
