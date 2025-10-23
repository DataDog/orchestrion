
.PHONY: all test test-e2e format lint build install dd-trace-go test-integration \
        dd-trace-go-setup actionlint yamlfmt gotestfmt ratchet ratchet/pin ratchet/update ratchet/check

# Allow overriding via env var `orchestrion_dir` or `ORCHESTRION_DIR`
ORCHESTRION_DIR ?= $(if $(orchestrion_dir),$(orchestrion_dir),$(CURDIR))
DD_TRACE_GO_DIR ?= $(CURDIR)/tmp/dd-trace-go
DDTRACE_INTEGRATION_DIR := $(DD_TRACE_GO_DIR)/internal/orchestrion/_integration

all: build format lint test

build:
	go build -o bin/orchestrion main.go

install:
	go install .

format: format/go format/yaml

format/go: golangci-lint
	@echo "Formatting Go code..."
	golangci-lint fmt

format/yaml: yamlfmt
	@echo "Formatting YAML files..."
	yamlfmt -dstar '**/*.yml' '**/*.yaml'

lint:  lint/action lint/yaml lint/action

lint/action: actionlint ratchet/check
	@echo "Linting GitHub Actions workflows..."
	actionlint

lint/go: golangci-lint
	@echo "Linting Go code..."
	golangci-lint run

lint/yaml: yamlfmt
	@echo "Linting YAML files..."
	yamlfmt -dstar '**/*.yml' '**/*.yaml'

# Ratchet - Pin GitHub Actions to commit SHAs

ratchet/pin: ratchet
	@echo "Pinning GitHub Actions to commit SHAs..."
	ratchet pin .github/workflows/*.yml .github/actions/**/action.yml

ratchet/update: ratchet
	@echo "Updating pinned GitHub Actions to latest versions..."
	ratchet update .github/workflows/*.yml .github/actions/**/action.yml

ratchet/check: ratchet
	@echo "Checking GitHub Actions are pinned..."
	ratchet lint .github/workflows/*.yml .github/actions/**/action.yml


# Tests

# Integration with dd-trace-go using orchestrion
#
# The following Make targets automate the manual steps below:
#
#   console
#   $ git clone github.com:DataDog/dd-trace-go         # Clone the DataDog/dd-trace-go repository
#   $ cd dd-trace-go/internal/orchestrion/_integration # Move into the integration tests directory
#   $ go mod edit \                                    # Use the local copy of orchestrion
#       -replace "github.com/DataDog/orchestrion=>${orchestrion_dir}"
#   $ go mod tidy                                      # Make sure go.mod & go.sum are up-to-date
#   $ go run github.com/DataDog/orchestrion \          # Run integration test suite with orchestrion
#       go test -shuffle=on ./...
#
# Usage examples:
#   make test-integration        # run tests (expects setup done)
#   make dd-trace-go-setup       # clone + set replace only


dd-trace-go:
	@mkdir -p ./tmp
	@if [ ! -d "$(DD_TRACE_GO_DIR)/.git" ]; then \
		echo "Cloning dd-trace-go (shallow) into $(DD_TRACE_GO_DIR)"; \
		git clone --depth 1 --no-tags git@github.com:DataDog/dd-trace-go.git "$(DD_TRACE_GO_DIR)"; \
	else \
		echo "dd-trace-go already exists at $(DD_TRACE_GO_DIR)"; \
	fi

.ONESHELL:
dd-trace-go-setup: dd-trace-go
	@echo "Using orchestrion from: $(ORCHESTRION_DIR)"
	@echo "Integration dir: $(DDTRACE_INTEGRATION_DIR)"
	cd $(DDTRACE_INTEGRATION_DIR)
	go mod edit -replace "github.com/DataDog/orchestrion=$(ORCHESTRION_DIR)"
	go mod tidy

.ONESHELL:
test: gotestfmt
	set -euo pipefail
	go test -json -v -timeout=5m ./... 2>&1 | tee ./gotest.log | gotestfmt

.ONESHELL:
test-e2e: build
	set -euo pipefail
	@echo "Running end-to-end tests..."
	go test -tags=e2e -v -timeout=10m ./test/e2e/ 2>&1 | tee test-e2e.log

.ONESHELL:
test-integration: dd-trace-go-setup
	cd $(DDTRACE_INTEGRATION_DIR)
	go run github.com/DataDog/orchestrion go test -v -shuffle=on -failfast ./... | tee $(ORCHESTRION_DIR)/test-integration.log

# Install tools

gotestfmt:
	@if ! command -v gotestfmt >/dev/null 2>&1; then \
		echo "Installing gotestfmt..."; \
		go install github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt@latest; \
	fi

golangci-lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; \
	fi

actionlint:
	@if ! command -v actionlint >/dev/null 2>&1; then \
		echo "Installing actionlint..."; \
		go install github.com/rhysd/actionlint/cmd/actionlint@latest; \
	fi

yamlfmt:
	@if ! command -v yamlfmt >/dev/null 2>&1; then \
		echo "Installing yamlfmt..."; \
		go install github.com/google/yamlfmt/cmd/yamlfmt@latest; \
	fi

ratchet:
	@if ! command -v ratchet >/dev/null 2>&1; then \
		echo "Installing ratchet..."; \
		go install github.com/sethvargo/ratchet@latest; \
	fi
