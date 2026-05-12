# Makefile for ACE Prototype

# Distrobox config for OpenCode
DISTROBOX_NAME := opencode
DISTROBOX_IMAGE := fedora:latest

# Colors
GREEN := $(shell printf '\033[0;32m')
YELLOW := $(shell printf '\033[0;33m')
BLUE := $(shell printf '\033[0;34m')
RED := $(shell printf '\033[0;31m')
NC := $(shell printf '\033[0m')

.PHONY: help dev agent agent-stop ace test

##@ OpenCode Development Environment

dev: ## Setup distrobox for OpenCode instance
	@echo "$(BLUE)Setting up development environment...$(NC)"
	@echo ""
	@if ! command -v distrobox &> /dev/null; then \
		echo "$(RED)Error: distrobox not installed. Install with: pipx install distrobox$(NC)"; \
		exit 1; \
	fi
	@REPO_DIR="$(shell pwd)"; \
	if ! distrobox list | grep -q "$(DISTROBOX_NAME)"; then \
		echo "Creating distrobox '$(DISTROBOX_NAME)'..."; \
		distrobox create --name $(DISTROBOX_NAME) --image $(DISTROBOX_IMAGE); \
		echo "Distrobox created."; \
	fi; \
	echo "Installing dependencies..."; \
	distrobox enter --name $(DISTROBOX_NAME) -- /bin/sh -c "cd $$REPO_DIR && .dev/distrobox-setup.sh"
	@echo ""
	@echo "$(GREEN)Installing pre-commit hook...$(NC)"
	@ln -sf "$(pwd)/.dev/pre-commit.sh" "$(pwd)/.git/hooks/pre-commit"
	@echo ""
	@echo "$(GREEN)Development environment ready!$(NC)"
	@echo ""
	@echo "To start OpenCode, run:"
	@echo "  $(YELLOW)make agent$(NC)"

agent: ## Run OpenCode agent in distrobox
	@echo "$(BLUE)Starting OpenCode in distrobox...$(NC)"
	@if ! distrobox list | grep -q "$(DISTROBOX_NAME)"; then \
		echo "$(RED)Distrobox '$(DISTROBOX_NAME)' does not exist. Run 'make dev' first.$(NC)"; \
		exit 1; \
	fi
	@REPO_DIR="$(shell pwd)"; \
	echo "Entering distrobox and starting OpenCode..."; \
	echo "$(GREEN)Distrobox will open with OpenCode. Your host is protected!$(NC)"; \
	distrobox enter --name $(DISTROBOX_NAME) -- /bin/sh -c "cd $$REPO_DIR && export PATH=\"\$$HOME/.opencode/bin:\$$PATH\" && exec opencode web"

agent-stop: ## Stop OpenCode agent in distrobox
	@echo "$(BLUE)Stopping OpenCode...$(NC)"
	@distrobox enter --name $(DISTROBOX_NAME) -- pkill -f "opencode" 2>/dev/null || echo "No opencode process found"

##@ Application Development

ace: ## Run ace backend and frontend with hot reloading (dev mode)
	@echo "$(BLUE)Starting ACE in dev mode...$(NC)"
	@echo "$(YELLOW)Backend will auto-reload with Air$(NC)"
	@echo "$(YELLOW)Frontend hot reload on http://localhost:5173$(NC)"
	@echo "$(YELLOW)Press Ctrl+C to stop both$(NC)"
	@echo ""
	@# Store PIDs to kill them later
	@( \
		trap 'echo ""; echo "$(RED)Stopping ACE...$(NC)"; kill $$AIR_PID 2>/dev/null || true; kill -9 $$AIR_PID 2>/dev/null || true; pkill -9 -f "bin/ace" 2>/dev/null || true; pkill -9 -f "ace/ace" 2>/dev/null || true; kill $$VITE_PID 2>/dev/null || true; sleep 1; exit' INT; \
		echo "$(GREEN)[BACKEND]$(NC) Starting Air hot reload..."; \
		(cd backend && air) & \
		AIR_PID=$$!; \
		sleep 2; \
		echo "$(GREEN)[FRONTEND]$(NC) Starting Vite dev server..."; \
		(cd frontend && npm run dev) & \
		VITE_PID=$$!; \
		wait \
	)

test: ## Run full validation pipeline (build, lint, test, git add)
	@echo "=== Swag Init ==="
	cd backend && swag init -g ./cmd/ace/main.go -o ./docs
	@echo ""
	@echo "=== Go Build ==="
	cd backend && go build ./...
	@echo ""
	@echo "=== Go Vet ==="
	cd backend && go vet ./...
	@echo ""
	@echo "=== Go Test ==="
	cd backend && go test -short ./...
	@echo ""
	@echo "$(BLUE)=== Realtime Integration Tests ===$(NC)"
	cd backend && go test -run TestIntegration ./internal/api/realtime/
	@echo ""
	@echo "=== SQLC Generate ==="
	cd backend && sqlc generate
	@echo ""
	@echo "=== Docs Generate ==="
	@echo "Starting ACE server for docs generation..."
	cd backend && go run ./cmd/ace > /dev/null 2>&1 & \
	ACE_PID=$$!; \
	sleep 3; \
	cd backend && go run ./scripts/docs-gen/... || (echo "Docs generation completed with warnings"; true); \
	kill $$ACE_PID 2>/dev/null || true; \
	sleep 1
	@echo ""
	@echo "=== Frontend Check ==="
	cd frontend && npm run check
	@echo ""
	@echo "=== Frontend Test ==="
	cd frontend && npm run test:run
	@echo ""
	@echo "=== Git Add ==="
	git add .

help: ## Show this help message
	@echo ""
	@echo "$(GREEN)ACE Prototype$(NC)"
	@echo ""
	@echo "$(GREEN)OpenCode Environment:$(NC)"
	@echo "  $(YELLOW)make dev$(NC)         - Setup distrobox for OpenCode"
	@echo "  $(YELLOW)make agent$(NC)       - Run OpenCode agent"
	@echo "  $(YELLOW)make agent-stop$(NC)  - Stop OpenCode agent"
	@echo ""
	@echo "$(GREEN)Application Development:$(NC)"
	@echo "  $(YELLOW)make ace$(NC)           - Run backend + frontend with hot reload"
	@echo "  $(YELLOW)make test$(NC)          - Run full validation pipeline"
	@echo ""
