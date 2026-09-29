# Security policy

Ascenda holds business plans and financial data, so security reports are welcome and handled
first.

## Reporting a vulnerability

Please use GitHub's **private vulnerability reporting**: the repository's **Security** tab →
**Report a vulnerability**. Do not open a public issue or pull request for a vulnerability.

Include what you found, how to reproduce it, and the version you tested. The About panel
(user menu → About Ascenda → Copy) gives the application and server versions and commits.

You will get an acknowledgement within a week. Fixes are released as soon as they are ready, and
the report is credited in the release notes unless you prefer otherwise.

## Scope

- In scope: the Ascenda backend API (`ovander/ascenda-backend`) and web application
  (`ovander/ascenda-frontend`), including workspace isolation, plan access and authentication
  handling.
- Out of scope: the Socrate identity provider itself (report those to its maintainers),
  denial-of-service by volume, and findings that need a compromised user device.

## Supported versions

Only the latest release, the one deployed at https://ascenda.vandermoten.eu, receives security
fixes.

## Past reviews

The technical audit of 2026-09-25 and the status of each of its findings are in
[`docs/AUDIT-2026-09-25.md`](https://github.com/ovander/ascenda-backend/blob/main/docs/AUDIT-2026-09-25.md)
in the backend repository.
