# Ascenda Backend

> AI-powered financial planning backend combining deterministic modelling with AI-driven insights for startups.

Ascenda is an **AI-powered financial planning and decision intelligence platform** designed for startups and growing companies.

This backend provides a multi-tenant, production-grade engine for:
- Financial modelling (P&L, cash flow, balance sheet, WCR)
- Scenario simulation and comparison
- Cap table and financing modelling
- Break-even and unit economics analysis
- AI-powered financial narration and insights

Unlike traditional FP&A tools, Ascenda combines a **deterministic financial engine** with an **AI layer** to help founders and operators not only compute their financials, but also understand, explain and optimise them.

The backend is implemented in Go and exposes a comprehensive REST API designed for scalability, correctness, and SaaS multi-tenancy.

---

## Table of Contents

- [Why Ascenda](#why-ascenda)
- [Positioning](#positioning)
- [What You Can Build](#what-you-can-build-with-ascenda)
- [Core Concepts](#core-concepts)
- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [Environment Variables](#environment-variables)
- [Deployment](#deployment)
- [Database](#database)
- [Running Tests](#running-tests)
- [API Overview](#api-overview)
- [Architecture Notes](#architecture-notes)
- [Makefile Reference](#makefile-reference)
- [Key Features](#key-features)
- [Status](#status)

---

## Why Ascenda

Traditional financial tools are either:

- Spreadsheet-based → flexible but error-prone and non-scalable
- Enterprise FP&A tools → powerful but complex and inaccessible to startups

Ascenda aims to bridge this gap by providing:

- A deterministic financial engine
- A structured data model
- AI-assisted interpretation and insights

This enables faster, more reliable financial decision-making for founders and operators.

---

## Positioning

Ascenda is positioned as a **Financial Operating System** for startups:

- Not just reporting → **decision-making**
- Not just spreadsheets → **structured models**
- Not just dashboards → **actionable insights**

It bridges the gap between:

- CFO-grade financial modelling
- Founder-level usability
- AI-assisted interpretation

---

## What You Can Build with Ascenda

- SaaS financial planning platforms
- Investor-ready financial models
- Scenario simulation tools
- AI-assisted financial analysis workflows
- Internal FP&A systems for startups

---

## Core Concepts

Ascenda is built around a few core principles:

- **Single Source of Truth** — All financial outputs are derived from a consistent, deterministic compute engine.
- **Separation of Input and Computation** — User inputs are stored independently from computed projections, enabling reproducibility and scenario comparison.
- **AI as a Layer, not a Replacement** — AI augments the financial engine (narration, insights, validation) but never replaces deterministic calculations.
- **Multi-Tenant by Design** — All data is scoped by tenant, enabling SaaS deployment with strict isolation.
- **Composable Financial Models** — P&L, cash flow, cap table, BEP and graphs are computed as composable modules.

---

## Tech Stack

| Concern | Library / Tool |
|---|---|
| Language | Go 1.27 (toolchain pinned in go.mod) |
| HTTP router | [chi v5](https://github.com/go-chi/chi) |
| ORM | [GORM](https://gorm.io) + PostgreSQL driver |
| Database | PostgreSQL 16 |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Auth | JWT ([golang-jwt/jwt v5](https://github.com/golang-jwt/jwt)) + Socrate OAuth2 |
| Decimal arithmetic | [shopspring/decimal](https://github.com/shopspring/decimal) |
| Validation | [go-playground/validator v10](https://github.com/go-playground/validator) |
| Logging | [logrus](https://github.com/sirupsen/logrus) |
| Metrics | [Prometheus client](https://github.com/prometheus/client_golang) |
| Rate limiting | [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate) (per-tenant token-bucket) |
| Testing | [testify](https://github.com/stretchr/testify) + [testcontainers-go](https://golang.testcontainers.org/) |
| Live reload (dev) | [air](https://github.com/air-verse/air) |

---

## Project Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go          # Entry point: config, bootstrap, graceful shutdown
├── internal/
│   ├── compute/             # Pure computation engine (no I/O)
│   ├── config/              # Configuration loading from env
│   ├── dto/                 # Request/response Data Transfer Objects
│   ├── event/               # Async event system (audit log, cache invalidation)
│   ├── handler/             # HTTP handlers (one file per domain)
│   ├── middleware/          # Auth, rate limiting, CORS, timeouts
│   ├── model/               # Domain models and computed output structs
│   ├── pkg/                 # Shared utilities (apierror, ctxutil, etc.)
│   ├── repo/                # Database repositories (GORM)
│   ├── router/              # Chi router setup and route registration
│   └── service/             # Business logic layer
├── migrations/              # SQL migration files (up/down)
├── docs/                    # Additional documentation
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── go.mod
└── .env.example
```

### Layered Architecture

```
HTTP Request
    │
    ▼
Middleware (auth, rate limit, timeout, CORS)
    │
    ▼
Handler (parse, validate, call service)
    │
    ▼
Service (business logic, orchestration)
    │
    ▼
Repo  ◄──────► PostgreSQL
    │
    ▼
Compute (pure functions, no DB access)
```

The `compute` package contains all financial modelling logic as pure functions operating on `model.FullPlanOutput`. It has no database dependencies and is fully unit-testable in isolation.

---

## Getting Started

### Prerequisites

- Go 1.21 or later — the `toolchain go1.27.1` line in go.mod downloads Go 1.27.1 automatically
- PostgreSQL 16
- [golang-migrate CLI](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) (for manual migration commands)

### Install dependencies and tools

```bash
make deps
make tools   # installs golangci-lint, goimports, migrate
```

### Configure environment

```bash
cp .env.example .env
# Edit .env with your local database URL and, to sign in, the Socrate settings.
```

`.env.example` lists the main variables of the table below with development values; `.env` is
git-ignored and never committed.

### Set up the database

```bash
make db-create       # creates the ascenda database
make migrate-up      # runs all pending migrations
make db-seed         # (optional) seeds development data
```

### Run the server

```bash
make run             # development mode (uses air for live reload)
make run-prod        # production mode
```

The server listens on port `8080`: on every interface in development, on `127.0.0.1` in production (override with `PORT=` and `BIND_ADDRESS=`).

### Docker

```bash
make docker-compose-up     # starts API + PostgreSQL via docker-compose
make docker-compose-down   # stops everything
```

---

## Environment Variables

| Variable | Description | Default / Example |
|---|---|---|
| `APP_ENV` | Environment (`development`, `staging`, `production`) | `development` |
| `APP_VERSION` | Application version string | `0.1.0` |
| `PORT` | HTTP listen port | `8080` |
| `BIND_ADDRESS` | Interface to listen on. In production the default is `127.0.0.1`: Caddy on the same host proxies to the API, and nothing else can reach it. Elsewhere the default is every interface. The Docker image sets `0.0.0.0` | `127.0.0.1` in production, else every interface |
| `LOG_LEVEL` | Log verbosity (`debug`, `info`, `warn`, `error`) | auto from `APP_ENV` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://ascenda:ascenda@localhost:5432/ascenda?sslmode=disable` |
| `DB_AUTO_MIGRATE` | Run migrations on startup | `true` |
| `TRUSTED_PROXY_CIDRS` | Reverse proxies whose `X-Forwarded-For` / `X-Real-IP` are trusted to give the client IP, used for rate limiting and sent to Socrate as the browser's address on sign-in calls (comma-separated CIDRs or IPs) | `127.0.0.1/32,::1/128` |
| `TENANT_DEFAULT_FALLBACK` | Provision users with no tenant claim and no user record into the seeded default workspace. **Development only** — the server refuses to start in production when enabled. | `true` in `development`, else `false` |
| `DB_MAX_OPEN_CONNS` | Max open DB connections | `25` |
| `DB_MAX_IDLE_CONNS` | Max idle DB connections | `5` |
| `DB_CONN_MAX_LIFETIME` | Connection max lifetime (seconds) | `3600` |
| `DB_CONN_MAX_IDLE_TIME` | Connection max idle time (seconds) | `300` |
| `CORS_ORIGINS` | Allowed CORS origins (comma-separated) | `http://localhost:5173` |
| `MAX_REQUEST_BODY_BYTES` | Max request body size | `1048576` (1 MB) |
| `SOCRATE_BASE_URL` | Socrate's public URL, which is also the issuer every access token must carry (`iss`, compared exactly: **no trailing slash**). Required in production | `https://socrate.vandermoten.eu` |
| `SOCRATE_INTERNAL_URL` | Optional. Where this server calls Socrate's OAuth endpoints (token, revoke, userinfo, magic link) instead of `SOCRATE_BASE_URL`: Socrate's loopback address when both run on the same host, so Socrate sees the browser's address (it trusts `X-Forwarded-For` only from loopback). The issuer and JWKS stay public | `http://127.0.0.1:8080` |
| `SOCRATE_ADMIN_URL` | Socrate's admin API (registration, invitations, admin user management, magic-link e-mails). It listens on loopback on the Socrate VPS only; the apps VPS reaches it through the host's SSH tunnel `socrate-admin-tunnel.service`. **Never derived**: required whenever `SOCRATE_BASE_URL` is set, in every environment (backendkit would otherwise guess `<base host>:8081`, a different service). No fallback: when the tunnel is down, admin calls fail. See [Deployment](#deployment) | `http://127.0.0.1:18082` |
| `SOCRATE_CLIENT_ID` | OAuth2 client ID | — |
| `SOCRATE_CLIENT_SECRET` | OAuth2 client secret | — |
| `SOCRATE_JWKS_URL` | JWKS endpoint for JWT validation (RS256) | `https://socrate.vandermoten.eu/.well-known/jwks.json` |
| `SOCRATE_REDIRECT_URL` | OAuth2 redirect URI, registered at Socrate exactly (no wildcards). The SPA sends its own at `/auth/callback`; this is the fallback. Must be an absolute URL in production | `https://ascenda.vandermoten.eu/callback` |
| `SOCRATE_VERIFY_AUDIENCE` | Require `SOCRATE_CLIENT_ID` in the access token's `aud` claim, so tokens issued for other applications of the same Socrate are rejected. Set `false` only for an IdP that does not set `aud` | `true` |
| `AI_PROVIDER` | AI backend (`claude` or `openai`) | `claude` |
| `AI_API_KEY` | API key for the AI provider | — |
| `AI_MODEL` | Model identifier | `claude-sonnet-4-6` |
| `AI_MAX_TOKENS` | Max tokens per AI response | `4096` |
| `AI_TEMPERATURE` | Sampling temperature | `0.3` |
| `AI_TIMEOUT_SECONDS` | AI call timeout | `30` |
| `AI_CACHE_ENABLED` | Cache AI responses | `true` |

---

## Deployment

Production runs on the apps VPS (135.125.107.71), next to other applications; Socrate runs on
its own VPS. The owner deploys a tag with `script/push.sh`, which uploads the binary and runs
`script/deploy-backend.sh` there (systemd service `ascenda`, env file
`/opt/apps/ascenda/env/.env`).

- **API**: listens on loopback; Caddy on the same host proxies `api.ascenda.vandermoten.eu` to it.
- **OAuth calls** (token, refresh, revoke, userinfo) and the JWKS go to Socrate's public URL,
  `SOCRATE_BASE_URL`, which is also the issuer.
- **Admin API**: Socrate's admin API is bound to loopback on the Socrate VPS and is never
  exposed publicly. The apps VPS reaches it through an SSH tunnel installed on the host,
  `socrate-admin-tunnel.service`, shared by every application there and not part of this
  repository. Ascenda only uses `SOCRATE_ADMIN_URL=http://127.0.0.1:18082`.

  **Dependency:** when the tunnel is down, every admin call fails: self-service registration,
  workspace invitations, platform-admin user management and magic-link e-mails (magic-link
  sign-in itself, the token redemption, uses the public URL). There is no fallback URL and no
  public admin URL. Check the tunnel with `systemctl status socrate-admin-tunnel` on the apps VPS.

## Database

Migrations live in `migrations/` as numbered SQL files (`NNNN_name.up.sql` / `NNNN_name.down.sql`) and are embedded in the binary. **They are the only source of truth for the schema**: `000000_baseline.up.sql` creates every base table (generated from the GORM models), and `000001` onwards evolve it. A fresh database is fully provisioned by `./ascenda-api migrate` (or `make migrate-up`) with no AutoMigrate step.

`DB_AUTO_MIGRATE` (default `true` in development only) additionally runs GORM AutoMigrate after the SQL migrations. It is a convenience for local model tinkering; any schema change that must reach staging or production needs a migration file, and the repository integration tests (`make test-integration`) provision their database from the migrations alone, so a model that drifts from them fails CI. `TestSchema_MigrationsMatchModels` makes that explicit: on a freshly migrated database, AutoMigrate over every model in `model.TableModels()` must not issue a single DDL statement, and the failure message prints the statements it would run. New table models go in that list.

Existing databases are unaffected by the baseline: golang-migrate only applies it below version 1, and every deployment is at version 15 or later.

```bash
make migrate-up                        # apply all pending
make migrate-down                      # roll back one step
make migrate-down-all                  # roll back everything (DANGER)
make migrate-create name=add_foobar    # scaffold a new migration
make migrate-version                   # show current version
make db-reset                          # drop + create + migrate (DANGER)
```

### Schema overview

The database is organised around four top-level concepts:

- **Tenants** — isolated organisations; all business data is scoped to a `tenant_id`
- **Users** — belong to one tenant; carry a role (`owner`, `admin`, `editor`, `viewer`) and a subscription tier (`starter`, `standard`, `pro`, `enterprise`)
- **Plans** — a named financial model belonging to a tenant; has members with per-plan roles
- **Scenarios** — variants within a plan (e.g. base case, optimistic); all input and output data is scoped to `(tenant_id, scenario_id)`

Key tables: `tenants`, `users`, `plans`, `plan_members`, `scenarios`, `products`, `staff_headcounts`, `staff_salaries`, `staff_incentives`, `capex_entries`, `opex_entries`, `pnl_entries`, `fiplan_entries`, `wcr_entries`, `cash_overrides`, `budget_overrides`, `bep_snapshots`, `cap_table_*`, `scenario_snapshots`, `audit_log`, `ai_usage`.

Deletes cascade through the hierarchy (migration `000016`): removing a plan removes its scenarios and members; removing a scenario removes every row keyed on it (settings, products and their assumptions, staff, entries, snapshots, BEP snapshots with their cost lines, optimisation plans and savings, cap-table rounds, shareholders, positions, option plans and grants). `audit_logs` is deliberately not linked, so history outlives its subject.

---

## Running Tests

```bash
make test            # all tests with race detector
make test-short      # unit tests only (skips integration / testcontainers)
make test-cover      # coverage report → coverage.html
make test-compute    # compute engine tests only
```

Integration tests live in `internal/repo` (repositories) and `internal/service` (snapshot restore) behind the `integration` build tag; the shared `internal/testdb` helper uses [testcontainers-go](https://golang.testcontainers.org/) to spin up a real PostgreSQL instance — no manual setup beyond a running Docker daemon. Without Docker, point them at any empty PostgreSQL database with `TEST_DATABASE_URL=postgres://…` instead. Either way the schema is provisioned from the embedded SQL migrations. Run them with `make test-integration`; `make test` / `make test-short` do not include them.

### Continuous integration

`.github/workflows/ci.yml` runs on every pull request and push to `main`:

| Job | What it runs | Gate |
|---|---|---|
| Build, vet, unit tests | `go build`, `go vet`, `go test -short -race` | blocking |
| golangci-lint | `.golangci.yml` (standard linters); pull requests fail only on **new** issues (`--new-from-rev`), pushes to `main` report the full backlog | blocking on PRs |
| gofmt | the whole tree must be gofmt-clean (`gofmt -l .` empty); `.git-blame-ignore-revs` hides the one-off reformat from `git blame` | blocking |
| Integration tests | `go test -tags integration ./internal/repo/... ./internal/service/...` (testcontainers) | blocking |
| govulncheck | reachable vulnerabilities in dependencies | blocking |
| Docker image | `docker build` of the Dockerfile (no push) | blocking |

Handler tests use hand-rolled mocks (no code-gen required) and `httptest.NewRecorder`. Each domain's handler test file defines a `Mock<Domain>Service` struct that satisfies the handler's service interface.

---

## API Overview

All authenticated endpoints are prefixed with `/api/v1`. JWT bearer token required, obtained at `/auth/callback` (authorization code with PKCE S256, started by the SPA) or `/auth/magic-link/verify`.

### Authentication

| Method | Path | Description |
|---|---|---|
| POST | `/auth/register` | Self-service registration |
| POST | `/auth/callback` | Exchange the authorization code and PKCE verifier for tokens |
| POST | `/auth/refresh` | Refresh access token |
| POST | `/auth/logout` | Revoke session |
| POST | `/auth/magic-link` | Ask Socrate to e-mail a sign-in link (always 202) |
| POST | `/auth/magic-link/verify` | Redeem the token from the link at Socrate; returns tokens like `/auth/callback` |

### Health & Observability

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Liveness check |
| GET | `/ready` | Readiness check |
| GET | `/metrics` | Prometheus metrics (if `METRICS_ENABLED=true`): Go/process defaults plus `ascenda_event_inline_dispatch_total{reason}`, `ascenda_event_subscriber_panics_total{mode}` and `ascenda_audit_write_failures_total`, all expected to stay at 0 |
| GET | `/api/v1/version` | Version info |

### Plans & Scenarios

```
GET  /api/v1/plans
POST /api/v1/plans
GET  /api/v1/plans/{planId}
PUT  /api/v1/plans/{planId}
DELETE /api/v1/plans/{planId}

GET  /api/v1/plans/{planId}/scenarios
POST /api/v1/plans/{planId}/scenarios
GET  /api/v1/plans/{planId}/scenarios/{scenarioId}
PUT  /api/v1/plans/{planId}/scenarios/{scenarioId}
DELETE /api/v1/plans/{planId}/scenarios/{scenarioId}
POST /api/v1/plans/{planId}/scenarios/{scenarioId}/clone
```

### Financial Inputs (all under `/api/v1/plans/{planId}/scenarios/{scenarioId}/`)

| Resource | Endpoints |
|---|---|
| `products` | CRUD + assumptions, volumes, margins, revenue |
| `staff/headcounts` | GET/PUT + payroll summary |
| `staff/salaries` | GET/PUT |
| `staff/incentives` | GET/PUT |
| `capex` | GET/PUT + summary |
| `opex` | GET/PUT + summary |
| `pnl` | GET/PUT |
| `fiplan` | GET/PUT |
| `pnl-cash` | GET/PUT |
| `wcr` | GET/PUT |
| `cash` | GET/PUT |
| `budget` | GET/PUT |

### Computed Reports (all read-only, 30s timeout, 10 req/s rate limit)

| Route | Description |
|---|---|
| `/pnl/report` | Profit & Loss statement (5 annual years) |
| `/fiplan/report` | Financial plan report |
| `/pnl-cash/report` | P&L + cash flow combined |
| `/bsheet/report` | Balance sheet |
| `/ratios/report` | Financial ratios |
| `/wcr/report` | Working capital requirement |
| `/cash/report` | Cash flow statement |
| `/budget/year1`, `/budget/year2` | Monthly budget vs. actuals |
| `/report` | Full consolidated financial report |
| `/graphs/annual/all` | All 7 annual chart datasets in one call |
| `/graphs/annual?name=<chart>` | Single annual chart |
| `/graphs/monthly?name=<chart>` | Single monthly chart |

**Annual chart names:** `sales-analysis`, `cost-structure`, `revenue-profit-cash`, `requirements-vs-cash`, `balance-sheet-structure`, `headcount-annual`, `pnl-cascade`

**Monthly chart names:** `cash-equity-debt`, `operating-cash-flows`, `invoicing-ebitda`, `headcount`

### Cap Table (Pro tier)

Full shareholder registry, funding rounds, stock option plans, grants, FastValo valuation scenarios, dilution waterfall, and InGeFiE / matrix reports.

```
GET  /api/v1/plans/{planId}/scenarios/{scenarioId}/cap-table/report
GET  /api/v1/plans/{planId}/scenarios/{scenarioId}/cap-table/report/waterfall
GET  /api/v1/plans/{planId}/scenarios/{scenarioId}/cap-table/report/valuation
...
```

### Break-Even Analysis (Pro tier)

Snapshot-based BEP analysis with fixed/variable cost classification, sensitivity analysis, and optimisation plans.

```
GET  /api/v1/plans/{planId}/scenarios/{scenarioId}/bep/snapshots
POST /api/v1/plans/{planId}/scenarios/{scenarioId}/bep/snapshots
GET  /api/v1/plans/{planId}/scenarios/{scenarioId}/bep/snapshots/{snapshotId}/report
...
```

### AI Narration (tier-gated)

| Endpoint | Minimum Tier |
|---|---|
| `/ai/narrate` | Standard |
| `/ai/unit-economics` | Pro |
| `/ai/assumption-review` | Pro |
| `/ai/investor-memo` | Enterprise (owner only) |
| Other `/ai/*` endpoints | Pro |

### Snapshots & Audit

```
POST /api/v1/plans/{planId}/scenarios/{scenarioId}/snapshots          # create point-in-time snapshot
POST .../snapshots/{snapshotId}/restore                               # restore scenario to snapshot
GET  .../snapshots/{snapshot1Id}/diff/{snapshot2Id}                   # diff two snapshots

GET  /api/v1/audit                                                    # audit trail
POST /api/v1/audit/export                                             # record export event
```

---

## Architecture Notes

### Multi-tenancy

Every database query is scoped to a `tenant_id` stored in the request context via `ctxutil.WithTenantID` / `ctxutil.GetTenantID`. The tenant middleware (`internal/middleware/tenant.go`) resolves it in this order, with the Ascenda `users` record as the source of truth:

1. JWT role `admin` → platform operator, no tenant context.
2. A `users` record matching the JWT subject → its `tenant_id`, role and plan apply. A `tenant_id` claim that disagrees with the record is logged and ignored. Deactivated users get `403`.
3. No record, but a pending invitation (empty `external_id`) matches the user's e-mail → the invitation is claimed.
4. No record, but the JWT carries a `tenant_id` of an existing tenant → the user is provisioned there (first member becomes `owner`, others `editor`).
5. Otherwise the request is rejected with `403` (`account is not linked to a workspace`). Only when `TENANT_DEFAULT_FALLBACK=true` (development) is the user provisioned into the seeded default workspace instead.

### Rate Limiting

Keyed token-bucket limiters (`internal/middleware/ratelimit.go`) protect four groups of routes. A limiter never passes a request through unkeyed: when its preferred key is missing it falls back to the next one, ending with the client IP.

| Routes | Key | Limit |
|---|---|---|
| `/auth/*` | client IP | 20 req/s, burst 10 |
| `POST /auth/register`, `POST /auth/magic-link`, `POST /auth/magic-link/verify` | client IP | 1 req / 5 s, burst 5 |
| `/api/v1/*` (authenticated) | user (JWT subject) | 100 req/s, burst 20 |
| reports, graphs, cap-table/BEP reports, `/ai/*` | tenant → user → IP | 10 req/s, burst 5 |

The client IP is the TCP peer unless the peer is listed in `TRUSTED_PROXY_CIDRS` (default: loopback, i.e. a reverse proxy on the same host), in which case the rightmost non-proxy `X-Forwarded-For` entry (or `X-Real-IP`) is used. Idle buckets are evicted after 10 minutes to bound memory usage.

The `/graphs/annual/all` batch endpoint was specifically introduced so that the graphs dashboard can fetch all 7 annual charts in a single HTTP request — avoiding burst exhaustion when charts were previously fetched in parallel.

### Computation Engine

All financial projections are calculated in `internal/compute/` as stateless pure functions. The entry point is typically:

```go
full, _ := reportService.GetFullReport(ctx, tenantID, scenarioID)
graphs  := compute.ComputeGraphs(full)    // annual chart data
graphs2 := compute.ComputeGraphs2(full)   // monthly chart data
```

All monetary values use `shopspring/decimal` throughout the compute layer to avoid floating-point rounding errors. Values are only converted to `float64` at the JSON serialisation boundary.

### Permission Model

Three levels of access control are enforced:

1. **Platform role** — `platform:admin` for cross-tenant admin routes
2. **Tenant role** — `owner`, `admin`, `editor`, `viewer` for tenant-level operations
3. **Plan membership** — users must be a member of a plan (`editor` or `viewer`) to access plan-scoped routes; tenant owners have access to every plan in their tenant. Demo plans are a shared sandbox: every tenant user gets `editor` access to them without membership (`reader` stays read-only), and owners can reset them via `POST /plans/reset-demo`.
4. **Scenario binding** — `PlanAccessMiddleware.RequireScenarioInPlan` verifies that `{scenarioId}` belongs to `{planId}` before any scenario-scoped handler runs, and snapshot operations are scoped to the scenario in the URL (clone targets must be in the same plan)
5. **Audit trail scoping** — `GET /audit` lists only entries of plans the caller can access (owners see the whole tenant, tenant-level events are visible to all) and `GET /audit/{entryId}/detail` resolves the entry's scenario to its plan and applies the same rule before capturing scenario data; entries of other plans read as `404`
6. **Sub-resource binding** — `ResourceScopeMiddleware` guards every route that addresses one product, BEP snapshot, BEP optimisation plan or scenario-level cap-table row (`{productId}` ∈ scenario, BEP `{snapshotId}` ∈ scenario, `{optPlanId}` ∈ BEP snapshot, cap-table `{id}`/`{roundId}` ∈ scenario); plan-level shareholders are checked against `{planId}` in the handler. A foreign ID is reported as `404`, never confirmed

### Graceful Shutdown

`main.go` listens for `SIGINT`/`SIGTERM` and:
1. Stops accepting new connections
2. Waits up to 30 seconds for in-flight HTTP requests to complete
3. Drains the async AI usage recorder
4. Drains the async event emitter (audit log / cache)
5. Closes the database connection pool

---

## Makefile Reference

```bash
make build              # compile → bin/ascenda-api
make run                # run with air (live reload)
make run-prod           # run without live reload

make test               # all tests + race detector
make test-short         # unit tests only (no testcontainers)
make test-cover         # HTML coverage report
make test-compute       # compute package tests only

make lint               # golangci-lint
make fmt                # gofmt + goimports
git config blame.ignoreRevsFile .git-blame-ignore-revs   # once per clone: skip the tree-wide reformat in git blame
make vet                # go vet

make migrate-up         # apply pending migrations
make migrate-down       # roll back one step
make migrate-create name=<migration_name>

make db-reset           # drop + create + migrate (DANGER)
make db-seed            # seed development data

make docker-compose-up
make docker-compose-down
```

---

## Key Features

- Deterministic financial modelling engine (P&L, cash flow, balance sheet)
- Scenario simulation and comparison
- Cap table and financing modelling
- Break-even and unit economics analysis
- AI-powered financial narration and insights
- Multi-tenant SaaS architecture
- High-performance compute layer with pure functions

---

## Status

Ascenda backend is under active development.

Current focus:
- Stabilising core financial engine
- Expanding AI-assisted features
- Preparing for production-grade SaaS deployment

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, the checks CI runs, and the pull-request
workflow; changes are listed in [CHANGELOG.md](CHANGELOG.md). Report vulnerabilities privately as
described in [SECURITY.md](SECURITY.md).

## License

Copyright © 2026 Olivier Vandermoten.

Ascenda is free software: you can redistribute it and/or modify it under the terms of the
[GNU Affero General Public License, version 3](LICENSE) (`AGPL-3.0-only`). If you run a
modified version as a network service, the AGPL requires you to offer its users the
corresponding source code.
