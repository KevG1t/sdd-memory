#!/bin/bash
set -e

# SDD Memory Uninstallation Script
# Usage: curl -sSL https://raw.githubusercontent.com/KevG1t/sdd-memory/master/uninstall.sh | bash

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

# Ask for user confirmation
ask_confirmation() {
    local prompt="$1"
    local default="${2:-n}"
    
    while true; do
        if [ "$default" = "y" ]; then
            echo -n "$prompt [Y/n]: "
        else
            echo -n "$prompt [y/N]: "
        fi
        
        read -r response
        response=${response:-$default}
        
        case $response in
            [Yy]|[Yy][Ee][Ss]) return 0 ;;
            [Nn]|[Nn][Oo]) return 1 ;;
            *) echo "Please answer yes or no." ;;
        esac
    done
}

# Find installed binary
find_binary() {
    local binary_path=""
    
    # Check common installation locations
    local locations=(
        "$HOME/.local/bin/$BINARY_NAME"
        "/usr/local/bin/$BINARY_NAME"
        "/usr/bin/$BINARY_NAME"
        "$(which $BINARY_NAME 2>/dev/null || true)"
    )
    
    for location in "${locations[@]}"; do
        if [ -x "$location" ]; then
            binary_path="$location"
            break
        fi
    done
    
    echo "$binary_path"
}

# Remove binary
remove_binary() {
    local binary_path="$1"
    
    if [ -z "$binary_path" ]; then
        log_warning "SDD Memory binary not found in common locations"
        return 0
    fi
    
    log_info "Found binary at: $binary_path"
    
    if ask_confirmation "Remove the SDD Memory binary?"; then
        if rm "$binary_path" 2>/dev/null; then
            log_success "Binary removed: $binary_path"
            return 0
        else
            log_error "Failed to remove binary. You may need sudo privileges:"
            echo "  sudo rm '$binary_path'"
            return 1
        fi
    else
        log_info "Binary left untouched"
        return 0
    fi
}

# Remove data directory
remove_data() {
    local data_dir="$HOME/.sdd-memory"
    
    if [ ! -d "$data_dir" ]; then
        log_info "No data directory found at $data_dir"
        return 0
    fi
    
    local db_size=$(du -sh "$data_dir" 2>/dev/null | cut -f1 || echo "unknown size")
    log_info "Found data directory: $data_dir ($db_size)"
    
    echo "This directory contains:"
    echo "  • Your local database (local.db)"
    echo "  • All stored memories and observations"
    echo "  • Configuration files"
    
    if ask_confirmation "⚠️  Remove all SDD Memory data? This cannot be undone!"; then
        if rm -rf "$data_dir"; then
            log_success "Data directory removed: $data_dir"
        else
            log_error "Failed to remove data directory: $data_dir"
            return 1
        fi
    else
        log_info "Data directory preserved at: $data_dir"
    fi
}

# Clean up shell profile
cleanup_path() {
    local install_dir="$HOME/.local/bin"
    local modified=false
    
    # Check common shell profiles
    local profiles=(
        "$HOME/.bashrc"
        "$HOME/.zshrc"
        "$HOME/.profile"
        "$HOME/.bash_profile"
    )
    
    for profile in "${profiles[@]}"; do
        if [ -f "$profile" ]; then
            if grep -q "$install_dir" "$profile" 2>/dev/null; then
                if ask_confirmation "Remove $install_dir from PATH in $(basename "$profile")?"; then
                    # Create backup
                    cp "$profile" "${profile}.backup.$(date +%Y%m%d_%H%M%S)"
                    
                    # Remove the PATH export line
                    grep -v "export PATH.*$install_dir" "$profile" > "${profile}.tmp" && mv "${profile}.tmp" "$profile"
                    log_success "Cleaned PATH in $(basename "$profile")"
                    modified=true
                fi
            fi
        fi
    done
    
    if [ "$modified" = true ]; then
        log_warning "Please restart your terminal or source your shell profile to update PATH"
    fi
}

# Verify uninstallation
verify_uninstallation() {
    log_info "Verifying uninstallation..."
    
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        log_warning "SDD Memory is still available in PATH"
        local remaining_path=$(which "$BINARY_NAME")
        echo "  Found at: $remaining_path"
        echo "  You may need to remove it manually or restart your terminal"
    else
        log_success "SDD Memory successfully removed from system"
    fi
}

# Main uninstallation process
main() {
    echo "╭─────────────────────────────────────╮"
    echo "│       SDD Memory Uninstaller       │"
    echo "╰─────────────────────────────────────╯"
    echo
    
    log_info "Starting SDD Memory uninstallation..."
    echo
    
    # Find and remove binary
    local binary_path=$(find_binary)
    remove_binary "$binary_path"
    echo
    
    # Remove data directory
    remove_data
    echo
    
    # Clean up PATH
    cleanup_path
    echo
    
    # Verify uninstallation
    verify_uninstallation
    echo
    
    echo "🧹 Uninstallation process complete!"
    echo
    echo "Thank you for using SDD Memory!"
    echo "If you want to reinstall later:"
    echo "  curl -sSL https://raw.githubusercontent.com/KevG1t/sdd-memory/master/install.sh | bash"
}

# Run uninstaller
main "$@"