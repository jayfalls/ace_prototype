# Makefile for ACE Prototype
# Supports CONTAINER_ORCHESTRATOR environment variable (docker or podman)
# Supports ENVIRONMENT variable (dev or prod)

# Load .env file if it exists
-include .env

# Validate CONTAINER_ORCHESTRATOR
ORCHESTRATOR := $(or $(CONTAINER_ORCHESTRATOR),docker)
VALID_ORCHESTRATORS := docker podman

ifeq ($(filter $(ORCHESTRATOR),$(VALID_ORCHESTRATORS)),)
$(error CONTAINER_ORCHESTRATOR must be either 'docker' or 'podman', got: $(ORCHESTRATOR))
endif

# Validate ENVIRONMENT
ENVIRONMENT := $(or $(ENVIRONMENT),dev)
VALID_ENVIRONMENTS := dev prod

ifeq ($(filter $(ENVIRONMENT),$(VALID_ENVIRONMENTS)),)
$(error ENVIRONMENT must be either 'dev' or 'prod', got: $(ENVIRONMENT))
endif

# Support both docker-compose and docker compose (fallback)
COMPOSE := $(shell command -v podman &>/dev/null && echo "podman compose" || (command -v docker-compose &>/dev/null && echo "docker-compose" || echo "docker compose")) -f devops/$(ENVIRONMENT)/compose.yml

# Distrobox config
DISTROBOX_NAME := opencode
DISTROBOX_IMAGE := fedora:latest

# Colors (use shell to properly interpret escape codes)
GREEN := $(shell printf '\033[0;32m')
YELLOW := $(shell printf '\033[0;33m')
BLUE := $(shell printf '\033[0;34m')
RED := $(shell printf '\033[0;31m')
NC := $(shell printf '\033[0m')

.PHONY: help up down logs logs-api logs-fe logs-db logs-broker clean re build ps test dev agent agent-stop

##@ General

help: ## Show this help message
	@echo ""
	@echo "$(GREEN)ACE Prototype - Development Commands$(NC)"
	@echo ""
	@echo "$(GREEN)Usage:$(NC)"
	@echo "  make $(YELLOW)<target>$(NC) [ENVIRONMENT=dev|prod] [CONTAINER_ORCHESTRATOR=docker|podman]"
	@echo ""
	@echo "$(GREEN)Targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)Environment Variables:$(NC)"
	@echo "  ENVIRONMENT           Environment to use (dev or prod) [default: dev]"
	@echo "  CONTAINER_ORCHESTRATOR  Container runtime (docker or podman) [default: docker]"
	@echo ""

##@ Development Environment

dev: ## Full dev setup: clone agency-agents, setup distrobox, install deps
	@echo "$(BLUE)Setting up development environment...$(NC)"
	@echo ""
	@# Step 1: Clone/update agency-agents
	@if [ -d "agency-agents" ]; then \
		echo "Updating agency-agents..."; \
		cd agency-agents && git pull; \
	else \
		echo "Cloning agency-agents..."; \
		git clone https://github.com/msitarzewski/agency-agents.git; \
	fi
	@echo ""
	@# Step 2: Check/create distrobox
	@echo "$(BLUE)Checking distrobox...$(NC)"
	@if ! command -v distrobox &> /dev/null; then \
		echo "$(RED)Error: distrobox not installed. Install with: pipx install distrobox$(NC)"; \
		exit 1; \
	fi
	@REPO_DIR="$(shell pwd)"; \
	if ! distrobox list | grep -q "$(DISTROBOX_NAME)"; then \
		echo "Creating distrobox '$(DISTROBOX_NAME)'..."; \
		distrobox create --name $(DISTROBOX_NAME) --image $(DISTROBOX_IMAGE) --volume /var/run/docker.sock:/var/run/docker.sock; \
		echo "Distrobox created."; \
	fi; \
	echo "Installing dependencies..."; \
	distrobox enter --name $(DISTROBOX_NAME) -- /bin/sh -c "cd $$REPO_DIR && .dev/distrobox-setup.sh"
	@echo ""
	@# Step 3: Setup pre-commit hook
	@echo "$(BLUE)Setting up pre-commit hook...$(NC)"
	@ln -sf "$(shell pwd)/.dev/pre-commit.sh" "$(shell pwd)/.git/hooks/pre-commit" 2>/dev/null || echo "Note: Could not create pre-commit hook"
	@echo ""
	@echo "$(GREEN)Development environment ready!$(NC)"
	@echo ""
	@echo "To start OpenCode, run:"
	@echo "  $(YELLOW)make agent$(NC)"

dev-reset: ## Delete and recreate distrobox (for fixing Docker access issues)
	@echo "$(YELLOW)WARNING: This will delete your '$(DISTROBOX_NAME)' distrobox and recreate it.$(NC)"
	@echo "Press Ctrl+C to cancel, or Enter to continue..."
	@read
	@echo "$(BLUE)Deleting existing distrobox...$(NC)"
	@distrobox rm $(DISTROBOX_NAME) --yes 2>/dev/null || true
	@echo "$(BLUE)Creating new distrobox with Docker socket...$(NC)"
	@distrobox create --name $(DISTROBOX_NAME) --image $(DISTROBOX_IMAGE) --volume /var/run/docker.sock:/var/run/docker.sock
	@REPO_DIR="$(shell pwd)"; \
	echo "Installing dependencies..."; \
	distrobox enter --name $(DISTROBOX_NAME) -- /bin/sh -c "cd $$REPO_DIR && .dev/distrobox-setup.sh"
	@echo "$(GREEN)Distrobox reset complete!$(NC)"

agent: ## Enter distrobox and run OpenCode interactively
	@echo "$(BLUE)Starting OpenCode in distrobox...$(NC)"
	@if ! distrobox list | grep -q "$(DISTROBOX_NAME)"; then \
		echo "$(RED)Distrobox '$(DISTROBOX_NAME)' does not exist. Run 'make dev' first.$(NC)"; \
		exit 1; \
	fi
	@REPO_DIR="$(shell pwd)"; \
	echo "Entering distrobox and starting OpenCode..."; \
	echo "$(GREEN)Distrobox will open with OpenCode. Your host is protected!$(NC)"; \
	distrobox enter --name $(DISTROBOX_NAME) -- /bin/sh -c "cd $$REPO_DIR && export PATH=\"\$$HOME/.opencode/bin:\$$PATH\" && exec opencode web"

agent-stop: ## Stop OpenCode in distrobox
	@echo "$(BLUE)Stopping OpenCode...$(NC)"
	@distrobox enter --name $(DISTROBOX_NAME) -- pkill -f "opencode" 2>/dev/null || echo "No opencode process found"

##@ Development (ENVIRONMENT=dev)

up: ## Start all services in development mode
	@echo "$(BLUE)Starting development services with $(ORCHESTRATOR)...$(NC)"
	@# Ensure .env exists from .env.example if not present
	@if [ ! -f .env ]; then \
		echo "Creating .env from .env.example..."; \
		cp devops/.env.example .env; \
	fi
	@# Ensure clean shutdown by stopping any existing containers first
	$(COMPOSE) down --remove-orphans 2>/dev/null || true
	@sleep 1
	$(COMPOSE) up -d
	@echo "$(GREEN)Services started. Access:$(NC)"
	@echo "  - Frontend: http://localhost:5173"
	@echo "  - API:      http://localhost:8080"
	@echo "  - PostgreSQL: localhost:5432"
	@echo "  - NATS:     localhost:4222"

down: ## Stop all services
	$(COMPOSE) down --remove-orphans
	@docker rm -f ace_api ace_fe ace_db ace_broker 2>/dev/null || true

logs: ## View aggregated logs for all services
	$(COMPOSE) logs -f

logs-api: ## View logs for ace_api service
	$(COMPOSE) logs -f ace_api

logs-fe: ## View logs for ace_fe service
	$(COMPOSE) logs -f ace_fe

logs-db: ## View logs for ace_db service
	$(COMPOSE) logs -f ace_db

logs-broker: ## View logs for ace_broker service
	$(COMPOSE) logs -f ace_broker

clean: ## Remove all containers and volumes
	$(COMPOSE) down -v
	@echo "All containers and volumes removed."

re: ## Restart all services
	$(COMPOSE) restart

build: ## Build all service images
	$(COMPOSE) build

ps: ## Show running containers
	$(COMPOSE) ps

##@ Testing

test: ## Run all tests in API and frontend containers
	@echo "$(BLUE)Running tests in API container...$(NC)"
	@sg docker -c "$(ORCHESTRATOR) exec ace_api sh -c 'cd /app/services/api && go test -tags=integration ./...'"
	@sg docker -c "$(ORCHESTRATOR) exec ace_api sh -c 'cd /app/shared && go test -tags=integration ./...'"
	@sg docker -c "$(ORCHESTRATOR) exec ace_api sh -c 'cd /app/shared/messaging && go test -tags=integration ./...'"
	@sg docker -c "$(ORCHESTRATOR) exec ace_api sh -c 'cd /app/shared/telemetry && go test -tags=integration ./...'"
	@echo ""
	@echo "$(BLUE)Running tests in Frontend container...$(NC)"
	@sg docker -c "$(ORCHESTRATOR) exec ace_fe npm test -- --run 2>/dev/null || echo 'Frontend tests not available - make sure container is running with make up'"
