#!/bin/bash
set -e

# Installs the ServFlow product binary (`servflow`), released from the
# Servflow/servflow-pro repository. The standalone engine published from this
# repository is available as `servflow-engine` on GitHub Releases and Docker
# Hub (servflow/servflow-engine); it has no install script.
REPO="Servflow/servflow-pro"
BINARY_NAME="servflow"
INSTALL_DIR="/usr/local/bin"
TEMP_DIR=$(mktemp -d)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" >&2
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Prints the GoReleaser archive platform token: {Darwin|Linux}_{x86_64|arm64}
detect_platform() {
    local os arch

    case "$OSTYPE" in
        linux*)
            os="Linux"
            ;;
        darwin*)
            os="Darwin"
            ;;
        *)
            print_error "Unsupported operating system: $OSTYPE (prebuilt binaries exist for Linux and macOS)"
            exit 1
            ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)
            arch="x86_64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac

    echo "${os}_${arch}"
}

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

download_and_extract() {
    local version="$1"
    local platform="$2"

    # Archive naming must stay in sync with the `archives` name_template in
    # servflow-pro's .goreleaser.yaml: servflow_{Title Os}_{x86_64|arm64}.tar.gz
    local filename="${BINARY_NAME}_${platform}.tar.gz"
    local download_url="https://github.com/${REPO}/releases/download/${version}/${filename}"
    local temp_archive="${TEMP_DIR}/${filename}"

    print_status "Downloading ${filename}..."

    if ! curl -fsSL "$download_url" -o "$temp_archive"; then
        print_error "Failed to download archive from $download_url"
        exit 1
    fi

    print_status "Extracting archive..."
    tar -xzf "$temp_archive" -C "$TEMP_DIR"

    local extracted_binary
    if [ -f "${TEMP_DIR}/${BINARY_NAME}" ]; then
        extracted_binary="${TEMP_DIR}/${BINARY_NAME}"
    else
        # Fall back to any binary with the expected name prefix
        extracted_binary=$(find "$TEMP_DIR" -name "${BINARY_NAME}*" -type f ! -name "*.tar.gz" | head -1)
        if [ -z "$extracted_binary" ] || [ ! -f "$extracted_binary" ]; then
            print_error "Binary '$BINARY_NAME' not found in extracted archive"
            print_error "Archive contents:"
            ls -la "$TEMP_DIR"
            exit 1
        fi
    fi

    chmod +x "$extracted_binary"
    echo "$extracted_binary"
}

install_binary() {
    local temp_file="$1"
    local install_path="${INSTALL_DIR}/${BINARY_NAME}"

    print_status "Installing ${BINARY_NAME} to ${install_path}..."

    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        if mkdir -p "$INSTALL_DIR" 2>/dev/null; then
            print_status "Created install directory: ${INSTALL_DIR}"
        else
            print_status "Administrator privileges required to create directory"
            if command -v sudo >/dev/null 2>&1; then
                sudo mkdir -p "$INSTALL_DIR"
            else
                print_error "Cannot create directory ${INSTALL_DIR} without sudo"
                exit 1
            fi
        fi
    fi

    if [ -w "$INSTALL_DIR" ]; then
        cp "$temp_file" "$install_path"
    else
        print_status "Administrator privileges required for installation"
        if command -v sudo >/dev/null 2>&1; then
            sudo cp "$temp_file" "$install_path"
        else
            print_error "Cannot install to ${INSTALL_DIR} without sudo"
            print_warning "Please run: cp $temp_file $install_path"
            exit 1
        fi
    fi
}

verify_installation() {
    print_status "Verifying installation..."

    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        local version
        version=$("$BINARY_NAME" --version 2>/dev/null || echo "unknown")
        print_success "ServFlow installed successfully! Version: $version"
    else
        print_warning "Binary installed but not found in PATH"
        print_warning "You may need to restart your terminal or add ${INSTALL_DIR} to PATH"
    fi
}

cleanup() {
    if [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
    fi
}

main() {
    print_status "Starting ServFlow installation..."

    trap cleanup EXIT

    if ! command -v curl >/dev/null 2>&1; then
        print_error "curl is required but not installed"
        exit 1
    fi

    local version="${SERVFLOW_VERSION:-}"
    if [ -z "$version" ]; then
        version=$(get_latest_version)
    fi

    print_status "Installing ServFlow version: $version"

    local platform
    platform=$(detect_platform)
    print_status "Detected platform: $platform"

    if [ -n "${SERVFLOW_INSTALL_DIR:-}" ]; then
        INSTALL_DIR="$SERVFLOW_INSTALL_DIR"
    fi

    local temp_file
    temp_file=$(download_and_extract "$version" "$platform")

    install_binary "$temp_file"
    verify_installation

    print_success "Installation completed successfully!"
    echo
    print_status "Next steps:"
    echo "  1. Run 'servflow --help' to see available commands"
    echo "  2. Run 'servflow start --config config.toml --dashboard' to start the server"
    echo "  3. Visit http://localhost:8080 to access the web interface"
    echo
    print_status "Documentation: https://docs.servflow.io"
}

main "$@"
