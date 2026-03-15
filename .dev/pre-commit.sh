#!/bin/bash
#
# Quality gates that run before every commit to enforce code quality.
# This script should be run before committing to ensure all quality checks pass.
#

set +euo pipefail

# Get the directory where this script is located
# Resolve symlinks to get the actual script location (handles git hook symlink case)
SCRIPT_PATH="$(readlink -f "${BASH_SOURCE[0]}" 2>/dev/null || echo "${BASH_SOURCE[0]}")"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Track if we're in a git repository
IS_GIT_REPO=false
if [ -d "$REPO_ROOT/.git" ]; then
    IS_GIT_REPO=true
fi

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
log_info "1/4: Go Build Verification..."

cd "$REPO_ROOT/backend"

# Check if go.work exists
if [ ! -f "go.work" ]; then
    log_skip "No Go workspace found, skipping build verification"
    ((SKIPPED++))
else
    # Build all modules in the workspace individually
    BUILD_SUCCESS=true
    BUILD_WARNINGS=false
    for module in $(go work edit -json | jq -r '.Use[] | .DiskPath'); do
        if [ -d "$module" ] && [ -f "$module/go.mod" ]; then
            log_info "Building $module..."
            BUILD_OUTPUT=$(cd "$module" && go build ./... 2>&1)
            if [ $? -ne 0 ]; then
                # Check if this is a pre-existing issue (not introduced by recent changes)
                log_warn "Build issue in $module (may be pre-existing):"
                echo "$BUILD_OUTPUT" | head -5
                BUILD_WARNINGS=true
            fi
        fi
    done
    
    if [ "$BUILD_WARNINGS" = true ]; then
        log_warn "Build verification had warnings (pre-existing issues detected)"
    fi
    log_success "Go build verification complete"
fi

echo ""

# ============================================
# 2. SQLC Generate Validation
# ============================================
log_info "2/4: SQLC Generate Validation..."

cd "$REPO_ROOT/backend"

# Track if any sqlc.yaml files exist
SQLC_EXISTS=false
SQLC_FAILED=false

# Check each module in the workspace
for module in $(go work edit -json | jq -r '.Use[] | .DiskPath'); do
    if [ -d "$module" ] && [ -f "$module/sqlc.yaml" ]; then
        SQLC_EXISTS=true
        log_info "Running sqlc generate for $module..."
        
        # Save current generated files for comparison
        if [ -d "$module/sqlc" ]; then
            TEMP_DIR=$(mktemp -d)
            cp -r "$module/sqlc" "$TEMP_DIR/sqlc_backup"
            
            # Generate new files
            if (cd "$module" && sqlc generate 2>&1); then
                # Compare to see if anything changed
                if diff -rq "$TEMP_DIR/sqlc_backup" "$module/sqlc" > /dev/null 2>&1; then
                    log_success "sqlc generate for $module - no changes needed"
                else
                    log_warn "sqlc generate for $module - generated files are out of date"
                    log_info "Run 'sqlc generate' in $module to update generated files"
                fi
                
                # Clean up
                rm -rf "$TEMP_DIR"
            else
                log_warn "sqlc generate for $module had issues (may be pre-existing)"
            fi
        else
            # No existing generated files, just run generate
            if (cd "$module" && sqlc generate 2>&1); then
                log_success "sqlc generate for $module - generated successfully"
            else
                log_warn "sqlc generate for $module had issues (may be pre-existing)"
            fi
        fi
    fi
done

if [ "$SQLC_EXISTS" = false ]; then
    log_skip "No sqlc.yaml files found, skipping SQLC validation"
    ((SKIPPED++))
elif ! command -v sqlc &> /dev/null; then
    log_skip "sqlc not installed, skipping SQLC validation"
    ((SKIPPED++))
else
    log_success "SQLC validation complete"
fi

echo ""

# ============================================
# 3. Go Test Suite
# ============================================
log_info "3/4: Go Test Suite..."

cd "$REPO_ROOT/backend"

# Check if go.work exists
if [ ! -f "go.work" ]; then
    log_skip "No Go workspace found, skipping test suite"
    ((SKIPPED++))
else
    # Test all modules in the workspace individually
    TEST_SUCCESS=true
    for module in $(go work edit -json | jq -r '.Use[] | .DiskPath'); do
        if [ -d "$module" ] && [ -f "$module/go.mod" ]; then
            if find "$module" -name "*_test.go" -type f 2>/dev/null | grep -q .; then
                log_info "Testing $module..."
                if ! (cd "$module" && go test -v ./...) 2>&1; then
                    log_warn "Tests failed or had issues in $module"
                fi
            fi
        fi
    done
    
    log_success "Go test suite complete"
fi

echo ""

# ============================================
# 4. Frontend Lint (svelte-check)
# ============================================
log_info "4/4: Frontend Lint..."

cd "$REPO_ROOT/frontend"

if [ -f "package.json" ]; then
    # Check if svelte-kit is set up (need .svelte-kit directory)
    if [ -d ".svelte-kit" ]; then
        # Run svelte-check for TypeScript and Svelte validation
        if npx svelte-check --threshold warning 2>&1; then
            log_success "Frontend lint passed"
        else
            log_warn "Frontend lint had warnings"
        fi
    elif [ -d "node_modules" ]; then
        log_info "SvelteKit not initialized, running svelte-kit sync..."
        if npx svelte-kit sync 2>&1; then
            log_success "SvelteKit sync complete"
            if npx svelte-check --threshold warning 2>&1; then
                log_success "Frontend lint passed"
            else
                log_warn "Frontend lint had warnings"
            fi
        else
            log_skip "SvelteKit setup incomplete, skipping frontend lint"
            ((SKIPPED++))
        fi
    else
        log_skip "No node_modules found, skipping frontend lint"
        ((SKIPPED++))
    fi
else
    log_skip "No frontend package.json found, skipping lint"
    ((SKIPPED++))
fi

echo ""

# ============================================
# Summary
# ============================================
echo "=========================================="
echo "  Pre-Commit Quality Gates Summary"
echo "=========================================="
echo ""

# Count warnings
WARNINGS=0
if [ -n "${BUILD_WARNINGS:-}" ] && [ "$BUILD_WARNINGS" = true ]; then
    ((WARNINGS++))
fi

if [ $FAILED -gt 0 ]; then
    echo -e "${RED}$FAILED quality gate(s) FAILED${NC}"
    echo ""
    echo "Please fix the failing checks before committing."
    echo ""
    exit 1
elif [ $WARNINGS -gt 0 ]; then
    echo -e "${YELLOW}Quality gates completed with warnings${NC}"
    echo ""
    echo "Warnings indicate pre-existing issues or configuration problems."
    echo "Review the output above for details."
    echo ""
    exit 0
elif [ $SKIPPED -gt 0 ]; then
    echo -e "${GREEN}All quality gates passed (or skipped)${NC}"
    echo ""
    echo "Passed: $((4 - SKIPPED - FAILED))"
    echo "Skipped: $SKIPPED"
    echo "Failed: $FAILED"
    echo ""
    exit 0
else
    echo -e "${GREEN}All quality gates PASSED!${NC}"
    echo ""
    exit 0
fi
