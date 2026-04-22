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

test: ## Run full validation pipeline
	@echo "$(GREEN)=== Swag Init ===$(NC)"
	cd backend && swag init -g ./cmd/ace/main.go -o ./docs
	@echo ""
	@echo "$(GREEN)=== Go Build ===$(NC)"
	cd backend && go build ./...
	@echo ""
	@echo "$(GREEN)=== Go Vet ===$(NC)"
	cd backend && go vet ./...
	@echo ""
	@echo "$(GREEN)=== Go Test ===$(NC)"
	cd backend && go test -short ./...
	@echo ""
	@echo "$(GREEN)=== SQLC Generate ===$(NC)"
	cd backend && sqlc generate
	@echo ""
	@echo "$(GREEN)=== Docs Generate ===$(NC)"
	@echo "Starting ACE server for docs generation..."
	cd backend && go run ./cmd/ace > /dev/null 2>&1 & \
	ACE_PID=$$!; \
	sleep 3; \
	cd backend && go run ./scripts/docs-gen/... || (echo "Docs generation completed with warnings"; true); \
	kill $$ACE_PID 2>/dev/null || true; \
	sleep 1
	@echo ""
	@echo "$(GREEN)=== Frontend Check ===$(NC)"
	cd frontend && npm run check
	@echo ""
	@echo "$(GREEN)=== Frontend Test ===$(NC)"
	cd frontend && npm run test:run
	@echo ""
	@echo "$(GREEN)=== Git Add ===$(NC)"
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
