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

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if we're in distrobox
if [ ! -f /run/.toolboxenv ]; then
    echo "This script must be run inside the distrobox"
    echo "Run: distrobox enter opencode"
    exit 1
fi

# Ensure opencode PATH is available
export PATH="$HOME/.opencode/bin:$PATH"

# Ensure Go is in PATH
export PATH="/usr/local/go/bin:$HOME/.opencode/bin:$PATH"

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Helper to run dnf with or without sudo
run_dnf() {
    sudo dnf "$@" || dnf "$@"
}

# Install core tools
log_info "Installing core tools..."
run_dnf install -y \
    git \
    make \
    curl \
    wget \
    nodejs \
    npm \
    python3 \
    python3-pip \
    which \
    findutils \
    jq \
    docker \
    podman \
    docker-compose \
    gh

# Add user to docker group for socket access
log_info "Adding user to docker group..."
if ! grep -q "^docker:" /etc/group || ! grep "docker" /etc/group | grep -q "$USER"; then
    sudo usermod -aG docker $USER
    log_success "User added to docker group"
fi

# Install/update system packages
log_info "Updating package manager..."
run_dnf update -y

# Install Go 1.26+ (required by go.work)
log_info "Installing Go 1.26..."
GO_VERSION="1.26.0"
GO_INSTALL_DIR="/usr/local"
if [ ! -d "$GO_INSTALL_DIR/go" ] || ! "$GO_INSTALL_DIR/go/bin/go" version 2>/dev/null | grep -q "go1.2[6-9]"; then
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
    sudo rm -rf $GO_INSTALL_DIR/go
    sudo tar -C $GO_INSTALL_DIR -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
fi
export PATH="/usr/local/go/bin:$PATH"
log_success "Go installed"

# Install docker-compose (fedora uses docker-compose, not docker compose)
log_info "Installing docker-compose..."
if ! command -v docker-compose &> /dev/null; then
    run_dnf install -y docker-compose
fi

# Verify docker-compose works
if command -v docker-compose &> /dev/null; then
    log_success "Docker compose ready"
else
    log_error "Docker compose not available"
fi

# Run go mod tidy for backend (always)
log_info "Running go mod tidy..."
if [ -d "$REPO_DIR/backend" ]; then
    cd "$REPO_DIR/backend"
    for module in $(go work edit -json 2>/dev/null | jq -r '.Use[] | .DiskPath' 2>/dev/null || echo ""); do
        if [ -d "$module" ] && [ -f "$module/go.mod" ]; then
            log_info "Tidying $module..."
            (cd "$module" && go mod tidy)
        fi
    done
fi

# Install Go linting tools
log_info "Installing Go linting tools..."
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install frontend deps (always)
log_info "Setting up frontend dependencies..."
if [ -d "$REPO_DIR/frontend" ]; then
    cd "$REPO_DIR/frontend"
    npm install
    npx svelte-kit sync
    
    # Install additional linting tools
    npm install -D eslint prettier eslint-config-prettier
    
    log_success "Frontend ready"
fi

# Install OpenCode (if not exists)
log_info "Installing OpenCode..."
if ! command -v opencode &> /dev/null; then
    curl -fsSL https://opencode.ai/install | bash
fi
export PATH="$HOME/.opencode/bin:$PATH"
echo "export PATH=\"\$HOME/.opencode/bin:\$PATH\"" >> ~/.bashrc 2>/dev/null || true

log_success "Development environment ready!"
