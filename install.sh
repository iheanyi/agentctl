#!/bin/bash
# agentctl installer script
# Usage: curl -sSL https://raw.githubusercontent.com/iheanyi/agentctl/main/install.sh | bash

set -e

REPO="iheanyi/agentctl"
INSTALL_DIR="${AGENTCTL_INSTALL_DIR:-$HOME/.local/bin}"
GITHUB_API="https://api.github.com"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

detect_os() {
    case "$(uname -s)" in
        Darwin*) echo "darwin" ;;
        Linux*)  echo "linux" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) log_error "Unsupported OS: $(uname -s)"; exit 1 ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) log_error "Unsupported architecture: $(uname -m)"; exit 1 ;;
    esac
}

get_latest_version() {
    curl -sL "${GITHUB_API}/repos/${REPO}/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

download_binary() {
    local version="$1"
    local os="$2"
    local arch="$3"
    local tmpdir="$4"

    local filename="agentctl_${version#v}_${os}_${arch}.tar.gz"
    if [ "$os" = "windows" ]; then
        filename="agentctl_${version#v}_${os}_${arch}.zip"
    fi

    local url="https://github.com/${REPO}/releases/download/${version}/${filename}"

    log_info "Downloading ${filename}..."
    curl -sL -o "${tmpdir}/${filename}" "$url"

    if [ "$os" = "windows" ]; then
        unzip -q "${tmpdir}/${filename}" -d "${tmpdir}"
    else
        tar -xzf "${tmpdir}/${filename}" -C "${tmpdir}"
    fi
}

install_binary() {
    local tmpdir="$1"
    local binary="agentctl"

    if [ "$(detect_os)" = "windows" ]; then
        binary="agentctl.exe"
    fi

    mkdir -p "$INSTALL_DIR"
    mv "${tmpdir}/${binary}" "$INSTALL_DIR/"
    chmod +x "${INSTALL_DIR}/${binary}"
}

check_path() {
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        log_warning "$INSTALL_DIR is not in your PATH"
        echo ""
        echo "Add this to your shell profile (.bashrc, .zshrc, etc.):"
        echo ""
        echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
        echo ""
    fi
}

main() {
    echo "🚀 agentctl installer"
    echo ""

    local os=$(detect_os)
    local arch=$(detect_arch)

    log_info "Detected: ${os}/${arch}"

    # Get latest version
    local version
    if [ -n "${AGENTCTL_VERSION:-}" ]; then
        version="$AGENTCTL_VERSION"
        log_info "Using specified version: ${version}"
    else
        version=$(get_latest_version)
        if [ -z "$version" ]; then
            log_error "Failed to get latest version. Building from source..."

            if command -v go &> /dev/null; then
                log_info "Installing via go install..."
                go install "github.com/${REPO}/cmd/agentctl@latest"
                log_success "Installed agentctl via go install"
                exit 0
            else
                log_error "Go is not installed. Please install Go or download a release manually."
                exit 1
            fi
        fi
        log_info "Latest version: ${version}"
    fi

    # Create temp directory
    local tmpdir=$(mktemp -d)
    trap "rm -rf ${tmpdir}" EXIT

    # Download and extract
    download_binary "$version" "$os" "$arch" "$tmpdir"

    # Install
    install_binary "$tmpdir"

    log_success "Installed agentctl to ${INSTALL_DIR}/agentctl"
    echo ""

    # Check PATH
    check_path

    # Verify installation
    if command -v agentctl &> /dev/null; then
        echo "Installed version: $(agentctl version)"
    else
        echo "Run 'agentctl version' to verify the installation"
    fi

    echo ""
    echo "🎉 Installation complete!"
    echo ""
    echo "Get started:"
    echo "  agentctl init          # Initialize configuration"
    echo "  agentctl install fs    # Install filesystem MCP server"
    echo "  agentctl sync          # Sync to your tools"
    echo ""
}

main "$@"
