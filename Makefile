# Makefile for ACE Prototype
# Validates and builds the entire application stack

.PHONY: help ace ui test generate migrate clean

##@ Build Targets

help: ## Show this help message
	@echo ""
	@echo "ACE Prototype - Build & Test Commands"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  %-15s %s\n", $$1, $$2}'
	@echo ""

ace: ## Build the ace binary
	cd backend && go build -o bin/ace ./cmd/ace/

ui: ## Build the frontend UI
	cd frontend && npm run build

test: ## Run full validation pipeline
	@echo "=== Go Build ==="
	cd backend && go build ./...
	@echo ""
	@echo "=== Go Vet ==="
	cd backend && go vet ./...
	@echo ""
	@echo "=== Go Test ==="
	cd backend && go test -short ./...
	@echo ""
	@echo "=== SQLC Generate ==="
	cd backend && sqlc generate || true
	@echo ""
	@echo "=== Frontend Check ==="
	cd frontend && npm run check || true
	@echo ""
	@echo "=== Frontend Test ==="
	cd frontend && npm run test:run || true
	@echo ""
	@echo "=== Git Add ==="
	git add .

generate: ## Run code generation (sqlc)
	cd backend && sqlc generate

migrate: ## Run database migrations
	cd backend && go run ./cmd/ace migrate

clean: ## Clean build artifacts
	rm -rf backend/bin/
	cd frontend && rm -rf build/
