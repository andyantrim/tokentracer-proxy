.PHONY: help build run test lint lint-fix vet fmt docker-build deps up down logs clean

# Detect docker compose command (prefer plugin, fall back to standalone)
DOCKER_COMPOSE := $(shell if docker compose version > /dev/null 2>&1; then echo "docker compose"; elif command -v docker-compose > /dev/null 2>&1; then echo "docker-compose"; else echo "docker compose"; fi)

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

test-e2e: ## Run Python e2e tests (requires running stack)
	@test -d .venv/e2e || python3 -m venv .venv/e2e
	.venv/e2e/bin/pip install -q -r tests/e2e/requirements.txt
	.venv/e2e/bin/pytest -v tests/e2e/test_api.py

# ---------------------------------------------------------------------------
# Docker
# ---------------------------------------------------------------------------

docker-build: ## Build the Docker image
	docker build -t tokentracer-proxy .

deps: ## Start dependencies only (postgres, redis)
	$(DOCKER_COMPOSE) up -d postgres redis

up: ## Start everything (deps + app)
	$(DOCKER_COMPOSE) --profile full up -d --build

down: ## Stop all containers
	$(DOCKER_COMPOSE) --profile full down

logs: ## Tail logs for all containers
	$(DOCKER_COMPOSE) --profile full logs -f

# ---------------------------------------------------------------------------
# Database
# ---------------------------------------------------------------------------

db-migrate: deps ## Apply schema to the running postgres
	@echo "Waiting for postgres..."
	@until $(DOCKER_COMPOSE) exec postgres pg_isready -U tokentracer > /dev/null 2>&1; do sleep 1; done
	$(DOCKER_COMPOSE) exec -T postgres psql -U tokentracer -d tokentracer < db/schema.sql

# ---------------------------------------------------------------------------
# Housekeeping
# ---------------------------------------------------------------------------

clean: ## Remove build artifacts
	rm -rf bin/

tidy: ## Run go mod tidy
	go mod tidy
