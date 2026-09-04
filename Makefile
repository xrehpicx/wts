APP_NAME := wts
BIN_DIR := bin
BIN := $(BIN_DIR)/$(APP_NAME)
GO := go
VERSION ?= $(shell git describe --tags --match 'v[0-9]*.[0-9]*.[0-9]*' --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)"
GOLANGCI_LINT := $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
GOIMPORTS := $(GO) run golang.org/x/tools/cmd/goimports@v0.48.0
GOVULNCHECK := $(GO) run golang.org/x/vuln/cmd/govulncheck@v1.6.0
COVERAGE_MIN ?= 60

# Allow: make run tui / make run switch demo-local
ifeq ($(firstword $(MAKECMDGOALS)),run)
ifeq ($(strip $(ARGS)),)
ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
endif
$(eval $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS)):;@:)
endif

.PHONY: help airflow check tidy tidy-check fmt fmt-check vet lint test test-race test-e2e coverage coverage-check vuln docs build run install clean release

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "Targets:\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

airflow: check build ## Full local dev flow

check: tidy-check fmt-check vet lint coverage-check vuln ## Run all quality checks

tidy: ## Tidy module dependencies
	$(GO) mod tidy

tidy-check: ## Verify module files are tidy without changing them
	$(GO) mod tidy -diff

fmt: ## Format Go code and imports
	$(GOIMPORTS) -w .

fmt-check: ## Verify Go formatting and import order
	@test -z "$$($(GOIMPORTS) -l .)" || { $(GOIMPORTS) -l .; exit 1; }

vet: ## Run go vet
	$(GO) vet ./...

lint: ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

test: ## Run tests
	$(GO) test -shuffle=on ./...

test-race: ## Run tests with race detection and coverage
	$(GO) test -race -shuffle=on -coverprofile=coverage.out ./...

test-e2e: ## Run the real git/tmux CLI integration test
	WTS_E2E=1 $(GO) test -count=1 -v ./integration

coverage: ## Generate coverage report in coverage.out
	$(GO) test -coverprofile=coverage.out ./...

coverage-check: test-race ## Enforce the minimum total test coverage
	@total=$$($(GO) tool cover -func=coverage.out | awk '/^total:/ {gsub("%", "", $$3); print int($$3)}'); \
		test "$$total" -ge "$(COVERAGE_MIN)" || { echo "coverage $$total% is below $(COVERAGE_MIN)%"; exit 1; }

vuln: ## Scan reachable code for known vulnerabilities
	$(GOVULNCHECK) ./...

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

BUMP ?= patch
release: airflow ## Publish verified main through GitHub Actions (BUMP=patch or minor)
	@scripts/release.sh "$(BUMP)"
