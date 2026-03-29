.PHONY: build install clean test release-dry-run help

# Binary name
BINARY_NAME=gs
INSTALL_PATH=/usr/local/bin

# Version info from git
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-s -w -X github.com/israelmalagutti/git-stack/cmd.Version=$(VERSION) -X github.com/israelmalagutti/git-stack/cmd.Commit=$(COMMIT) -X github.com/israelmalagutti/git-stack/cmd.BuildDate=$(BUILD_DATE)"

# Build the binary for current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@go build $(LDFLAGS) -o bin/$(BINARY_NAME) .
	@echo "✓ Binary built at bin/$(BINARY_NAME)"

# Dry-run release locally (builds all platforms, creates archives, generates Homebrew formula)
release-dry-run:
	@goreleaser release --snapshot --clean

# Build and install to /usr/local/bin
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@cp bin/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME) 2>/dev/null || \
		(echo "Need sudo permissions..." && sudo cp bin/$(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME))
	@chmod +x $(INSTALL_PATH)/$(BINARY_NAME) 2>/dev/null || sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✓ $(BINARY_NAME) $(VERSION) installed to $(INSTALL_PATH)"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/ dist/
	@go clean
	@echo "✓ Clean complete"

# Go cache locations (override if needed)
GOCACHE ?= /tmp/go-build-cache
GOMODCACHE ?= /tmp/go-mod-cache

# Run tests
test:
	@echo "Running tests..."
	@GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# Lint the code
lint:
	@echo "Linting..."
	@golangci-lint run ./... || echo "Install golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

# Uninstall from /usr/local/bin
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✓ $(BINARY_NAME) uninstalled"

# Show version info
version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(BUILD_DATE)"

# Show help
help:
	@echo "gs $(VERSION) - Makefile targets:"
	@echo ""
	@echo "  make build           - Build binary for current platform"
	@echo "  make install         - Build and install to /usr/local/bin"
	@echo "  make uninstall       - Remove from /usr/local/bin"
	@echo "  make release-dry-run - GoReleaser snapshot (all platforms, no publish)"
	@echo "  make clean           - Remove build artifacts"
	@echo "  make test            - Run tests"
	@echo "  make test-coverage   - Run tests with coverage report"
	@echo "  make lint            - Run golangci-lint"
	@echo "  make version         - Show version info"
