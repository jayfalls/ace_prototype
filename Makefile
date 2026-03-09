# ACE Framework - Development Makefile

.PHONY: help setup test run migrate db-start db-stop clean

help:
	@echo "ACE Framework - Development Commands"
	@echo ""
	@echo "  make setup       - Install dependencies"
	@echo "  make test        - Run tests"
	@echo "  make run         - Run the server locally"
	@echo "  make migrate     - Run database migrations"
	@echo "  make db-start    - Start PostgreSQL container"
	@echo "  make db-stop     - Stop PostgreSQL container"
	@echo "  make clean       - Clean up build artifacts"

setup:
	cd backend && go mod download
	cd frontend && npm install

test:
	cd backend && go test -v ./...

run:
	cd backend && go run ./cmd/server

migrate-up:
	cd backend && go run -tags migrate github.com/golang-migrate/migrate/v4/cmd/migrate -path db/migrations -database "postgres://ace:ace@localhost:5432/ace_framework?sslmode=disable" up

migrate-down:
	cd backend && go run -tags migrate github.com/golang-migrate/migrate/v4/cmd/migrate -path db/migrations -database "postgres://ace:ace@localhost:5432/ace_framework?sslmode=disable" down

db-start:
	docker run --name ace_postgres -e POSTGRES_USER=ace -e POSTGRES_PASSWORD=ace -e POSTGRES_DB=ace_framework -p 5432:5432 -d postgres:15-alpine

db-stop:
	docker stop ace_postgres || true
	docker rm ace_postgres || true

clean:
	cd backend && go clean
	cd frontend && rm -rf node_modules .svelte-kit

# Generate SQLC code
sqlc:
	cd backend && sqlc generate

# Lint code
lint:
	cd backend && golangci-lint run

# Build for production
build:
	cd backend && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/server ./cmd/server
