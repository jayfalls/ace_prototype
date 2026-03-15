#!/bin/bash
#
# .dev/setup.sh
#
# Installs all development dependencies inside the distrobox.
# Run this after first creating the distrobox.
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[SETUP]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

# Check if we're in distrobox
if [ ! -f /run/.toolboxenv ]; then
    echo "This script must be run inside the distrobox"
    echo "Run: distrobox enter opencode"
    exit 1
fi

# Ensure opencode PATH is available
export PATH="$HOME/.opencode/bin:$PATH"

# Check if already set up (idempotency)
log_info "Checking existing installation..."
SKIP_SETUP=true

for cmd in git make go node npm docker opencode; do
    if ! command -v $cmd >/dev/null 2>&1; then
        SKIP_SETUP=false
        break
    fi
done || true

if [ "$SKIP_SETUP" = true ]; then
    log_success "All dependencies already installed!"
    exit 0
fi

log_info "Installing development dependencies..."

# Helper to run dnf with or without sudo
run_dnf() {
    sudo dnf "$@" || dnf "$@"
}

# Update package manager
log_info "Updating package manager..."
run_dnf update -y

# Install core tools
log_info "Installing core tools..."
run_dnf install -y \
    git \
    make \
    curl \
    wget \
    golang \
    nodejs \
    npm \
    python3 \
    python3-pip \
    docker \
    which \
    findutils \
    jq

# Install pipx (needed for distrobox)
log_info "Installing pipx..."
pip3 install --user pipx
pipx ensurepath

# Run go mod tidy for backend dependencies
log_info "Running go mod tidy..."
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [ -d "$REPO_DIR/backend" ]; then
    cd "$REPO_DIR/backend"
    for module in $(go work edit -json 2>/dev/null | jq -r '.Use[] | .DiskPath' 2>/dev/null || echo ""); do
        if [ -d "$module" ] && [ -f "$module/go.mod" ]; then
            log_info "Tidying $module..."
            (cd "$module" && go mod tidy 2>/dev/null || true)
        fi
    done
fi

# Install OpenCode
log_info "Installing OpenCode..."
if ! command -v opencode &> /dev/null; then
    curl -fsSL https://opencode.ai/install | bash
fi
export PATH="$HOME/.opencode/bin:$PATH"
echo "export PATH=\"\$HOME/.opencode/bin:\$PATH\"" >> ~/.bashrc 2>/dev/null || true

# Verify installations
log_info "Verifying tools..."
command -v git >/dev/null 2>&1 && log_success "git"
command -v make >/dev/null 2>&1 && log_success "make"
command -v go >/dev/null 2>&1 && log_success "go"
command -v node >/dev/null 2>&1 && log_success "node"
command -v npm >/dev/null 2>&1 && log_success "npm"
command -v docker >/dev/null 2>&1 && log_success "docker"
command -v opencode >/dev/null 2>&1 && log_success "opencode"

log_success "Development environment ready!"
