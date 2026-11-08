.PHONY: build test cover vet fmt fmt-check lint tidy check install clean help

BINARY := hackatrak
PREFIX ?= $(HOME)/.local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

# Local tool install location (kept out of the module, see .gitignore).
TOOLBIN := $(CURDIR)/bin
GOLANGCI := $(TOOLBIN)/golangci-lint
GOLANGCI_VERSION ?= v2.15.0

build: ## Build the binary to ./hackatrak
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/hackatrak

test: ## Run the test suite with the race detector
	go test -race ./...

cover: ## Run tests and open an HTML coverage report
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go code
	gofmt -w .

fmt-check: ## Fail if any Go code is not gofmt'd
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt'd:"; echo "$$unformatted"; exit 1; \
	fi

$(GOLANGCI):
	@mkdir -p $(TOOLBIN)
	@echo "Installing golangci-lint $(GOLANGCI_VERSION) into $(TOOLBIN)"
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
		| sh -s -- -b $(TOOLBIN) $(GOLANGCI_VERSION)

lint: $(GOLANGCI) ## Run golangci-lint (installs it locally if missing)
	$(GOLANGCI) run

tidy: ## Tidy and verify go.mod/go.sum
	go mod tidy
	go mod verify

check: fmt-check vet lint test ## Run the full local gate (what CI runs)

install: build ## Install the binary to $(PREFIX)/bin
	install -d $(PREFIX)/bin
	install -m 755 $(BINARY) $(PREFIX)/bin/$(BINARY)

clean: ## Remove build artifacts
	rm -f $(BINARY) coverage.out
	rm -rf $(TOOLBIN)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
