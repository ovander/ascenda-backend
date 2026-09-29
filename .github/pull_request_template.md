## What and why

<!-- What this changes and why. Link the issue if there is one ("Closes #…"). -->

## How it was tested

<!-- New or changed tests, and anything checked by hand. -->

- [ ] `gofmt`, `go vet`, `go test -short -race ./...` pass
- [ ] `golangci-lint run --new-from-rev=origin/main ./...` reports no new issue
- [ ] `make test-integration` passes (when repositories, migrations or SQL change)
- [ ] A line is added under `## [Unreleased]` in `CHANGELOG.md`

## Deploy notes

<!-- Delete what does not apply. -->
- Migration: <!-- number, and whether it deletes or rewrites data (back up first) -->
- New or changed environment variable: <!-- name, default, also in the README table -->
- Order with the frontend: <!-- e.g. deploy the backend first -->
