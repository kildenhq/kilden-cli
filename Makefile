# Kilden CLI (kd) — dev tasks. Release binaries are built by goreleaser in CI
# on a pushed v* tag (see .github/workflows/release.yml); this Makefile is for
# local development only.

BINARY  := kd
# Stamp the version from git so a local `make build` reports something useful;
# CI overrides this via goreleaser's own ldflags.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := build

.PHONY: build install test fmt vet lint check tidy snapshot clean

build: ## Build the kd binary into ./kd
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install: ## Install kd into $GOBIN
	go install -ldflags "$(LDFLAGS)" .

test: ## Run tests with the race detector
	go test -race ./...

fmt: ## Format all Go sources
	gofmt -w .

vet: ## Run go vet
	go vet ./...

lint: ## Check formatting without writing (matches CI)
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)

check: lint vet test ## Everything CI runs, locally

tidy: ## Tidy go.mod / go.sum
	go mod tidy

snapshot: ## Build cross-platform archives locally (needs goreleaser)
	goreleaser release --snapshot --clean

clean: ## Remove build artifacts
	rm -rf $(BINARY) dist/
