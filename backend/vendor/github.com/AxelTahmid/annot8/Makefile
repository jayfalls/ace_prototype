# Load .env file if it exists (local overrides system)
ifneq (,$(wildcard ./.env))
	include .env
	export
endif

.PHONY: help
## help: Display this help message
help:
	@echo "Usage:"
	@echo "  make <target> [variables]"
	@echo ""
	@echo "Available targets:"
	@echo ${MAKEFILE_LIST}
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' |  sed -e 's/^/ /'

# ----------------------------------------------------------------------
# Development Targets
# ----------------------------------------------------------------------
.PHONY: check-env all tidy install test build run clean fmt lint vet

## check-env: Ensure .env exists; if not, copy from .env.example
check-env:
	@test -f .env || cp .env.example .env

## all: Run tidy and build targets
all: tidy build

## tidy: Clean up go.mod and go.sum
tidy:
	@go mod tidy

## install: Download Go module dependencies
install:
	@go mod download

## test: Run all tests with verbose output
test:
	@go test -v ./...

## build: Compile the Go project
build:
	@go build ./...

## fmt: Format Go code using gofmt
fmt:
	@gofmt -s -w .

## lint: Run golint for code style checks
lint:
	@golint ./...

## vet: Run @go vet for static analysis
vet:
	@go vet ./...

## clean: Remove generated files
clean:
	@rm -rf annot8.json

## run: Run the Go application
run:
	@go run .
