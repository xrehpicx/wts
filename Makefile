APP_NAME := wts
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP_NAME)
GO := go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
GOLANGCI_LINT := $(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

# Allow: make run tui / make run switch demo-local
ifeq ($(firstword $(MAKECMDGOALS)),run)
ifeq ($(strip $(ARGS)),)
ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
endif
$(eval $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS)):;@:)
endif

.PHONY: help airflow check tidy fmt vet lint test coverage docs build run install clean

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "Targets:\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

airflow: check build ## Full local dev flow

check: tidy fmt vet lint test ## Run all quality checks

tidy: ## Tidy module dependencies
	$(GO) mod tidy

fmt: ## Format Go code
	$(GO) fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

lint: ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

test: ## Run tests
	$(GO) test ./...

coverage: ## Generate coverage report in coverage.out
	$(GO) test -coverprofile=coverage.out ./...

docs: ## Generate CLI markdown docs and man pages
	$(GO) run ./cmd/genman

build: ## Build binary to ./bin/wts
	mkdir -p $(BIN_DIR)
	$(GO) build $(LDFLAGS) -o $(BIN) .

run: ## Run CLI. Example: make run ARGS="list"
	$(GO) run . $(ARGS)

install: ## Install binary in GOPATH/bin
	$(GO) install $(LDFLAGS) .

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) coverage.out
