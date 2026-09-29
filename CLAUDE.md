# CLAUDE.md — ascenda-backend

Standing instructions for Claude Code in this repository. Read this file and `CONTRIBUTING.md`
before any change. The frontend lives in `ovander/ascenda-frontend`; many changes touch both.

## Project in one paragraph

Ascenda is a financial-planning SaaS for startups and SMEs: multi-year business plans (P&L, cash
flow, balance sheet, WCR, break-even, cap table), scenario simulation and AI narration. This
repository is the Go API: chi router, GORM on PostgreSQL 16, SQL migrations with golang-migrate,
authentication by the Socrate OAuth 2.1 / OIDC provider through `github.com/ovander/backendkit`.
It is multi-tenant: every business row belongs to a tenant (workspace), and plans have members.

## Sources of truth, in order

1. The code. Read it before proposing changes; do not describe code you have not opened.
2. `docs/AUDIT-2026-09-25.md` (with its dated status table) and `SOCRATE-COMPAT-REPORT.md` for
   known findings and the identity-provider contract.
3. `docs/openapi.yaml` describes part of the API only; the router (`internal/router/router.go`)
   is authoritative.

## Hard rules

- **Layering.** `handler` → `service` → `repo`; handlers do not call repositories. `internal/compute`
  is pure: it imports only the standard library, `uuid`, `decimal` and `model`/`pkg` types — no
  GORM, no `net/http`, no repo or service.
- **Tenant scoping.** Every repository call takes the tenant from the request context
  (`ctxutil.GetTenantID`), never from the request body. A plan-scoped route checks plan access
  (`RequirePlanAccess`/`RequirePlanEdit`), and a scenario-scoped route checks that the scenario
  belongs to the plan in the URL (`RequireScenarioInPlan`). Any ID taken from a body must be
  checked against the caller's plan before it is written to.
- **Roles.** Read the role from the context the middleware sets (`ctxutil.GetUserRole`). The auth
  middleware scopes it to Ascenda (`app_roles[SOCRATE_CLIENT_ID]`) and checks the token audience;
  never read the token's top-level `role` claim, and do not turn the audience check off.
- **Migrations.** A schema change is a new numbered pair in `migrations/`
  (`make migrate-create name=...`), with a working `down`. Never edit a migration that has been
  released (tagged). `AutoMigrate` is development-only.
- **Money** is `decimal.Decimal`, never `float64`.
- **Never weaken a gate** to get green: no skipped or deleted tests, no `//nolint` or `t.Skip`
  without a one-line reason, no `continue-on-error`, no required check removed.
- **Secrets** never enter the repository: no `.env`, keys or tokens. The deploy SSH settings live
  in `~/.config/ascenda/deploy.env`, outside the repo.
- **Scope.** One change per PR; do not widen a PR with unrelated fixes (open a separate one).

## Local gate (the same checks as CI)

```bash
test -z "$(gofmt -l cmd internal migrations)"          # gofmt
go vet ./...                                          # Build, vet, unit tests
go test -short -race -count=1 ./...
golangci-lint run --new-from-rev=origin/main ./...    # golangci-lint v2.14.0: no new issues
make test-integration                                 # Docker, or TEST_DATABASE_URL (-p 1)
govulncheck ./...                                     # no reachable vulnerability
```

All six CI checks are required on `main`. golangci-lint fails a PR only on the issues it adds;
the existing backlog is reported on pushes to `main`.

## Git workflow

- Branch from `main`: `feat/…`, `fix/…`, `chore/…`, `ci/…`, `docs/…`. Conventional Commits.
- Open a PR; never push to `main`, never force-push a shared branch, never merge with red CI.
  The owner merges; merged branches are deleted automatically.
- Each PR adds a line under `## [Unreleased]` in `CHANGELOG.md`, and says in its body what it
  changes, how it was tested, and any deploy note (migration, env variable, order with the
  frontend).

## Releases and deploys (the owner runs them)

- A release is an annotated tag `vX.Y.Z` on `main`, with the `[Unreleased]` changelog section
  moved under the new version. Do not tag unless asked.
- `script/push.sh` builds, uploads and deploys a tag; `script/version-guard.sh` refuses a dirty
  or untagged tree. Back up the database before any release with a migration. Deploy the backend
  before the frontend when the API changes.

## Docs discipline

A new environment variable goes into the README table (and `internal/config/config.go`
validation when production needs it). Fixing an audit finding updates its row in the audit
status table. A new route should be added to `docs/openapi.yaml`.
