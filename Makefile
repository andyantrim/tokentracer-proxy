.PHONY: help build run test lint lint-fix vet fmt docker-build deps up down logs clean

# Default target
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# Build & Run
# ---------------------------------------------------------------------------

build: ## Build the binary
	go build -o bin/proxy .

run: ## Run the server locally
	go run .

# ---------------------------------------------------------------------------
# Quality
# ---------------------------------------------------------------------------

test: ## Run all tests
	go test ./... -race -count=1

test-v: ## Run all tests (verbose)
	go test ./... -race -count=1 -v

lint: ## Run golangci-lint
	golangci-lint run ./...

lint-fix: ## Run golangci-lint with auto-fix
	golangci-lint run --fix ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format code
	gofmt -w .

check: vet lint test ## Run vet, lint, and tests

# ---------------------------------------------------------------------------
# Docker
# ---------------------------------------------------------------------------

docker-build: ## Build the Docker image
	docker build -t tokentracer-proxy .

deps: ## Start dependencies only (postgres, redis)
	docker compose up -d postgres redis

up: ## Start everything (deps + app)
	docker compose --profile full up -d --build

down: ## Stop all containers
	docker compose --profile full down

logs: ## Tail logs for all containers
	docker compose --profile full logs -f

# ---------------------------------------------------------------------------
# Database
# ---------------------------------------------------------------------------

db-migrate: deps ## Apply schema to the running postgres
	@echo "Waiting for postgres..."
	@until docker compose exec postgres pg_isready -U tokentracer > /dev/null 2>&1; do sleep 1; done
	docker compose exec -T postgres psql -U tokentracer -d tokentracer < db/schema.sql

# ---------------------------------------------------------------------------
# Housekeeping
# ---------------------------------------------------------------------------

clean: ## Remove build artifacts
	rm -rf bin/

tidy: ## Run go mod tidy
	go mod tidy
