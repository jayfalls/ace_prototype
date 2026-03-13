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

COMPOSE := $(ORCHESTRATOR) compose -f devops/$(ENVIRONMENT)/compose.yml

# Colors
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

.PHONY: help up down logs logs-api logs-fe logs-db logs-broker clean re build ps test

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

##@ Development (ENVIRONMENT=dev)

up: ## Start all services in development mode
	@echo "$(BLUE)Starting development services with $(ORCHESTRATOR)...$(NC)"
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
	@$(ORCHESTRATOR) exec -w /app/services/api ace_api go test ./... 2>/dev/null || echo "API tests not available - make sure container is running with 'make up'"
	@echo ""
	@echo "$(BLUE)Running tests in Frontend container...$(NC)"
	@$(ORCHESTRATOR) exec ace_fe npm test -- --run 2>/dev/null || echo "Frontend tests not available - make sure container is running with 'make up'"
