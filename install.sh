#!/bin/bash
set -e

# Servflow Installation Script
# Usage: curl -fsSL https://github.com/servflow/servflow/releases/latest/download/install.sh | bash

REPO="servflow/servflow"
BINARY_NAME="servflow"
INSTALL_DIR="/usr/local/bin"
TEMP_DIR=$(mktemp -d)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    local os arch

    case "$OSTYPE" in
        linux*)
            os="linux"
            ;;
        darwin*)
            os="darwin"
            ;;
        msys*|cygwin*|win*)
            os="windows"
            ;;
        *)
            print_error "Unsupported operating system: $OSTYPE"
            exit 1
            ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)
            arch="amd64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        armv7l|armv6l)
            arch="arm"
            ;;
        *)
            print_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac

    echo "${os}-${arch}"
}

# Get latest release version
get_latest_version() {
    print_status "Fetching latest release information..."

    local version
    version=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$version" ]; then
        print_error "Failed to fetch latest release information"
        exit 1
    fi

    echo "$version"
}

# Download binary
download_binary() {
    local version="$1"
    local platform="$2"
    local binary_name="${BINARY_NAME}"

    if [[ "$platform" == *"windows"* ]]; then
        binary_name="${BINARY_NAME}.exe"
    fi

    local filename="${BINARY_NAME}-${platform}"
    if [[ "$platform" == *"windows"* ]]; then
        filename="${filename}.exe"
    fi

    local download_url="https://github.com/${REPO}/releases/download/${version}/${filename}"
    local temp_file="${TEMP_DIR}/${binary_name}"

    print_status "Downloading ${filename} from ${download_url}..."

    if ! curl -fsSL "$download_url" -o "$temp_file"; then
        print_error "Failed to download binary from $download_url"
        exit 1
    fi

    chmod +x "$temp_file"
    echo "$temp_file"
}

# Install binary
install_binary() {
    local temp_file="$1"
    local binary_name="${BINARY_NAME}"

    if [[ "$OSTYPE" == *"windows"* ]]; then
        binary_name="${BINARY_NAME}.exe"
    fi

    local install_path="${INSTALL_DIR}/${binary_name}"

    print_status "Installing ${binary_name} to ${install_path}..."

    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        cp "$temp_file" "$install_path"
    else
        print_status "Administrator privileges required for installation to ${INSTALL_DIR}"
        if command -v sudo >/dev/null 2>&1; then
            sudo cp "$temp_file" "$install_path"
        else
            print_error "Cannot install to ${INSTALL_DIR} without sudo"
            print_warning "Please run: cp $temp_file $install_path"
            exit 1
        fi
    fi
}

# Verify installation
verify_installation() {
    local binary_name="${BINARY_NAME}"

    if [[ "$OSTYPE" == *"windows"* ]]; then
        binary_name="${BINARY_NAME}.exe"
    fi

    print_status "Verifying installation..."

    if command -v "$binary_name" >/dev/null 2>&1; then
        local version
        version=$("$binary_name" --version 2>/dev/null || echo "unknown")
        print_success "Servflow installed successfully! Version: $version"
        print_success "Run '${binary_name} --help' to get started"
    else
        print_warning "Binary installed but not found in PATH"
        print_warning "You may need to add ${INSTALL_DIR} to your PATH or restart your terminal"
    fi
}

# Cleanup
cleanup() {
    if [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
    fi
}

# Main installation process
main() {
    print_status "Starting Servflow installation..."

    # Set trap for cleanup
    trap cleanup EXIT

    # Check dependencies
    if ! command -v curl >/dev/null 2>&1; then
        print_error "curl is required but not installed"
        exit 1
    fi

    # Override version if specified
    local version="${SERVFLOW_VERSION:-}"
    if [ -z "$version" ]; then
        version=$(get_latest_version)
    fi

    print_status "Installing Servflow version: $version"

    # Detect platform
    local platform
    platform=$(detect_platform)
    print_status "Detected platform: $platform"

    # Override install directory if specified
    if [ -n "${SERVFLOW_INSTALL_DIR:-}" ]; then
        INSTALL_DIR="$SERVFLOW_INSTALL_DIR"
    fi

    # Download binary
    local temp_file
    temp_file=$(download_binary "$version" "$platform")

    # Install binary
    install_binary "$temp_file"

    # Verify installation
    verify_installation

    print_success "Installation completed successfully!"
    echo
    print_status "Next steps:"
    echo "  1. Run 'servflow --help' to see available commands"
    echo "  2. Run 'servflow start' to start the server"
    echo "  3. Visit http://localhost:8080 to access the web interface"
    echo
    print_status "Documentation: https://docs.servflow.io"
    print_status "Issues: https://github.com/${REPO}/issues"
}

# Run main function
main "$@"
