
.PHONY: all test format lint build install dd-trace-go test-integration \
        dd-trace-go-setup

# Allow overriding via env var `orchestrion_dir` or `ORCHESTRION_DIR`
ORCHESTRION_DIR ?= $(if $(orchestrion_dir),$(orchestrion_dir),$(CURDIR))
DD_TRACE_GO_DIR ?= $(CURDIR)/tmp/dd-trace-go
DDTRACE_INTEGRATION_DIR := $(DD_TRACE_GO_DIR)/internal/orchestrion/_integration

all: build format lint test

.ONESHELL:
test: gotestfmt
	set -euo pipefail
	go test -json -v ./... 2>&1 | tee ./gotest.log | gotestfmt

format:
	golangci-lint fmt

lint:
	golangci-lint run

build:
	go build -o bin/orchestrion main.go

install:
	go install .

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


.ONESHELL:
test-integration: dd-trace-go-setup
	cd $(DDTRACE_INTEGRATION_DIR)
	go run github.com/DataDog/orchestrion go test -v -shuffle=on -failfast ./... | tee $(ORCHESTRION_DIR)/test-integration.log

gotestfmt:
	@if ! command -v gotestfmt >/dev/null 2>&1; then \
		echo "Installing gotestfmt..."; \
		go install github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt@latest; \
	fi
