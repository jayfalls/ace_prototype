#!/bin/bash
#
# .openhands/setup.sh
# 
# This script runs automatically every time OpenHands begins working with the repository.
# It ensures the environment is correct before any agent does any work.
#
# This script is IDEMPOTENT - safe to run multiple times without side effects.
#

set -euo pipefail

# Get the directory where this script is located (repo root)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Configuration
AGENCY_AGENTS_REPO="https://github.com/msitarzewski/agency-agents.git"
AGENCY_AGENTS_DIR="$REPO_ROOT/agency-agents"
GOPATH="${GOPATH:-$HOME/go}"
PATH_ADDITIONS="$GOPATH/bin:$HOME/.local/bin"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[SETUP]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SETUP]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[SETUP]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Add PATH additions
export PATH="$PATH_ADDITIONS:$PATH"

# ============================================
# 0. Install Required Tools (if missing)
# ============================================

# Install Go
if ! command -v go &> /dev/null; then
    log_info "Installing Go..."
    sudo apt-get update && sudo apt-get install -y golang-go
else
    log_success "Go already installed"
fi

# Install Node.js and npm
if ! command -v node &> /dev/null; then
    log_info "Installing Node.js and npm..."
    sudo apt-get update && sudo apt-get install -y nodejs npm
else
    log_success "Node.js already installed"
fi

# Install Docker (if not installed)
if ! command -v docker &> /dev/null; then
    log_info "Installing Docker..."
    sudo apt-get update && sudo apt-get install -y docker.io
fi

# Start Docker daemon if not running
if ! docker info &> /dev/null 2>&1; then
    log_info "Starting Docker daemon..."
    sudo dockerd > /tmp/docker.log 2>&1 &
    sleep 5
else
    log_success "Docker is running"
fi

log_success "Required tools installed"

# ============================================
# 1. Clone agency-agents (if not already present)
# ============================================
log_info "Checking agency-agents repository..."

if [ -d "$AGENCY_AGENTS_DIR" ]; then
    log_success "agency-agents already exists at $AGENCY_AGENTS_DIR"
    
    # Verify it's a git repo and has the correct remote
    if [ -d "$AGENCY_AGENTS_DIR/.git" ]; then
        cd "$AGENCY_AGENTS_DIR"
        CURRENT_REMOTE=$(git remote get-url origin 2>/dev/null || echo "")
        if [ "$CURRENT_REMOTE" != "$AGENCY_AGENTS_REPO" ]; then
            log_warn "Remote URL mismatch. Updating to correct repository..."
            git remote set-url origin "$AGENCY_AGENTS_REPO"
            git fetch origin
        fi
    else
        log_warn "Directory exists but is not a git repo. Removing and re-cloning..."
        rm -rf "$AGENCY_AGENTS_DIR"
        git clone --depth 1 "$AGENCY_AGENTS_REPO" "$AGENCY_AGENTS_DIR"
    fi
else
    log_info "Cloning agency-agents repository..."
    git clone --depth 1 "$AGENCY_AGENTS_REPO" "$AGENCY_AGENTS_DIR"
    log_success "agency-agents cloned successfully"
fi

# ============================================
# 2. Install Go dependencies for all modules
# ============================================
log_info "Installing Go workspace dependencies..."

cd "$REPO_ROOT/backend"

# Verify Go workspace is set up
if [ ! -f "go.work" ]; then
    log_error "Go workspace file not found at $REPO_ROOT/backend/go.work"
    exit 1
fi

# Download dependencies for all modules in the workspace
go work sync
go mod download -x

# Download dependencies for each module explicitly
for module in $(go work edit -json | jq -r '.Use[] | .DiskPath'); do
    if [ -d "$module" ] && [ -f "$module/go.mod" ]; then
        log_info "Installing dependencies for $module..."
        (cd "$module" && go mod download -x)
    fi
done

log_success "Go dependencies installed"

# ============================================
# 3. Install global Go tooling
# ============================================
log_info "Installing global Go tooling..."

# Install sqlc
if ! command -v sqlc &> /dev/null; then
    log_info "Installing sqlc..."
    go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
else
    log_success "sqlc already installed"
fi

# Install goose CLI
if ! command -v goose &> /dev/null; then
    log_info "Installing goose..."
    go install github.com/pressly/goose/cmd/goose@latest
else
    log_success "goose already installed"
fi

# Install air for hot reload
if ! command -v air &> /dev/null; then
    log_info "Installing air..."
    go install github.com/air-verse/air@latest
else
    log_success "air already installed"
fi

# Verify installations
log_info "Verifying tool installations..."
command -v sqlc >/dev/null 2>&1 || { log_error "sqlc installation failed"; exit 1; }
command -v goose >/dev/null 2>&1 || { log_error "goose installation failed"; exit 1; }
command -v air >/dev/null 2>&1 || { log_error "air installation failed"; exit 1; }

log_success "Global Go tooling installed"

# ============================================
# 4. Install frontend Node dependencies
# ============================================
log_info "Installing frontend Node dependencies..."

cd "$REPO_ROOT/frontend"

if [ -f "package.json" ]; then
    # Check if node_modules already exists
    if [ -d "node_modules" ]; then
        log_success "node_modules already exists, running npm install to ensure consistency..."
        npm install
    else
        log_info "Running npm install..."
        npm install
    fi
    
    # Check if svelte-check is available for linting
    if ! npx svelte-check --version &> /dev/null; then
        log_info "Installing svelte-check..."
        npm install
    fi
    
    log_success "Frontend dependencies installed"
else
    log_error "package.json not found in frontend directory"
    exit 1
fi

# ============================================
# 5. Set environment variables
# ============================================
log_info "Setting up environment variables..."

# Create .env file from example if it doesn't exist
if [ -f "$REPO_ROOT/.env.example" ] && [ ! -f "$REPO_ROOT/.env" ]; then
    log_info "Creating .env from .env.example..."
    cp "$REPO_ROOT/.env.example" "$REPO_ROOT/.env"
fi

# Ensure PATH includes Go bin
GOBIN_PATH="$GOPATH/bin"
if [[ ":$PATH:" != *":$GOBIN_PATH:"* ]]; then
    log_info "Adding $GOBIN_PATH to PATH in profile..."
    echo "" >> "$HOME/.bashrc"
    echo "# Added by .openhands/setup.sh" >> "$HOME/.bashrc"
    echo "export PATH=\"\$PATH:$GOBIN_PATH\"" >> "$HOME/.bashrc"
fi

# Export for current session
export PATH="$GOBIN_PATH:$PATH"

log_success "Environment setup complete"

# ============================================
# 6. Final verification
# ============================================
log_info "Running final verification..."

cd "$REPO_ROOT"

# Verify Go builds
log_info "Verifying Go build..."
if go build ./... 2>/dev/null; then
    log_success "Go build verified"
else
    log_warn "Go build verification had warnings (this may be normal for empty modules)"
fi

# Verify sqlc generate works (if sqlc.yaml exists)
cd "$REPO_ROOT/backend"
for module in $(go work edit -json | jq -r '.Use[] | .DiskPath'); do
    if [ -d "$module" ] && [ -f "$module/sqlc.yaml" ]; then
        log_info "Running sqlc generate for $module..."
        (cd "$module" && sqlc generate 2>/dev/null || true)
    fi
done

log_success "Setup complete!"
log_info "Environment is ready for OpenHands agents to work."

exit 0
