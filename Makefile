BINARY  := no-pilot
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"
OUTDIR  := bin

.PHONY: build build-all lint test clean run

## build: compile for the current platform
build:
	go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY) .

## build-all: cross-compile for all distribution targets
build-all:
	GOOS=linux   GOARCH=amd64  go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-linux-amd64   .
	GOOS=linux   GOARCH=arm64  go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-linux-arm64   .
	GOOS=darwin  GOARCH=amd64  go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-darwin-amd64  .
	GOOS=darwin  GOARCH=arm64  go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-darwin-arm64  .
	GOOS=windows GOARCH=amd64  go build $(LDFLAGS) -o $(OUTDIR)/$(BINARY)-windows-amd64.exe .

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## test: run all tests
test:
	go test -race -count=1 ./...

## run: build and start the MCP server (stdio)
run: build
	./$(OUTDIR)/$(BINARY)

## clean: remove build artifacts
clean:
	rm -rf $(OUTDIR)
