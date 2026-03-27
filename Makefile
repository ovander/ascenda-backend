# KerPlan API — Makefile
# ──────────────────────────────────────

APP_NAME     := kerplan-api
VERSION      := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO           := go
GOFLAGS      := -ldflags "-X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

DATABASE_URL ?= postgres://kerplan:kerplan@localhost:5432/kerplan?sslmode=disable
MIGRATE      := migrate -database "$(DATABASE_URL)" -path migrations

.PHONY: help build run test lint clean migrate-up migrate-down migrate-create docker-build docker-run

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Build & Run ──────────────────────

build: ## Build the binary
	$(GO) build $(GOFLAGS) -o bin/$(APP_NAME) ./cmd/server

run: ## Run the server (development)
	APP_ENV=development $(GO) run $(GOFLAGS) ./cmd/server

run-prod: ## Run with production settings
	APP_ENV=production $(GO) run $(GOFLAGS) ./cmd/server

# ── Testing ──────────────────────────

test: ## Run all tests
	$(GO) test -v -race -count=1 ./...

test-short: ## Run tests without integration tests
	$(GO) test -v -short -race ./...

test-cover: ## Run tests with coverage
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-compute: ## Run only computation engine tests
	$(GO) test -v -race ./internal/compute/...

# ── Code Quality ─────────────────────

lint: ## Run linters
	golangci-lint run ./...

fmt: ## Format code
	$(GO) fmt ./...
	goimports -w .

vet: ## Run go vet
	$(GO) vet ./...

# ── Database Migrations ──────────────

migrate-up: ## Run all pending migrations
	$(MIGRATE) up

migrate-down: ## Roll back one migration
	$(MIGRATE) down 1

migrate-down-all: ## Roll back all migrations (DANGER)
	$(MIGRATE) down

migrate-create: ## Create a new migration (usage: make migrate-create name=add_foobar)
	$(MIGRATE) create -ext sql -dir migrations -seq $(name)

migrate-version: ## Show current migration version
	$(MIGRATE) version

migrate-force: ## Force a specific version (usage: make migrate-force version=5)
	$(MIGRATE) force $(version)

# ── Database ─────────────────────────

db-create: ## Create the database
	createdb kerplan || true

db-drop: ## Drop the database (DANGER)
	dropdb kerplan || true

db-reset: db-drop db-create migrate-up ## Reset database (drop + create + migrate)

db-seed: ## Seed with development data
	$(GO) run ./cmd/seed

# ── Docker ───────────────────────────

docker-build: ## Build Docker image
	docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .

docker-run: ## Run in Docker
	docker run -p 8080:8080 --env-file .env $(APP_NAME):latest

docker-compose-up: ## Start with docker-compose (includes PostgreSQL)
	docker-compose up -d

docker-compose-down: ## Stop docker-compose
	docker-compose down

# ── Dependencies ─────────────────────

deps: ## Download dependencies
	$(GO) mod download

deps-tidy: ## Tidy go.mod
	$(GO) mod tidy

deps-update: ## Update all dependencies
	$(GO) get -u ./...
	$(GO) mod tidy

# ── Tools ────────────────────────────

tools: ## Install development tools
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# ── Cleanup ──────────────────────────

clean: ## Clean build artifacts
	rm -rf bin/ coverage.out coverage.html
	$(GO) clean -cache
