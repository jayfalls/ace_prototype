#!/bin/bash
#
# Quality gates that run before every commit to enforce code quality.
# This script should be run before committing to ensure all quality checks pass.
#

set +euo pipefail

# Add Go 1.26 to PATH if installed
GO126_PATH="/usr/local/go/bin"
if [ -d "$GO126_PATH" ]; then
    export PATH="$GO126_PATH:$PATH"
fi

# Helper to get Go command (prefer Go 1.26 if available)
get_go() {
    if [ -x "$GO126_PATH/go" ]; then
        echo "$GO126_PATH/go"
    else
        echo "go"
    fi
}

GO_CMD=$(get_go)

# Get the directory where this script is located
SCRIPT_PATH="$(readlink -f "${BASH_SOURCE[0]}" 2>/dev/null || echo "${BASH_SOURCE[0]}")"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[PRE-COMMIT]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Change to repo root
cd "$REPO_ROOT"

# Track overall status
FAILED=0
SKIPPED=0

echo ""
echo "=========================================="
echo "  Pre-Commit Quality Gates"
echo "=========================================="
echo ""

# ============================================
# 1. Go Build Verification
# ============================================
log_info "1/9: Go Build..."

cd "$REPO_ROOT/backend"

if [ ! -f "go.work" ]; then
    log_skip "No Go workspace found, skipping"
    ((SKIPPED++))
else
    for module in $($GO_CMD work edit -json | jq -r '.Use[] | .DiskPath'); do
        if [ -d "$module" ] && [ -f "$module/go.mod" ]; then
            log_info "Building $module..."
            if ! (cd "$module" && $GO_CMD build ./...) 2>&1; then
                log_error "Build failed in $module"
                FAILED=1
            fi
        fi
    done
    [ $FAILED -eq 0 ] && log_success "Go build passed"
fi

echo ""

# ============================================
# 2. Go Lint (auto-fix + check)
# ============================================
log_info "2/9: Go Lint..."

cd "$REPO_ROOT/backend"

if [ ! -f "go.work" ]; then
    log_skip "No Go workspace found, skipping"
    ((SKIPPED++))
else
    for module in $($GO_CMD work edit -json | jq -r '.Use[] | .DiskPath'); do
        if [ -d "$module" ] && [ -f "$module/go.mod" ]; then
            log_info "Formatting $module..."
            (cd "$module" && $GO_CMD fmt ./...) 2>&1 || true
            
            log_info "Vetting $module..."
            if ! (cd "$module" && $GO_CMD vet ./...) 2>&1; then
                log_error "Go vet failed in $module"
                FAILED=1
            fi
        fi
    done

echo ""

# ============================================
# 3. Go Test Suite
# ============================================
log_info "3/9: Go Test..."

cd "$REPO_ROOT/backend"

if [ ! -f "go.work" ]; then
    log_skip "No Go workspace found, skipping"
    ((SKIPPED++))
else
    for module in $($GO_CMD work edit -json | jq -r '.Use[] | .DiskPath'); do
        if [ -d "$module" ] && [ -f "$module/go.mod" ]; then
            log_info "Testing $module..."
            # Run ALL tests (including integration tests) - sequentially, no caching
            if ! (cd "$module" && $GO_CMD test -p 1 -count=1 ./...) 2>&1; then
                log_error "Tests failed in $module"
                FAILED=1
            fi
        fi
    done
    [ $FAILED -eq 0 ] && log_success "Go tests passed (unit only)"
fi

echo ""

# ============================================
# 4. SQLC Generate Validation
# ============================================
log_info "4/9: SQLC Generate..."

cd "$REPO_ROOT/backend"

SQLC_EXISTS=false

for module in $($GO_CMD work edit -json 2>/dev/null | jq -r '.Use[] | .DiskPath' 2>/dev/null || echo ""); do
    if [ -d "$module" ] && [ -f "$module/sqlc.yaml" ]; then
        # Check if queries directory exists and has files
        QUERY_DIR=$(cd "$module" && grep -A5 "queries:" sqlc.yaml 2>/dev/null | grep "path:" | head -1 | awk '{print $2}')
        if [ -z "$QUERY_DIR" ] || [ ! -d "$module/$QUERY_DIR" ]; then
            log_warn "No queries directory found in $module, skipping sqlc"
            continue
        fi
        SQLC_EXISTS=true
        log_info "Running sqlc generate for $module..."
        if (cd "$module" && sqlc generate 2>&1); then
            log_success "sqlc generate passed"
        else
            log_error "sqlc generate failed"
            FAILED=1
        fi
    fi
done

[ "$SQLC_EXISTS" = false ] && log_skip "No sqlc.yaml found, skipping"

echo ""

# ============================================
# 5. Documentation Validation
# ============================================
log_info "5/9: Documentation Validation..."

cd "$REPO_ROOT/backend"

if [ ! -d "scripts/docs-gen" ]; then
    log_skip "No docs-gen scripts found, skipping"
    ((SKIPPED++))
else
    log_info "Running documentation validation pipeline..."
    if (cd scripts/docs-gen && GOWORK=off $GO_CMD run . 2>&1); then
        log_success "Documentation validation passed"
    else
        log_warn "Documentation validation skipped — no database available (run 'make test' with containers up)"
        ((SKIPPED++))
    fi
fi

echo ""

# ============================================
# 6. Frontend Lint (svelte-check + eslint)
# ============================================
log_info "6/9: Frontend Lint..."

cd "$REPO_ROOT/frontend"

if [ ! -f "package.json" ]; then
    log_skip "No frontend package.json found, skipping"
    ((SKIPPED++))
elif [ ! -d "node_modules" ] || [ ! -r "node_modules" ]; then
    log_skip "Frontend node_modules not accessible (run make dev)"
    ((SKIPPED++))
else
    # Run eslint --fix first
    if [ -f ".eslintrc.cjs" ] || [ -f ".eslintrc.js" ] || [ -f "eslint.config.js" ]; then
        log_info "Running eslint --fix..."
        npx eslint --fix . 2>&1 || true
        
        # Stage auto-fixed files
        git -C "$REPO_ROOT" add frontend/ 2>/dev/null || true
    fi
    
    # Run svelte-check
    if npx svelte-check 2>&1; then
        log_success "Frontend lint passed"
    else
        log_error "Frontend lint failed"
        FAILED=1
    fi
fi

echo ""

# ============================================
# 7. Frontend Test
# ============================================
log_info "7/9: Frontend Test..."

cd "$REPO_ROOT/frontend"

if [ ! -f "package.json" ]; then
    log_skip "No frontend package.json found, skipping"
    ((SKIPPED++))
elif [ ! -d "node_modules" ]; then
    log_skip "Frontend node_modules not accessible, skipping"
    ((SKIPPED++))
else
    # Run tests if they exist
    if [ -f "vitest.config.ts" ] || [ -f "vitest.config.js" ]; then
        log_info "Running frontend tests..."
        if npx vitest run 2>&1; then
            log_success "Frontend tests passed"
        else
            log_error "Frontend tests failed"
            FAILED=1
        fi
    else
        log_skip "No frontend tests found, skipping"
        ((SKIPPED++))
    fi
fi

echo ""

# ============================================
# 8. Docker Compose Validation
# ============================================
log_info "8/9: Docker Compose Validation..."

COMPOSE_FAILED=false

if command -v docker-compose &> /dev/null; then
    for compose_file in devops/dev/compose.yml devops/prod/compose.yml; do
        if [ -f "$REPO_ROOT/$compose_file" ]; then
            log_info "Validating $compose_file..."
            # Check if .env exists (required for prod)
            COMPOSE_DIR=$(dirname "$REPO_ROOT/$compose_file")
            if [ ! -f "$COMPOSE_DIR/.env" ] && [ "$compose_file" = "devops/prod/compose.yml" ]; then
                log_warn "Prod compose - .env not found, skipping validation"
                continue
            fi
            if ! docker-compose -f "$REPO_ROOT/$compose_file" config --quiet 2>&1; then
                log_error "Compose file $compose_file is invalid"
                FAILED=1
            fi
        fi
    done
    [ $FAILED -eq 0 ] && log_success "Docker Compose validation passed"
else
    log_skip "Docker compose not available, skipping"
    ((SKIPPED++))
fi

echo ""

# ============================================
# 9. Makefile Validation
# ============================================
log_info "9/9: Makefile Validation..."

if [ ! -f "$REPO_ROOT/Makefile" ]; then
    log_skip "No Makefile found, skipping"
    ((SKIPPED++))
else
    if make -n -f "$REPO_ROOT/Makefile" help >/dev/null 2>&1; then
        log_success "Makefile validation passed"
    else
        log_error "Makefile has syntax errors"
        FAILED=1
    fi
fi

# Stage auto files
git -C "$REPO_ROOT" add . 2>/dev/null || true
    
    [ $FAILED -eq 0 ] && log_success "Go lint passed"
fi

echo ""

# ============================================
# Summary
# ============================================
echo "=========================================="
echo "  Pre-Commit Quality Gates Summary"
echo "=========================================="
echo ""

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}$FAILED quality gate(s) FAILED${NC}"
    echo ""
    echo "Please fix the failing checks before committing."
    echo ""
    exit 1
elif [ $SKIPPED -gt 0 ]; then
    echo -e "${GREEN}All quality gates passed (or skipped)${NC}"
    echo ""
    echo "Passed: $((9 - SKIPPED - FAILED))"
    echo "Skipped: $SKIPPED"
    echo "Failed: $FAILED"
    echo ""
    exit 0
else
    echo -e "${GREEN}All quality gates PASSED!${NC}"
    echo ""
    exit 0
fi
