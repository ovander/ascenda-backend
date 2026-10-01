# Moving Ascenda to the new Socrate: what happened and what we learned

**Period:** 2026-09-29 to 2026-10-01. **Outcome:** sign-in works end to end on
`https://socrate.vandermoten.eu`, through the backend's Backend-for-Frontend (BFF); no OAuth
token reaches the browser. In production: backend **v2.8.1** (`7645d74`, Go 1.27.1), frontend
**v1.7.0** (`5d13bfb`, Node 24.21.0, Vite 8.3.1, Vue 3.5.43, TypeScript 6.0.3).

This is the retrospective, meant for the next application that moves onto Socrate. Parashift
followed it on 2026-10-01; what that move added is in §6 and in the checklist, and its own
retrospective is [`parashift-backend/docs/SOCRATE-MIGRATION-2026-10-01.md`](https://github.com/ovander/parashift-backend/blob/main/docs/SOCRATE-MIGRATION-2026-10-01.md). The detailed
records stay where they are:

- [`SOCRATE-COMPAT-REPORT.md`](../SOCRATE-COMPAT-REPORT.md): the read-only audit against Socrate
  v1.3.0 and its fix status.
- [`docs/SOCRATE-APP-ID-2026-09-30.md`](SOCRATE-APP-ID-2026-09-30.md): the app ID change request,
  what a service account may call, and the Socrate follow-ups.

## 1. Final shape

```
browser ── https://ascenda.vandermoten.eu ── Caddy ─┬─ /bff/* /api/* /auth/* → API 127.0.0.1:8082
           (HttpOnly __Host-ascenda_session cookie, │
            CSRF token in memory)                   └─ everything else → SPA (try_files → index.html)

API ── OAuth (authorize, token, refresh, revoke, userinfo, JWKS) → https://socrate.vandermoten.eu
    └─ admin API (service-account calls) → http://127.0.0.1:18082 → SSH tunnel → Socrate loopback
```

| Setting (API, `/opt/apps/ascenda/env/.env`) | Value |
|---|---|
| `SOCRATE_BASE_URL` | `https://socrate.vandermoten.eu` (also the issuer; no trailing slash) |
| `SOCRATE_JWKS_URL` | `https://socrate.vandermoten.eu/.well-known/jwks.json` |
| `SOCRATE_ADMIN_URL` | `http://127.0.0.1:18082` (never derived) |
| `SOCRATE_CLIENT_ID` | `cVqYPgs3uGZYgw1x6F3aOQ` |
| `SOCRATE_CLIENT_SECRET` | set on the VPS only |
| `SOCRATE_APP_ID` | `3` |
| `BFF_REDIRECT_URL` | `https://ascenda.vandermoten.eu/bff/callback` |

| Setting (Socrate, app 3) | Value |
|---|---|
| Redirect URI | `https://ascenda.vandermoten.eu/bff/callback` only |
| Magic-link URL | `https://ascenda.vandermoten.eu/magic-link` |

The SPA has no Socrate setting at all: it calls its own origin, and the backend picks the client,
the redirect URI and the issuer.

## 2. The sequence that worked

| Step | Backend | Frontend |
|---|---|---|
| 1. Read-only audit against the provider's source | #28, #30 (v2.5.0) | same report |
| 2. Fix the audit's blockers on the old flow (refresh rotation, magic link, audience, roles) | #29, #31 (v2.5.0) | #22, #23 |
| 3. Phase 1 cut-over: every Socrate call through backendkit, admin URL never derived, client attribution; SPA pointed at the new issuer | #38–#44 (v2.7.0) | #29–#33 (v1.6.0) |
| 4. BFF, additive: `/bff/*` next to the old routes | #46 | |
| 5. SPA switched to the BFF; old token code deleted | | #35 (v1.7.0) |
| 6. Old browser token routes removed | #47 (v2.8.0) | |
| 7. App ID recorded; backendkit v1.15.1 for sign-up and profile look-ups | #49, #50 (v2.8.1) | |

Steps 4–6 kept every PR deployable on its own: the backend could ship A while the old SPA
still worked, and C only removed what B no longer used. **The deploy itself is not
independent:** v2.8.0+ and v1.7.0 must go out in the same window, backend first.

## 3. Lessons

Each lesson is the symptom we saw, the cause, and the rule we keep.

### Identity-provider contract

1. **Audit before changing anything, against the provider's code.** This environment could not
   fetch Socrate's discovery document (the egress proxy refused it), so the unknowns were
   settled from the Socrate source by someone with access. Every later fix traced back to a row
   of that report. *Rule:* a dated, read-only compatibility report first; fixes reference its rows.
2. **Check the audience and read the app's own role.** Tokens from one Socrate serve every app on
   it; a top-level `role` made an admin of another app an Ascenda platform admin.
   *Rule:* require `aud` = client ID (`SOCRATE_VERIFY_AUDIENCE`, default on) and read
   `app_roles[client_id]` only.
3. **New instance, new identifiers.** The client ID changed (`VowmS…` → `cVqY…`), and so did the
   app ID (legacy `6` was not carried over; it is `3`). Socrate's `sub` is its user row's primary
   key, so users kept their workspaces only because the user rows were carried over with their IDs.
   *Rule:* confirm, before the cut-over, the new client ID, app ID and whether `sub` values are kept.
4. **A service account cannot find its own app ID.** `GET /api/admin/apps` is for human global
   admins, so backendkit's lookup got `401` and every call Ascenda made as itself failed.
   *Rule:* set `SOCRATE_APP_ID` explicitly; ask the provider for it.
5. **Service tokens work on `/api/apps/{id}/service/*` only.** Registration and profile look-ups
   used the app-admin routes and got `401`; Ascenda answered `500` on sign-up and silently skipped
   names in team lists. Fixed upstream in backendkit v1.15.1, then a one-line bump here.
   *Rule:* for every admin call, check which token the route accepts; fix the library, not the app.
6. **The provider's magic-link URL may not suit a BFF.** Socrate's e-mail linked to a POST-only
   endpoint (`405` on click). The fix is a per-app landing URL on the SPA, which posts the token to
   `/bff/magic-link/verify`. A provider-hosted fallback page would redeem the link at Socrate and
   hand the tokens to the browser: refuse it.
7. **The admin API is loopback-only.** Its default (the public URL with port 8081) is wrong behind
   a TLS proxy. *Rule:* `SOCRATE_ADMIN_URL` is required and never derived; on the apps VPS it is the
   shared SSH tunnel `127.0.0.1:18082` (`systemctl status socrate-admin-tunnel`). When the tunnel
   is down, invitations, sign-up and magic-link e-mails fail; sign-in does not.
8. **Tell the provider who the user is, from one trusted hop.** Socrate rate-limits and audits by
   client address. *Rule:* send one address resolved by the API (`X-Forwarded-For` trusted from a
   loopback peer only, rightmost non-loopback entry); never forward the browser's
   `X-Forwarded-For` or `X-Real-IP`.

### Backend-for-Frontend

9. **Keep tokens on the server.** The browser holds an HttpOnly `__Host-` cookie and a CSRF token
   in memory; the SPA has no Socrate setting and no token code (`noBrowserTokens.spec.ts` keeps it
   that way). Sessions live in memory: a restart signs everyone out.
10. **Register the redirect URI before deploying the API that uses it.** Symptom: Socrate's page
    *"redirect_uri is not registered for this client"* (`invalid_request`). The URI is compared
    exactly: scheme, host, path, no trailing slash.
11. **Mismatched releases fail loudly.** With the new API and the old SPA, sign-in showed
    *"Authentication Error — Request failed with status code 404"*: the old `/callback` page
    posted to `/auth/callback`, which v2.8.0 removed. *Rule:* deploy the API, then the SPA, in
    one window.
12. **Old tabs keep old code.** A tab opened before the frontend deploy still ran the old flow,
    so Socrate sent it to the old `/callback`, which the new SPA sends to
    `/landing?code=…&state=…`. Tell-tale: a 32-character base-36 `state` (the old SPA's), where
    the BFF issues 43 base64url characters. *Rule:* after the cut-over, sign in from a fresh tab,
    and remove the old redirect URI at the provider so stale code fails with a clear error.

### Operations

13. **The provider's env template is not the app's.** Socrate's generated snippet says
    `SOCRATE_ADMIN_BASE_URL` and `SOCRATE_ISSUER`; Ascenda reads `SOCRATE_ADMIN_URL` and derives
    the issuer from `SOCRATE_BASE_URL`. The template's `SOCRATE_CLIENT_SECRET=` is empty: pasting
    it would erase the real secret. Compare names, never paste the block.
14. **Ignored local files still build.** `.env.production.local` (git-ignored by `*.local`) is
    loaded by Vite in production mode; it still named the old client and `/callback`. It was inert
    only because v1.7.0 reads none of those variables. Delete stale local env files.
15. **Passwords belong to the Socrate instance.** *"Invalid email or password"* on Socrate's form
    is the account, not the app: check in Socrate's admin that the user exists, is verified,
    unlocked and a member of the app ("User" is enough; Ascenda assigns its own role), then use
    *Forgot password?*.
16. **Probe the live system with GET.** `curl -I` sends HEAD, which `/bff/login` does not answer,
    so no `Location` came back. Use
    `curl -s -o /dev/null -D - "https://ascenda.vandermoten.eu/bff/login?return_to=/" | grep -i '^location'`.
    `/api/v1/version` and `/VERSION` (frontend) give the running builds.

### Release discipline

17. **Tag only the merged release commit.** Two tags were pushed before their release PRs merged
    and had to be moved. *Rule:* tag from an updated `main`, guarded by
    `grep -q "^## \[X.Y.Z\]" CHANGELOG.md && git tag -a vX.Y.Z …`.
18. **A gate that does not stop a push is no gate.** One commit was pushed with failing tests
    because the push was chained after a `grep`. *Rule:* run the full gate, check its exit code,
    then push.
19. **e2e tests that pass locally can hide redirect bugs.** In Playwright, a request that follows
    a `302` returned by `route.fulfill` bypasses `page.route()`, so the fake identity provider
    behaved differently in CI. The e2e suite now fakes redirects with forwarding pages and serves
    the fake issuer on the same origin.

## 4. Checklist for the next application

Before code:
- [ ] New client ID, app ID, issuer and admin URL from the provider; `sub` values carried over?
- [ ] Read-only compatibility report against the provider's source, dated, with a status table.
- [ ] The server as it is: the reverse-proxy block, the service unit
      (`systemctl show -p User -p WorkingDirectory`), the env variable names (values cut to four
      characters) and the running release (`/api/v1/version`, `readlink` of the current link).
- [ ] Every provider URL in the env file compared with the provider's discovery document.
- [ ] A secret ever committed is presumed live: rotate it at the provider and replace it on the
      server in the same step.

Code (one PR each):
- [ ] Every provider call through backendkit (`jwtauth`, `socrate.Client`, `bff`); no hand-written
      OAuth or admin requests.
- [ ] Audience check on; roles from `app_roles[client_id]`.
- [ ] `SOCRATE_APP_ID` and `SOCRATE_ADMIN_URL` required in production, never derived.
- [ ] Service-account calls only on `/api/apps/{id}/service/*`.
- [ ] Client attribution from a loopback-trusted `X-Forwarded-For`.
- [ ] BFF additive → SPA switch with a no-token test → old token routes removed.
- [ ] Magic-link landing page on the SPA, redeemed through the BFF.
- [ ] One environment variable (`APP_ENV` here), required, no default outside tests.
- [ ] Accounts linked by e-mail only on a verified e-mail from the profile, never from the token.
- [ ] e2e: the catch-all API mock answers 404 JSON (a same-origin preview answers `index.html`).
- [ ] govulncheck in CI; a unit test that builds the tracing resource, if there is one.

Cut-over:
- [ ] Register the BFF redirect URI and the magic-link URL on the app at the provider.
- [ ] Reverse proxy: `/bff/*`, `/api/*`, `/auth/*` → API on `127.0.0.1:<PORT>` (not `localhost`,
      which may resolve to `::1`); the rest → SPA.
- [ ] Env: compare names with the app's README table; keep the existing secret. Keep the file valid
      for the old release too until the new one is confirmed, or restore the backup on rollback.
- [ ] A new or changed deploy script runs first against a sandbox copy of the layout; migrations
      run as the service user from its working directory; any failure after the stop restarts a
      release.
- [ ] Backups first: database, env file, proxy config, the old web app.
- [ ] Release section in `CHANGELOG.md`, tag on the merge commit, then `git ls-remote --tags` and
      the release workflow run checked before deploying.
- [ ] `ls .env*` (names only) in the build checkout; deploy settings copied from a sibling app.
- [ ] Deploy the API, then the SPA; check `/api/v1/version` and `/VERSION`.
- [ ] Sign in from a fresh private window; then magic link, sign-up, invitation, team list.
- [ ] Remove the old redirect URI; trim `CORS_ORIGINS`.

## 5. Symptom index

| Symptom | Cause | Lesson |
|---|---|---|
| Socrate: *redirect_uri is not registered for this client* | BFF callback not registered on the app | 10 |
| *Authentication Error — Request failed with status code 404* | old SPA with the new API | 11 |
| Lands on `/landing?code=…&state=…` after Socrate | stale tab running the old flow | 12 |
| Socrate: *Invalid email or password* | account password on the new instance | 15 |
| Sign-up answers `500`; names missing in team lists | service token on a user-JWT route | 5 |
| Magic-link e-mail opens a `405` | provider link is POST-only | 6 |
| Every service-account call `401` | app ID looked up instead of configured | 4 |
| Invitations and magic-link e-mails fail, sign-in works | admin SSH tunnel down | 7 |

## 6. After Parashift

Parashift moved on 2026-10-01 with this checklist from the start ([its retrospective](https://github.com/ovander/parashift-backend/blob/main/docs/SOCRATE-MIGRATION-2026-10-01.md)). What
it added, and where Ascenda stands (checked 2026-10-01):

| Parashift lesson | Ascenda |
|---|---|
| Look at the server before planning: the API was not on the app's own host | Done at the cut-over (Caddy routes `/bff`, `/api`, `/auth`). |
| The VPS ran a release five months older than the latest tag | Running releases checked: v2.8.1 and v1.7.0. |
| The committed secret was the live one, already revoked by Socrate | No secret was ever committed here. |
| An issuer from the old provider in the env file | Ascenda has no `SOCRATE_ISSUER`; the issuer is `SOCRATE_BASE_URL`. |
| The code read `ENV`, the VPS set `APP_ENV`: production checks silently off | VPS has `APP_ENV=production` and listens on `127.0.0.1:8082`; `APP_ENV` is now required (#53). |
| Linking by e-mail without a verified e-mail | Ascenda provisions users by `sub`, not by e-mail. |
| Same-origin e2e: unmocked calls get `index.html` | `mockApiCalls` handles every `/api/v1/**` call. |
| The deploy script had never run as written (user, working directory, rollback) | Migrations are embedded in the binary; `PREVIOUS` is read before the stop; the rollback restarts the service. |
| An env file edited for the new release broke the rollback | Rule added to the checklist; no pending env change. |
| `localhost` upstream after binding `127.0.0.1` | Caddy already proxies to `127.0.0.1:8082`. |

