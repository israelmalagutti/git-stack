#!/bin/bash
set -e

# gs installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/israelmalagutti/git-stack/main/scripts/install.sh | bash

REPO="israelmalagutti/git-stack"
BINARY_NAME="gs"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)     OS="linux" ;;
        Darwin*)    OS="darwin" ;;
        MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
        *)          error "Unsupported operating system: $(uname -s)" ;;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   ARCH="amd64" ;;
        arm64|aarch64)  ARCH="arm64" ;;
        *)              error "Unsupported architecture: $(uname -m)" ;;
    esac
}

# Get latest version from GitHub
get_latest_version() {
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        error "Failed to get latest version. Check your internet connection or try again later."
    fi
}

# Download and install
install() {
    detect_os
    detect_arch

    info "Detected: ${OS}/${ARCH}"

    # Allow version override
    if [ -n "$GS_VERSION" ]; then
        VERSION="$GS_VERSION"
        info "Using specified version: ${VERSION}"
    else
        info "Fetching latest version..."
        get_latest_version
        info "Latest version: ${VERSION}"
    fi

    # Construct download URL
    if [ "$OS" = "windows" ]; then
        FILENAME="${BINARY_NAME}-${OS}-${ARCH}.zip"
    else
        FILENAME="${BINARY_NAME}-${OS}-${ARCH}.tar.gz"
    fi

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf ${TMP_DIR}" EXIT

    info "Downloading ${FILENAME}..."
    if ! curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${FILENAME}"; then
        error "Failed to download from ${DOWNLOAD_URL}"
    fi

    info "Extracting..."
    cd "$TMP_DIR"
    if [ "$OS" = "windows" ]; then
        unzip -q "$FILENAME"
    else
        tar -xzf "$FILENAME"
    fi

    # Find the binary
    BINARY=$(find . -name "${BINARY_NAME}*" -type f ! -name "*.tar.gz" ! -name "*.zip" | head -1)
    if [ -z "$BINARY" ]; then
        error "Binary not found in archive"
    fi

    # Install
    info "Installing to ${INSTALL_DIR}..."
    chmod +x "$BINARY"

    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        warn "Need sudo permissions to install to ${INSTALL_DIR}"
        sudo mv "$BINARY" "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    # Verify installation
    if command -v gs &> /dev/null; then
        info "Successfully installed gs $(gs --version 2>/dev/null | head -1 | awk '{print $3}')"
        echo ""
        echo "Run 'gs --help' to get started"
    else
        warn "Installation complete, but 'gs' not found in PATH"
        echo "Add ${INSTALL_DIR} to your PATH if needed"
    fi
}

# Run installer
install
