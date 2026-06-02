#!/bin/bash
set -e

# SDD Memory Installation Script
# Usage: curl -sSL https://raw.githubusercontent.com/KevG1t/sdd-memory/main/install.sh | bash

REPO="KevG1t/sdd-memory"
BINARY_NAME="sdd-memory"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
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

# Detect OS and architecture
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case $os in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="macos"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
    
    case $arch in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            log_warning "Architecture $arch may not be supported, trying amd64"
            ARCH="amd64"
            ;;
    esac
    
    if [ "$OS" = "macos" ]; then
        BINARY_SUFFIX=""
        DOWNLOAD_NAME="sdd-memory-macos"
    else
        BINARY_SUFFIX=""
        DOWNLOAD_NAME="sdd-memory-linux"
    fi
    
    log_info "Detected platform: $OS ($arch)"
}

# Get latest release version
get_latest_version() {
    log_info "Fetching latest release information..."
    LATEST_URL="https://api.github.com/repos/$REPO/releases/latest"
    
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -s "$LATEST_URL" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- "$LATEST_URL" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        log_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
    
    if [ -z "$VERSION" ]; then
        log_error "Failed to get latest version"
        exit 1
    fi
    
    log_info "Latest version: $VERSION"
}

# Download and install binary
install_binary() {
    local install_dir="$HOME/.local/bin"
    local tmp_dir="/tmp/sdd-memory-install"
    
    # Create directories
    mkdir -p "$install_dir"
    mkdir -p "$tmp_dir"
    
    local download_url="https://github.com/$REPO/releases/download/$VERSION/$DOWNLOAD_NAME"
    local tmp_file="$tmp_dir/$BINARY_NAME"
    
    log_info "Downloading from: $download_url"
    
    # Download binary
    if command -v curl >/dev/null 2>&1; then
        curl -sSL "$download_url" -o "$tmp_file"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$download_url" -O "$tmp_file"
    else
        log_error "Neither curl nor wget found"
        exit 1
    fi
    
    # Verify download
    if [ ! -f "$tmp_file" ]; then
        log_error "Download failed"
        exit 1
    fi
    
    # Make executable and move to install directory
    chmod +x "$tmp_file"
    mv "$tmp_file" "$install_dir/$BINARY_NAME"
    
    log_success "Binary installed to: $install_dir/$BINARY_NAME"
    
    # Clean up
    rm -rf "$tmp_dir"
}

# Check if binary is in PATH and add if needed
setup_path() {
    local install_dir="$HOME/.local/bin"
    
    if ! echo "$PATH" | grep -q "$install_dir"; then
        log_warning "$install_dir is not in your PATH"
        
        # Add to shell profile
        local shell_profile=""
        if [ -n "$BASH_VERSION" ]; then
            shell_profile="$HOME/.bashrc"
        elif [ -n "$ZSH_VERSION" ]; then
            shell_profile="$HOME/.zshrc"
        else
            shell_profile="$HOME/.profile"
        fi
        
        echo "export PATH=\"\$PATH:$install_dir\"" >> "$shell_profile"
        log_info "Added $install_dir to PATH in $shell_profile"
        log_warning "Please restart your terminal or run: source $shell_profile"
    fi
}

# Verify installation
verify_installation() {
    local install_dir="$HOME/.local/bin"
    local binary_path="$install_dir/$BINARY_NAME"
    
    if [ -x "$binary_path" ]; then
        log_success "Installation completed successfully!"
        log_info "Binary location: $binary_path"
        
        # Try to run version command
        if echo "$PATH" | grep -q "$install_dir" || [ -x "$binary_path" ]; then
            log_info "Testing installation..."
            if "$binary_path" mcp --help >/dev/null 2>&1; then
                log_success "SDD Memory is working correctly"
            else
                log_warning "Binary installed but may not be working correctly"
            fi
        fi
    else
        log_error "Installation failed - binary not found or not executable"
        exit 1
    fi
}

# Main installation process
main() {
    echo "╭─────────────────────────────────────╮"
    echo "│         SDD Memory Installer       │"
    echo "╰─────────────────────────────────────╯"
    echo
    
    detect_platform
    get_latest_version
    install_binary
    setup_path
    verify_installation
    
    echo
    echo "🎉 Installation complete!"
    echo
    echo "Next steps:"
    echo "1. Restart your terminal or run: source ~/.bashrc (or ~/.zshrc)"
    echo "2. Run: sdd-memory"
    echo "3. For IDE integration: sdd-memory mcp"
    echo
    echo "Documentation: https://github.com/$REPO"
}

# Run installer
main "$@"