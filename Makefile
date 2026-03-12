# Makefile for ACE Prototype
# Supports CONTAINER_ORCHESTRATOR environment variable (docker or podman)

# Validate CONTAINER_ORCHESTRATOR
ORCHESTRATOR := $(or $(CONTAINER_ORCHESTRATOR),docker)
VALID_ORCHESTRATORS := docker podman

ifeq ($(filter $(ORCHESTRATOR),$(VALID_ORCHESTRATORS)),)
$(error CONTAINER_ORCHESTRATOR must be either 'docker' or 'podman', got: $(ORCHESTRATOR))
endif

COMPOSE := $(ORCHESTRATOR) compose
COMPOSE_PROD := $(ORCHESTRATOR) compose -f docker-compose.yml

# Colors
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

.PHONY: help up down logs logs-api logs-fe logs-db logs-broker clean re build ps

##@ Development

help: ## Show this help message
	@echo ""
	@echo "$(GREEN)ACE Prototype - Development Commands$(NC)"
	@echo ""
	@echo "$(GREEN)Usage:$(NC)"
	@echo "  make $(YELLOW)<target>$(NC) [CONTAINER_ORCHESTRATOR=docker|podman]"
	@echo ""
	@echo "$(GREEN)Targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)Environment Variables:$(NC)"
	@echo "  CONTAINER_ORCHESTRATOR  Container runtime (docker or podman) [default: docker]"
	@echo ""

up: ## Start all services
	@echo "Starting services with $(ORCHESTRATOR)..."
	$(COMPOSE_PROD) up -d
	@echo "Services started. Access:"
	@echo "  - Frontend: http://localhost:5173"
	@echo "  - API:      http://localhost:8080"
	@echo "  - PostgreSQL: localhost:5432"
	@echo "  - NATS:     localhost:4222"

down: ## Stop all services
	$(COMPOSE_PROD) down

logs: ## View aggregated logs for all services
	$(COMPOSE_PROD) logs -f

logs-api: ## View logs for ace_api service
	$(COMPOSE_PROD) logs -f ace_api

logs-fe: ## View logs for ace_fe service
	$(COMPOSE_PROD) logs -f ace_fe

logs-db: ## View logs for ace_db service
	$(COMPOSE_PROD) logs -f ace_db

logs-broker: ## View logs for ace_broker service
	$(COMPOSE_PROD) logs -f ace_broker

clean: ## Remove all containers and volumes
	$(COMPOSE_PROD) down -v
	@echo "All containers and volumes removed."

re: ## Restart all services
	$(COMPOSE_PROD) restart

build: ## Build all service images
	$(COMPOSE_PROD) build

ps: ## Show running containers
	$(COMPOSE_PROD) ps
