# Contributing to the Ascenda backend

Thank you for your interest. Ascenda is two repositories: this Go API and the Vue frontend,
[`ovander/ascenda-frontend`](https://github.com/ovander/ascenda-frontend). Contributions are
accepted under the project's licence, [AGPL-3.0](LICENSE).

## Development setup

Requirements: Go (the `toolchain` line in `go.mod` downloads the exact version, 1.27.1),
PostgreSQL 16, and Docker for the database integration tests.

```bash
git clone https://github.com/ovander/ascenda-backend && cd ascenda-backend
go mod download
# Configure the environment variables listed in README.md → Environment Variables,
# at least DATABASE_URL. Without the SOCRATE_* settings, authenticated routes are unavailable.
make run            # APP_ENV=development; migrations run at start-up
```

Useful targets: `make help` lists them. `make migrate-create name=add_foo` creates a new
migration pair.

## Architecture rules

- Layers: handler (parse, validate) → service (business logic) → repo (GORM) → PostgreSQL.
  Handlers never call repositories directly.
- `internal/compute` holds the financial model as pure functions: no database, no HTTP, no I/O.
  Any new calculation goes there, with unit tests.
- Every query is scoped by the tenant from the request context. Plan- and scenario-scoped routes
  go through the plan-access middleware; an ID from a request body is checked against the
  caller's plan before use.
- Money uses `decimal.Decimal`.
- A schema change is a new migration pair; released migrations are never edited.

## Tests and checks

Run these before opening a pull request; CI runs the same and all of them are required:

```bash
test -z "$(gofmt -l cmd internal migrations)"
go vet ./...
go test -short -race -count=1 ./...
golangci-lint run --new-from-rev=origin/main ./...   # v2.14.0
make test-integration                                # needs Docker, or TEST_DATABASE_URL
govulncheck ./...
```

- Unit tests sit next to the code (`*_test.go`), table-driven with testify where it helps.
- Database tests carry the `integration` build tag and run against a real PostgreSQL
  (testcontainers, or the database in `TEST_DATABASE_URL`).
- A bug fix comes with a test that fails without it.

## Pull requests

1. Branch from `main` (`feat/…`, `fix/…`, `chore/…`, `docs/…`).
2. Commit with [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`,
   `chore:`, `docs:`, `ci:`, `test:`).
3. Add a line under `## [Unreleased]` in [`CHANGELOG.md`](CHANGELOG.md).
4. Open the PR with the template filled in, including deploy notes (migration, new environment
   variable, order with the frontend).
5. CI must be green. The maintainer reviews and merges.

## Releases

The maintainer tags releases `vX.Y.Z` on `main` and deploys them with `script/push.sh`, which
refuses an untagged or dirty tree. The deploy SSH settings come from
`~/.config/ascenda/deploy.env`; see the script's header.

## Security

Please do not open a public issue for a vulnerability. See [SECURITY.md](SECURITY.md).
