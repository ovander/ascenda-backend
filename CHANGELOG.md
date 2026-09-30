# Changelog

All notable changes to the Ascenda backend. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [Semantic Versioning](https://semver.org/). Numbers like (#29) are pull requests
in this repository. Entries before 2.4.0 are rebuilt from the release tags.

## [Unreleased]

### Changed
- Test through the production router that the removed `POST /auth/login` answers 404/405
  (Socrate audit row 3).
- `SOCRATE_ADMIN_URL` is never derived: start-up fails in every environment when
  `SOCRATE_BASE_URL` is set without it, as in production already. Documented value
  `http://127.0.0.1:18082`, the apps VPS's SSH tunnel to Socrate's admin API (README
  *Deployment*, new `.env.example`).
- The address sent to Socrate is resolved from `X-Forwarded-For` only when the peer is loopback
  (Caddy on this host), else from the peer; `X-Real-IP` is never read, and
  `TRUSTED_PROXY_CIDRS` no longer applies to it (it still does to rate limiting). Uses
  backendkit's `bff.WithClientAttribution`; a browser's `X-Forwarded-For`/`X-Real-IP` never
  reaches Socrate.

### Removed
- `SOCRATE_INTERNAL_URL`: OAuth calls always go to the public issuer URL, never through the
  admin tunnel.

## [2.7.0] - 2026-09-29

### Added
- Client attribution: sign-in, refresh, logout and magic-link calls tell Socrate the browser's
  address (resolved through `TRUSTED_PROXY_CIDRS`) and User-Agent, so Socrate v1.5.0+ audits
  and rate-limits each user instead of this server's address (#38).
- `SOCRATE_INTERNAL_URL` (optional): calls to Socrate's OAuth endpoints go to its loopback
  address when set, so Socrate sees the browser's address; the issuer and JWKS stay public (#39).
- `BIND_ADDRESS` (optional, default every interface): set `127.0.0.1` to listen on loopback only (#39).

### Changed
- Production start-up requires `SOCRATE_BASE_URL` and `SOCRATE_ADMIN_URL`, and refuses a Socrate
  URL that is malformed, or a base, internal or admin URL that ends with `/` (the issuer is
  compared exactly) (#39).
- `script/deploy-backend.sh` health-checks the port set in the VPS env file instead of 8082 (#39).
- `backendkit` v1.13.0 → v1.15.0. The code exchange, refresh and revocation of `/auth/callback`,
  `/auth/refresh` and `/auth/logout` go through its `socrate.Client` instead of hand-written
  requests to `/oauth/token` and `/oauth/revoke`; responses are unchanged (#38).

### Removed
- `POST /auth/login`: unused, and it built an authorize URL without PKCE or `state`, which
  Socrate refuses. Sign-in is the SPA's PKCE flow or a magic link (#39).

## [2.6.0] - 2026-09-29

### Added
- Contributor files: `CLAUDE.md`, `CONTRIBUTING.md`, this changelog, `SECURITY.md`,
  `CODEOWNERS`, issue and pull-request templates (#35).
- CI gates: a unit-test coverage floor (75%, `script/coverage-floor.sh`); a test that
  `internal/compute` imports no I/O, database or HTTP code; a test that every route is in
  `docs/openapi.yaml` or on a shrinking list of undocumented routes, and that the spec lists no
  route the router lacks (#36).
- Release workflow: a `vX.Y.Z` tag publishes a GitHub Release with its `CHANGELOG.md` section as
  notes and Linux amd64/arm64 server binaries (#36).

### Changed
- CI: golangci-lint v2.14.0, built with Go 1.27; the lint check runs again (#34).
- `github.com/moby/go-archive` 0.2.0 → 0.3.0, used by the database test tooling (#33).

## [2.5.0] - 2026-09-29

### Added
- Magic-link sign-in through Socrate: `POST /auth/magic-link/verify` redeems Socrate's single-use
  token and returns tokens like `/auth/callback` (#29).
- `SOCRATE_VERIFY_AUDIENCE` (default `true`) (#31).
- AGPL-3.0 licence (#32).
- Socrate v1.3.0 compatibility report (#28, #30).

### Changed
- `backendkit` v1.5.0 → v1.13.0: access tokens must carry `exp`, JWKS re-fetch cooldown, bounded
  reads from upstream services (#29).
- `script/push.sh` reads the VPS SSH settings from `~/.config/ascenda/deploy.env` (#32).

### Removed
- `GET /auth/magic-link/verify` and Ascenda's own magic-link token table (migration 000019) (#29).

### Security
- Roles are scoped to Ascenda (`app_roles[client_id]`) and the token audience is checked: an admin
  of another application on the same Socrate is no longer an Ascenda platform admin (#31).
- The audit report gets a dated status table of its findings (#32).

## [2.4.2] - 2026-09-26

### Changed
- Go toolchain pinned to 1.27.1 (`go.mod` toolchain line; Dockerfile `golang:1.27`) (#26).

### Security
- govulncheck: chi 5.3.0, x/crypto 0.56.0, klauspost/compress 1.18.7; no known vulnerable symbol
  reached (#26).

## [2.4.1] - 2026-09-26

### Changed
- One-command deploy: `push.sh` builds for the VPS's CPU, stamps the version, uploads and
  deploys; version guard; safer rollback (#22).
- Golf demo modelled with the competition and contract drivers (#23).

### Removed
- Unused product cost-variability columns (migration 000018) (#24).

### Fixed
- Product update is partial: a rename no longer resets the sort order, and saving a driver no
  longer blanks the name (#25).

## [2.4.0] - 2026-09-25

### Added
- Athlete drivers: competition (results-driven prize money, per-event and coach costs) and
  contract (sponsorship and image rights with a bonus per win) (#20); coach costed as an annual
  fee plus a share of winnings (#21).
- Baseline SQL migration: a fresh database provisions from migrations alone (#11).
- CI workflow, golangci-lint configuration, tree-wide gofmt gate (#10, #17); technical audit
  report (#1).

### Changed
- Demo plans are an editable sandbox for every user of the tenant (#6); purged with one cascading
  delete (#16).

### Fixed
- Cascade foreign keys from plans and scenarios down the hierarchy (migration 000016) (#12, #15).
- Atomic snapshot restore with replace semantics (#13); scenario clone fails on any error (#14).
- Late migrations aligned with the models (migration 000017), with a schema drift test (#18).
- Audit events are never dropped; audit writes are retried (#19).
- Demo plan definitions test (#3).

### Security
- Tenant resolved from the user record; no default-tenant fallback in production (#4).
- Scenarios bound to their plan, snapshots to their scenario (#5); products, BEP and cap-table
  sub-resources bound to their parent (#7).
- Rate limiters keyed by client IP, user or tenant, never pass-through (#8).
- Audit trail scoped to the plans the caller can access (#9).
- Dependency bumps for govulncheck findings: pgx, jwt, x/text (#2).

## [2.3.1] - 2026-04-29

### Fixed
- Pro Tour Golfer demo: realistic financing and travel costs; cash stays positive.

## [2.3.0] - 2026-04-29

### Added
- Pro Tour Golfer demo business plan.

## [2.2.3] - 2026-04-29

### Fixed
- Budget and cash monthly-override tables created by migration 000015; budget queries use
  `year_index`.

## [2.2.2] - 2026-04-29

### Fixed
- `wcr_entries` table created by migration 000014; WCR queries use `year_index`.

## [2.2.1] - 2026-04-29

### Fixed
- Demo plans readable by every authenticated user without plan membership.

## [2.2.0] - 2026-04-24

### Changed
- `GET /api/v1/tenant/` open to every authenticated user; updating the tenant stays owner-only.

## [2.1.0] - 2026-04-24

### Added
- Reference deploy model: versioned releases, atomic switch, migrations, health check, rollback.

## [2.0.4] - 2026-04-22

### Changed
- Migrations are idempotent and bootstrap an empty database.

## [2.0.3] - 2026-04-22

### Fixed
- Migrations handle missing tables; more robust deploy.

## [2.0.2] - 2026-04-22

### Changed
- Push script updated for Ascenda.

## [2.0.1] - 2026-04-21

### Changed
- Push script finalised.

## [2.0.0] - 2026-04-21

### Changed
- Major refactor: compute orchestrator, scenario analysis, AI platform, enterprise features.

## [0.3] - 2026-03-27

### Added
- Initial platform: compute engine, AI, cap table, infrastructure.

[Unreleased]: https://github.com/ovander/ascenda-backend/compare/v2.7.0...HEAD
[2.7.0]: https://github.com/ovander/ascenda-backend/compare/v2.6.0...v2.7.0
[2.6.0]: https://github.com/ovander/ascenda-backend/compare/v2.5.0...v2.6.0
[2.5.0]: https://github.com/ovander/ascenda-backend/compare/v2.4.2...v2.5.0
[2.4.2]: https://github.com/ovander/ascenda-backend/compare/v2.4.1...v2.4.2
[2.4.1]: https://github.com/ovander/ascenda-backend/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/ovander/ascenda-backend/compare/v2.3.1...v2.4.0
[2.3.1]: https://github.com/ovander/ascenda-backend/compare/v2.3.0...v2.3.1
[2.3.0]: https://github.com/ovander/ascenda-backend/compare/v2.2.3...v2.3.0
[2.2.3]: https://github.com/ovander/ascenda-backend/compare/v2.2.2...v2.2.3
[2.2.2]: https://github.com/ovander/ascenda-backend/compare/v2.2.1...v2.2.2
[2.2.1]: https://github.com/ovander/ascenda-backend/compare/v2.2.0...v2.2.1
[2.2.0]: https://github.com/ovander/ascenda-backend/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/ovander/ascenda-backend/compare/v2.0.4...v2.1.0
[2.0.4]: https://github.com/ovander/ascenda-backend/compare/v2.0.3...v2.0.4
[2.0.3]: https://github.com/ovander/ascenda-backend/compare/v2.0.2...v2.0.3
[2.0.2]: https://github.com/ovander/ascenda-backend/compare/v2.0.1...v2.0.2
[2.0.1]: https://github.com/ovander/ascenda-backend/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/ovander/ascenda-backend/compare/v0.3...v2.0.0
[0.3]: https://github.com/ovander/ascenda-backend/releases/tag/v0.3
