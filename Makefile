# KerPlan API — Makefile
# ──────────────────────────────────────

APP_NAME     := kerplan-api
VERSION      := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO           := go
GOFLAGS      := -ldflags "-X github.com/ovander/backendkit/buildinfo.Version=$(VERSION) -X github.com/ovander/backendkit/buildinfo.BuildTime=$(BUILD_TIME) -X github.com/ovander/backendkit/buildinfo.GitCommit=$(GIT_COMMIT)"

DATABASE_URL ?= postgres://kerplan:kerplan@10.10.5.13:5432/kerplan?sslmode=disable

# golang-migrate CLI — used for local dev (make migrate-*).
# Install via: make tools
# The production deploy script uses the binary's built-in sub-command instead:
#   ./kerplan-api migrate
MIGRATE      := migrate -database "$(DATABASE_URL)" -path migrations

.PHONY: help build run test lint clean migrate-up migrate-down migrate-create migrate-version migrate-force migrate-status docker-build docker-run

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

test-integration: ## Run database integration tests (testcontainers, needs Docker; or set TEST_DATABASE_URL)
	# -p 1: with TEST_DATABASE_URL the packages share one database and must not migrate it concurrently.
	go test -tags integration -count=1 -p 1 ./internal/repo/... ./internal/service/...

test-short: ## Run tests without integration tests
	$(GO) test -v -short -race ./...

test-cover: ## Run tests with coverage
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

cover-check: ## Unit tests with coverage, checked against the CI floor (script/coverage-floor.sh)
	$(GO) test -short -race -count=1 -coverprofile=coverage.out ./...
	bash script/coverage-floor.sh coverage.out

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

migrate-up: ## Apply all pending SQL migrations (CLI runner)
	$(MIGRATE) up

migrate-down: ## Roll back one migration
	$(MIGRATE) down 1

migrate-down-all: ## Roll back all migrations (DANGER)
	$(MIGRATE) down

migrate-create: ## Create a new migration pair (usage: make migrate-create name=add_foobar)
	$(MIGRATE) create -ext sql -dir migrations -seq $(name)
	@echo "Created migrations/$$(ls migrations/ | grep -E '[0-9]+_$(name)' | tail -1)"

migrate-version: ## Show current migration version
	$(MIGRATE) version

migrate-status: ## Show pending vs applied migration count
	@echo "Applied:" && $(MIGRATE) version 2>&1 || true
	@echo "Files:  " && ls migrations/*.up.sql | wc -l | tr -d ' ' && echo " total"

migrate-force: ## Mark a specific version as applied without running it (usage: make migrate-force version=12)
	$(MIGRATE) force $(version)

# On an existing database whose schema was created by GORM AutoMigrate and
# whose migrations were applied by hand (no schema_migrations table), mark
# every migration up to and including the current head as applied without
# re-running them. Fresh databases never need this: 000000_baseline.up.sql
# creates the base schema and `migrate-up` applies everything in order.
migrate-baseline: ## Mark all existing migrations as applied (run once on legacy DBs)
	$(MIGRATE) force 15
	@echo "Baseline set to version 15 — only future migrations will be applied"

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
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# ── Cleanup ──────────────────────────

clean: ## Clean build artifacts
	rm -rf bin/ coverage.out coverage.html
	$(GO) clean -cache
