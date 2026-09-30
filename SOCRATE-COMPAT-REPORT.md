# Socrate v1.3.0 compatibility report: Ascenda

**Scope:** the two repositories that make up Ascenda, audited together because the OAuth integration is split between them:

- `ovander/ascenda-backend`, `main` at `5d41054`. Go API; confidential OAuth client; uses `github.com/ovander/backendkit` **v1.5.0**.
- `ovander/ascenda-frontend`, `main` at `e47f87e`. Vue 3 SPA; starts the login and holds the tokens.

The same report is in the root of both repositories. Paths are prefixed `backend/` or `frontend/` for the repository they belong to. `backendkit/` refers to the module source `github.com/ovander/backendkit@v1.5.0`.

**Date:** 2026-09-29. This is a read-only audit: no application code was changed.

**Discovery document:** not fetched from this environment, because the egress proxy refused `https://socrate.vandermoten.eu/.well-known/openid-configuration` (`CONNECT tunnel failed, response 403`). The seven unknowns this left were then checked against the Socrate v1.3.0 source (`go-oauth2`), by a separate review with access to that repository, on 2026-09-29. Six are resolved; the Socrate-side file and line references in the table (`oauth_service.go`, `token.go`, `keys_test.go`) come from that review. U7, the live VPS values, was read on the VPS the same day.

---

## 1. Verdict

**Compatible with changes: 2 blockers · 7 warnings · 0 unknowns.** U1–U6 were resolved from the Socrate v1.3.0 source and U7 from the VPS env file (see §3).

The main interactive login already fits the Socrate v1.3.0 contract:
- Authorization Code with S256 PKCE, started by the SPA;
- code exchanged server-side by the confidential backend;
- access tokens validated as RS256 JWTs against JWKS, keyed by `kid`, with an issuer check.

No implicit flow, password grant or HS256 appears anywhere.

The Socrate source also confirms the rest of the wiring: the endpoint paths, `client_secret_post`, the `api` scope and `use:"sig"` on the keys all match what Ascenda does today. Refresh tokens are issued on every authorization-code grant, without needing `offline_access`.

The two blockers are in flows around that login:

1. **Refresh-token rotation is broken.** The SPA reads the refresh response in a shape the backend never sends. The rotated refresh token is therefore thrown away, and the user is logged out at every access-token expiry.
2. **Magic-link sign-in starts an authorization request without PKCE.** Socrate v1.3.0 rejects it.

Both are small, contained code fixes. Whether existing users keep their workspaces is now known to depend on one migration choice. Socrate's `sub` is the numeric primary key of its `users` table, so a migration that **carries the user rows over with their IDs** keeps every `sub` identical and needs no identity work in Ascenda (U5).


**Fix status (2026-09-29).** This report describes the code as audited. Since then:
- **B1** (refresh rotation) and **W3** (logout revocation) are fixed in ovander/ascenda-frontend#22.
- **B2** (magic link) is fixed in ovander/ascenda-backend#29 and ovander/ascenda-frontend#23. Magic-link sign-in now redeems Socrate's own token through the backend, with no authorize redirect. The backend PR also upgrades `backendkit` to v1.13.0, which covers **W6**.
- **W2** is fixed in ovander/ascenda-backend#31. Tokens must name `SOCRATE_CLIENT_ID` in `aud` (`SOCRATE_VERIFY_AUDIENCE`, default on), and roles come from `app_roles[client_id]`, never the top-level `role`.
- The code exchange, refresh and revocation no longer call `/oauth/token` and `/oauth/revoke` by hand: they go through `backendkit/socrate.Client` (v1.15.0), which also sends Socrate the browser's address and User-Agent (client attribution, used by Socrate v1.5.0+).
- Cut-over to `https://socrate.vandermoten.eu`: production requires `SOCRATE_BASE_URL` (no trailing slash) and `SOCRATE_ADMIN_URL` (`http://127.0.0.1:18082`, the apps VPS's SSH tunnel to Socrate's loopback admin API, which covers **U6**; never derived, in any environment) and rejects a malformed `SOCRATE_REDIRECT_URL` (**U7**). The optional `SOCRATE_INTERNAL_URL` sends OAuth calls over loopback. `POST /auth/login`, which had no PKCE, is removed.
- **W1**, **W4**, **W5** and **W7** are open.

---

## 2. Integration inventory

| Aspect | What the code does | Evidence |
|---|---|---|
| **Current IdP** | Socrate at `https://golfperformance.fr` (frontend build-time value). The backend reads its IdP settings from the VPS env file `/opt/apps/ascenda/env/.env`, which is not in the repo. | `frontend/.env.production:3`; `backend/internal/config/config.go:155-163` |
| **Libraries** | No generic OIDC library. The backend uses `backendkit/jwtauth` (JWT validation) and `backendkit/socrate` (Socrate REST/OAuth client), with `golang-jwt/jwt/v5`. The SPA uses hand-written PKCE with WebCrypto and `axios`. | `backend/internal/middleware/auth.go:7-21`; `backend/go.mod` (`backendkit v1.5.0`); `frontend/src/composables/useAuth.ts:1-20` |
| **Client type** | One **confidential** client (`client_id` + `client_secret`). The SPA only builds the authorize URL; the backend performs every token-endpoint call with the secret. Production refuses to start without `SOCRATE_JWKS_URL`, `SOCRATE_CLIENT_ID`, `SOCRATE_CLIENT_SECRET` and `SOCRATE_REDIRECT_URL`. | `backend/internal/config/config.go:54-62,199-211`; `frontend/.env.production:2` |
| **Interactive flow** | Authorization Code + PKCE S256. The SPA makes a 128-character verifier and a 32-character `state`, stores both in `sessionStorage`, and redirects to `${VITE_SOCRATE_BASE_URL}/oauth/authorize` with `response_type=code`, `scope=openid email profile api` and `code_challenge_method=S256`. The callback checks `state` and posts `{code, codeVerifier, redirectUri}` to `POST /auth/callback`. | `frontend/src/composables/useAuth.ts:46-82`; `frontend/src/features/auth/views/CallbackView.vue:14-27`; `frontend/src/router/index.ts:21-31` |
| **Code exchange** | The backend posts `grant_type=authorization_code, code, client_id, client_secret, redirect_uri, code_verifier` to `${SOCRATE_BASE_URL}/oauth/token`. That is `client_secret_post`. | `backend/internal/handler/auth_handler.go:97-133` |
| **Other interactive entry points** | `POST /auth/login` builds an authorize URL **without PKCE or state**; no frontend code calls it. `GET /auth/magic-link/verify` redirects to authorize **without PKCE**, with `state` set to the post-login path. | `backend/internal/handler/auth_handler.go:49-69`; `backend/internal/handler/magic_link_handler.go:63-116`; `backend/internal/router/router.go:102-111` |
| **Machine-to-machine** | `client_credentials` with `client_secret_post`. The token is cached per process and renewed 30 s before expiry. It is used for the Socrate admin API: register, invite, user look-up and magic-link e-mail. | `backendkit/socrate/client.go:257-300`; callers in `backend/internal/service/{registration_service.go:104, user_service.go:162,217,243, organization_service.go:414, magic_link_service.go:91}` |
| **Endpoints used** | **OAuth port**, relative to `SOCRATE_BASE_URL`, all hard-coded paths rather than read from discovery: `/oauth/authorize`, `/oauth/token`, `/oauth/revoke`, `/oauth/userinfo`, `/.well-known/openid-configuration` (startup reachability check only), and JWKS via `SOCRATE_JWKS_URL`. **Admin port** (`SOCRATE_ADMIN_URL`, default: same host with port `:8081`): `/api/admin/apps`, `/api/apps/{id}/users[/…]`, `/api/apps/{id}/service/…`, including `service/magic-link`. | `frontend/src/composables/useAuth.ts:64`; `backend/internal/handler/auth_handler.go:56,119,223,298`; `backend/internal/handler/magic_link_handler.go:97`; `backendkit/socrate/client.go:95-101,680,710-720`; `backend/cmd/server/bootstrap.go:387-436` |
| **Token validation** | Every `/api/*` request passes through `jwtauth.Middleware`, which requires RS256 and a `kid` header. It looks the key up in the JWKS, cached for 1 h and re-fetched on an unknown `kid`, then checks `iss` against `SOCRATE_BASE_URL`. It does **not** check `aud` or require `exp`, and it keeps only JWKS keys with `use:"sig"`. | `backend/cmd/server/bootstrap.go:145`; `backendkit/jwtauth/middleware.go:92-101,191-240,261-264` |
| **Identity mapping** | The Ascenda user is found by `users.external_id = sub`. Otherwise a pending invitation is claimed by e-mail (`external_id` empty). Otherwise the user is provisioned into the JWT `tenant_id` tenant. Otherwise the request gets 403. A top-level `role: "admin"` claim makes the user a platform admin. | `backend/internal/middleware/tenant.go:79,144,172,215`; `backend/internal/repo/user_repo.go:30-60` |
| **ID token** | Returned by the token endpoint; decoded **without signature verification** to copy `email`/`name` into the user record; not forwarded to the SPA. | `backend/internal/handler/auth_handler.go:135-202` |
| **Refresh** | The SPA's axios interceptor catches a 401 and calls `auth.refresh()`, which is single-flight within a tab. That posts the refresh token to `POST /auth/refresh`, and the backend proxies `grant_type=refresh_token` and returns `{accessToken, refreshToken, expiresIn}`. On any refresh failure the SPA logs out and redirects to the landing page. | `frontend/src/composables/useApi.ts:12-24,53-84`; `frontend/src/stores/auth.ts:31-39`; `backend/internal/handler/auth_handler.go:210-243` |
| **Token storage** | Access and refresh tokens are held in the SPA's JavaScript memory (Pinia store), not in `localStorage`/`sessionStorage`, so they are lost on reload. The PKCE verifier and `state` live briefly in `sessionStorage`. | `frontend/src/stores/auth.ts:11-13`; `frontend/src/composables/useAuth.ts:51-52,80-81` |
| **Logout** | The SPA posts `{refreshToken}` to `POST /auth/logout`; the backend expects `{token}` and would post it to `/oauth/revoke` with `client_secret_post`. There is no RP-initiated logout (`end_session_endpoint`). | `frontend/src/stores/auth.ts:48-59`; `backend/internal/handler/auth_handler.go:281-309` |
| **Not used** | Introspection (a client method exists but has no callers), token exchange (RFC 8693), DPoP, the policy `decide`/PEP endpoint, implicit flow, password grant, HS256. | code search across both repos |

---

## 3. Findings

Ranked most severe first. The *Location* column cites the code that must change or be checked.

| # | Severity | Area | Location | Issue | Required change |
|---|---|---|---|---|---|
| B1 | **BLOCKER** | Refresh rotation (req. 6) | `frontend/src/stores/auth.ts:33-38`; `backend/internal/handler/auth_handler.go:238-242`; test mocks the wrong shape at `frontend/src/stores/auth.spec.ts:65-73` | `refresh()` reads `response.data.tokens.accessToken`, but `/auth/refresh` returns a flat `{accessToken, refreshToken, expiresIn}`. `data.tokens` is undefined, so `refresh()` throws **after** Socrate has already consumed the old refresh token and issued a new one. The new refresh token is discarded, the interceptor logs the user out (`useApi.ts:74-81`), and no session outlives its first access token. The old token is never replayed, because logout clears it, so Socrate's reuse detection is not triggered, but "persist and use the newest refresh token" is not met. | Read the flat shape (`response.data.accessToken` / `response.data.refreshToken`) and fix the unit test to use the real response. Then re-test refresh survival (test plan T3). Longer term, see W1: move refresh into a BFF. |
| B2 | **BLOCKER** (magic-link sign-in only) | PKCE (req. 2) | `backend/internal/handler/magic_link_handler.go:96-116`; route `backend/internal/router/router.go:111` | `/auth/magic-link/verify` redirects the browser to `/oauth/authorize` with no `code_challenge`, which Socrate v1.3.0 rejects. It also uses `state` to carry the post-login path. The SPA callback compares `state` with its stored random value and needs a stored verifier (`useAuth.ts:67-76`), so even a code issued without PKCE would fail there. The landing form posts to a relative `/auth/magic-link` (`frontend/src/features/landing/components/LandingLogin.vue:77`), which reaches the backend only if the frontend host proxies `/auth`. | Start the magic-link login in the SPA, or in a BFF, with the same PKCE + `state` generation as `initiateLogin()`. Keep the post-login path in `sessionStorage['auth_redirect']`, not in `state`. Point the landing form at `VITE_API_BASE_URL`. If magic-link sign-in is not wanted, remove the routes. |
| W1 | WARNING | Architecture (BFF) | `frontend/src/stores/auth.ts:11-13,19-39`; `backend/internal/handler/auth_handler.go:143-147,238-242` | Access **and refresh** tokens are held in the browser. The backend exchanges the code with the client secret but then returns the tokens to the SPA, so it is a token proxy, not a BFF. Keeping them in memory only is better than `localStorage`, but an XSS can still read them. | Adopt the BFF pattern: server-side session, `__Host-` httpOnly cookie, CSRF token, refresh done server-side. `backendkit` ≥ v1.10 ships a `bff` package for Socrate: PKCE S256, per-session coalesced refresh with write-through of the rotated token, `IsFatalRefreshError` for `invalid_grant`, login binding. |
| W2 | WARNING | Token validation / authorization | `backendkit/jwtauth/middleware.go:191-196`; `backend/cmd/server/bootstrap.go:145`; admin role `backend/internal/middleware/tenant.go:79` | No `aud` check. On a Socrate instance that serves several apps (same `iss`), an access token minted for **another client** is accepted here. Platform-admin rights come from the top-level `role` claim, so an `admin` of another app may become an Ascenda platform admin (audit finding S-M1). `exp` is not required either. | Upgrade `backendkit` (≥ v1.8 adds `jwtauth.WithAudience`; ≥ v1.10 requires `exp` and adds leeway) and pass `WithAudience(SOCRATE_CLIENT_ID)`: Socrate puts the `client_id` in `aud` (U3). Take the admin role from `app_roles[client_id]` rather than the top-level `role`. |
| W3 | WARNING | Revocation / logout | `frontend/src/stores/auth.ts:50`; `backend/internal/handler/auth_handler.go:281-283` | The SPA sends `{refreshToken}`; the backend requires `{token}` (`validate:"required"`), answers 400, and the SPA ignores the error. The refresh token is therefore **never revoked** at logout, and its rotation chain stays valid until it expires. There is also no RP-initiated logout, so the Socrate SSO session survives an Ascenda logout. | Send `{ token: refreshToken }`. Also send `token_type_hint=refresh_token` if discovery lists it. If discovery has `end_session_endpoint`, redirect to it with `id_token_hint` and a registered `post_logout_redirect_uri`. |
| W4 | WARNING | PKCE (req. 2) | `backend/internal/handler/auth_handler.go:49-69`; route `router.go:102` | `POST /auth/login` returns an authorize URL without `code_challenge` or `state`. Nothing in the frontend calls it, but the route is live and would produce an authorization request that Socrate v1.3.0 rejects. | Remove the endpoint, or make it generate PKCE S256 + `state`. |
| W5 | WARNING | PKCE hardening | `backend/internal/handler/auth_handler.go:73-75,116-118` | `/auth/callback` treats `codeVerifier` as optional and forwards the code without one. Socrate enforces PKCE, so this does not break compatibility, but the client should not permit a PKCE-less exchange. | Make `codeVerifier` required (43–128 unreserved characters). Pin `redirect_uri` to the configured value instead of accepting it from the request body (`auth_handler.go:104-107`). |
| W6 | WARNING | JWKS handling | `backendkit/jwtauth/middleware.go:215-240` | An unknown `kid` triggers a synchronous JWKS fetch on **every** request, with no cooldown. Rotation works (a new `kid` is fetched on first sight), but a stream of tokens with random `kid`s turns into a stream of requests to Socrate. | Upgrade `backendkit` (≥ v1.10: refetch cooldown + negative cache, `WithMinRefetchInterval`). |
| W7 | WARNING | M2M rate limits | `backendkit/socrate/client.go:257-300`; loop at `backend/internal/service/user_service.go:157-166` | The `client_credentials` token is cached, so there is no hot-loop minting in the normal case. But when minting **fails** nothing is cached and there is no back-off. The per-user enrichment loop in `ListUsers` then calls `/oauth/token` once per user until its 2 s deadline, against a bcrypt-verified, rate-limited endpoint. | Cache the failure briefly (back-off), or skip enrichment after the first token failure in a request. |
| U1 | ✅ RESOLVED | Endpoints (req. 7) | `frontend/src/composables/useAuth.ts:64`; `backend/internal/handler/auth_handler.go:56,119,223,298`; `magic_link_handler.go:97`; `backendkit/socrate/client.go:680,720` | Ascenda hard-codes `{base}/oauth/{authorize,token,revoke,userinfo}`. Socrate v1.3.0 serves exactly these at `{iss}/oauth/authorize`, `/oauth/token`, `/oauth/userinfo`, `/oauth/revoke` and `/oauth/introspect`, with JWKS at `{iss}/.well-known/jwks.json` (`oauth_service.go:1246-1252`). | No path changes. Set `SOCRATE_JWKS_URL=https://socrate.vandermoten.eu/.well-known/jwks.json` (under `.well-known`, not `/oauth`). |
| U2 | ✅ RESOLVED | Client authentication | `backend/internal/handler/auth_handler.go:109-115,217-222,293-297`; `backendkit/socrate/client.go:268-272` | Ascenda sends the secret in the form body (`client_secret_post`). Socrate advertises `client_secret_basic` and `client_secret_post` (`oauth_service.go:1266`). | No change. |
| U3 | ✅ RESOLVED | Scopes / audience | `frontend/src/composables/useAuth.ts:58` | `api` is a valid scope (`oauth_service.go:96`, advertised at `:1261`). Access tokens carry `aud[0] == client_id` (`:655`). Refresh tokens are issued on every authorization-code grant, so `offline_access` is not needed (`:620`). | Register the client with the allowed scopes `openid profile email api`. The W2 fix `jwtauth.WithAudience(SOCRATE_CLIENT_ID)` matches what Socrate issues. |
| U4 | ✅ RESOLVED | JWKS format (req. 5) | `backendkit/jwtauth/middleware.go:261-264` | Socrate publishes every key with `use:"sig"`, `kty:"RSA"`, `alg:"RS256"`; this is test-enforced (`keys_test.go:398`) and was seen in the live JWKS. `backendkit` keeps only `RSA` + `use=="sig"` keys, so every key loads. | No change. |
| U5 | ✅ RESOLVED (migration policy) | Issuer change / user identity (req. 7) | `backend/internal/middleware/tenant.go:144,172,215`; `backend/internal/repo/user_repo.go:30-60` | Ascenda matches users only by `external_id = sub`, and a user it cannot match gets **403** in production. Socrate's `sub` is its user's numeric primary key (`strconv(user.ID)`, `token.go:389`). Carrying the `users` rows over **with their `id`s** (and resetting the sequence) keeps every `sub` identical. A clean-slate user table changes them all. | **Recommended:** make the Socrate data migration keep user IDs; then Ascenda needs no identity work and T1 passes as is. Only if IDs cannot be kept, relink before cut-over: remap `users.external_id` by e-mail, or blank it so the claim-by-e-mail step relinks each account on first login (needs identical e-mails). |
| U6 | ✅ RESOLVED | Socrate admin API | `backendkit/socrate/client.go:95-101,183-243,616-680,802-815`; `backend/internal/config/config.go:56,157` | Registration, invitations, admin user management, profile enrichment and magic-link e-mails use Socrate's admin API. It exists in v1.3.0 (`/api/admin/*`) but listens on the loopback admin port **8082**, not on the `:8081` public-host default `backendkit` derives when `SOCRATE_ADMIN_URL` is unset. Ascenda runs on another VPS (the apps VPS), which reaches it through the host's SSH tunnel, `socrate-admin-tunnel.service`, at `127.0.0.1:18082`. | Set `SOCRATE_ADMIN_URL=http://127.0.0.1:18082` explicitly, and `SOCRATE_APP_ID` (without it the app-ID lookup needs an admin JWT and fails for service calls, `client.go:196-216`). |
| U7 | ✅ RESOLVED | Current backend configuration | `/opt/apps/ascenda/env/.env` on the VPS (not in the repo) | Read on 2026-09-29: `SOCRATE_BASE_URL=https://golfperformance.fr`, `SOCRATE_ADMIN_URL=https://socrate-admin.golfperformance.fr`, `SOCRATE_CLIENT_ID=VowmSxfnObxDKFvdk1Lucg` (the same client ID as the frontend build), `SOCRATE_JWKS_URL=https://golfperformance.fr/.well-known/jwks.json`, **`SOCRATE_APP_ID` empty**, and **`SOCRATE_REDIRECT_URL=http:httpd://ascenda.vandermoten.eu/callback`**, which is malformed. The redirect URL is used by magic-link sign-in and `/auth/login`, and as the fallback in `/auth/callback`. The SPA's own login always sends its redirect URI, so it is unaffected. With `SOCRATE_APP_ID` empty, the service-account admin calls (invitations, registration) cannot resolve the app ID (`backendkit/socrate/client.go:196-216`). | Now, independent of the migration: set `SOCRATE_REDIRECT_URL=https://ascenda.vandermoten.eu/callback` and the current numeric `SOCRATE_APP_ID`. At cut-over: replace every value as in checklist step 7. |
| I1 | INFO | ID token | `backend/internal/handler/auth_handler.go:135-202` | The ID token is decoded without signature verification. OIDC Core §3.1.3.7 allows relying on TLS for a token received directly from the token endpoint, so this is acceptable. There is no `nonce`, and `iss`/`aud` are not checked. | Optional: verify it with the same JWKS middleware and check `iss`/`aud`. Low priority. |
| I2 | INFO | Documentation / dev defaults | `backend/README.md:234-238`; `backend/docker-compose.yml:34-38` | The examples point at `auth.ascenda.com` / `localhost:9000`, the client `kerplan-api`, and a `/auth/callback` redirect path the SPA does not use. | Update them to the Socrate v1.3.0 values when migrating. |

**Requirements that pass:**
- **Req. 1, grants:** only `authorization_code`, `refresh_token` and `client_credentials` are used.
- **Req. 3, no implicit or password grant:** a search of both repositories finds no `response_type=token`/`id_token` and no `grant_type=password`.
- **Req. 4, fixed redirect URIs:** the redirect URI is a single value from configuration (`VITE_SOCRATE_REDIRECT_URI`, `SOCRATE_REDIRECT_URL`), with no wildcard and no dynamic path.
- **Req. 5, RS256 with JWKS and `kid`:** tokens are validated against JWKS, keys are looked up by `kid`, and a new `kid` triggers a re-fetch, so rotation works. Nothing uses HS256.
- **Req. 6, `invalid_grant`:** any refresh failure logs the user out and sends them back to sign in (`useApi.ts:74-81`), never retrying with the old token.
- **Refresh single-flight:** refreshes are coalesced within a tab (`useApi.ts:12-24,54-64`).
- **Token exchange, DPoP, the `decide` endpoint and introspection:** none is relied on.

---

## 4. Migration checklist

Ordered: fix the code first, then configure, then cut over.

1. **Fix B1 (refresh shape).**
   - `frontend/src/stores/auth.ts`: read `response.data.accessToken` / `response.data.refreshToken`.
   - Correct the mock in `auth.spec.ts`.
   - Add a test that the stored refresh token is the **new** one after `refresh()`.
2. **Fix B2 (magic link).** Either:
   - start the post-verification authorize request with PKCE S256 + random `state` (reuse `initiateLogin()`, keeping the return path in `sessionStorage['auth_redirect']`) and point the landing form at the API origin;
   - or remove `/auth/magic-link*`.

   Also fix W4 (remove `POST /auth/login`) and W5 (require `codeVerifier`; pin `redirect_uri`).
3. **Fix W3 (logout):** send `{ token: refreshToken }` to `/auth/logout`. If discovery lists `end_session_endpoint`, add RP-initiated logout.
4. **Fix the live configuration now (U7).** The VPS values read on 2026-09-29 show two problems that exist today, before any migration:
   - `SOCRATE_REDIRECT_URL` is malformed (`http:httpd://…`); set it to `https://ascenda.vandermoten.eu/callback`.
   - `SOCRATE_APP_ID` is empty; set the numeric app ID from the current Socrate admin console.

   Restart the backend afterwards (`sudo systemctl restart ascenda`).
   U1–U4 and U6 are confirmed from the Socrate v1.3.0 source. A final check of the live discovery document costs one command:
   ```bash
   curl -s https://socrate.vandermoten.eu/.well-known/openid-configuration | jq '{issuer, authorization_endpoint, token_endpoint, jwks_uri, scopes_supported, token_endpoint_auth_methods_supported}'
   ```
   `issuer` must be exactly `https://socrate.vandermoten.eu`.
5. **Keep user identity (U5).** Make the Socrate data migration carry the `users` rows over with their `id`s and reset the ID sequence, so every `sub` stays the same. Ascenda then needs no identity work. Only if that is impossible: prepare and test the `external_id` relink (by e-mail, or by blanking it for the claim-by-e-mail path). Either way, take an Ascenda database backup before cut-over.
6. **Register the client on `socrate.vandermoten.eu`:**
   - confidential client;
   - grants: `authorization_code`, `refresh_token`, `client_credentials`;
   - PKCE S256 required;
   - redirect URI registered exactly: `https://ascenda.vandermoten.eu/callback`, plus any dev URIs such as `http://localhost:5173/callback`;
   - if RP-initiated logout is added, a post-logout redirect URI;
   - allowed scopes: `openid profile email api`;
   - admin-API access for the service account.

   Record the new `client_id`, `client_secret` and the numeric app ID.
7. **Backend configuration** (`/opt/apps/ascenda/env/.env`):
   ```
   SOCRATE_BASE_URL=https://socrate.vandermoten.eu          # also the expected iss: no trailing slash
   SOCRATE_JWKS_URL=https://socrate.vandermoten.eu/.well-known/jwks.json
   SOCRATE_CLIENT_ID=<new client_id>
   SOCRATE_CLIENT_SECRET=<new secret>                         # env file only, root-owned; never committed
   SOCRATE_APP_ID=<numeric app id>
   SOCRATE_ADMIN_URL=http://127.0.0.1:18082                   # SSH tunnel to Socrate's loopback admin API; never the :8081 default
   SOCRATE_REDIRECT_URL=https://ascenda.vandermoten.eu/callback
   ```
8. **Frontend configuration** (`frontend/.env.production`), then rebuild and deploy:
   ```
   VITE_SOCRATE_BASE_URL=https://socrate.vandermoten.eu
   VITE_SOCRATE_CLIENT_ID=<new client_id>
   VITE_SOCRATE_REDIRECT_URI=https://ascenda.vandermoten.eu/callback
   ```
9. **Harden token validation (W2, W6).**
   - Upgrade `backendkit` from v1.5.0 to the current v1.13.0, reading its migration notes, including the `bff.Gateway` rename at v1.11.
   - Pass `jwtauth.WithAudience(SOCRATE_CLIENT_ID)`; Socrate puts the `client_id` in `aud` (U3).
   - Derive the platform-admin role from `app_roles[client_id]`.
10. **Recommended follow-up (W1):** move token handling into a BFF (`backendkit/bff`), so the SPA holds only an httpOnly session cookie and refresh rotation runs server-side, coalesced per session.
11. **Deploy the backend and the frontend together, then run the test plan.** Keep the old IdP client active until T1–T6 pass, so you can roll back by restoring both env files.

---

## 5. Test plan (post-migration)

| # | Flow | Steps | Expected |
|---|---|---|---|
| T1 | Login (Authorization Code + PKCE) | Sign in from `/login`. In devtools, inspect the `/oauth/authorize` request, then the `/auth/callback` call. | Authorize carries `code_challenge_method=S256` and a random `state`; the OP accepts it; the callback returns 200; `/api/v1/users/me` returns the **existing** account and workspace (not a 403, and no newly provisioned user). This confirms the user IDs were carried over (U5). |
| T2 | Token validation via JWKS | Call an API route with a fresh access token; then with a token whose signature is altered; with a token for another client of the same issuer (after W2); with an expired token. | Fresh token 200; the altered, other-client and expired tokens 401. Backend logs show `JWKS keys refreshed` with a non-zero `key_count`. |
| T3 | Refresh-rotation survival | Stay signed in past the access-token lifetime (or shorten it on the OP) while using the app; repeat over at least three expiries. Trigger several parallel API calls at expiry. | Each expiry produces **one** `/auth/refresh` (single-flight), the stored refresh token **changes** each time, and the session continues with no logout. Before the B1 fix this test logs the user out. |
| T4 | Forced re-auth on reuse | Capture a refresh token, use it once (for example via `curl` against `/auth/refresh`), then let the app refresh with the same, now consumed, token. | The OP answers `invalid_grant` and revokes the chain; the app logs out and returns to sign-in; it does not retry the old token. |
| T5 | Logout | Sign out; replay the old refresh token at `/auth/refresh`. If RP-initiated logout was added, open `/login` again. | Logout returns 204 (not 400), and the replay fails with `invalid_grant`, which proves revocation (W3). With end-session, the OP asks for credentials again. |
| T6 | Magic link (if kept) | Request a link from the landing page, open it. | The authorize request carries PKCE; the user lands signed in on the requested page. |
| T7 | Machine-to-machine | Invite a user, register a new account, open *Team* (profile enrichment) and the admin user list. | One `client_credentials` token per ~expiry window in the OP logs, not one per user or request; admin-API calls succeed on the configured `SOCRATE_ADMIN_URL`. |
| T8 | Key rotation | Rotate the signing key on the OP (or wait for a scheduled rotation) while signed in. | Tokens signed with the new `kid` validate after one JWKS re-fetch; there is no outage. |
