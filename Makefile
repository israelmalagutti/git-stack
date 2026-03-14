.PHONY: build build-all install clean test release help

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

# Build for all platforms
build-all: clean
	@echo "Building $(BINARY_NAME) $(VERSION) for all platforms..."
	@mkdir -p dist

	@echo "  → linux/amd64"
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 .

	@echo "  → linux/arm64"
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 .

	@echo "  → darwin/amd64"
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 .

	@echo "  → darwin/arm64"
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 .

	@echo "  → windows/amd64"
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe .

	@echo "✓ All binaries built in dist/"

# Create release archives
release: build-all
	@echo "Creating release archives..."
	@cd dist && for f in $(BINARY_NAME)-*; do \
		if [ -f "$$f" ]; then \
			if echo "$$f" | grep -q ".exe"; then \
				zip "$${f%.exe}.zip" "$$f"; \
			else \
				tar -czf "$$f.tar.gz" "$$f"; \
			fi \
		fi \
	done
	@cd dist && sha256sum *.tar.gz *.zip > checksums.txt 2>/dev/null || shasum -a 256 *.tar.gz *.zip > checksums.txt
	@echo "✓ Release archives created with checksums"

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
	@echo "  make build        - Build binary for current platform"
	@echo "  make build-all    - Build for Linux, macOS, Windows (amd64/arm64)"
	@echo "  make release      - Build all + create archives with checksums"
	@echo "  make install      - Build and install to /usr/local/bin"
	@echo "  make uninstall    - Remove from /usr/local/bin"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make test         - Run tests"
	@echo "  make test-coverage- Run tests with coverage report"
	@echo "  make lint         - Run golangci-lint"
	@echo "  make version      - Show version info"
