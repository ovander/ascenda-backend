# Socrate v1.3.0 compatibility report: Ascenda

**Scope:** the two repositories that make up Ascenda, audited together because the OAuth integration is split between them:

- `ovander/ascenda-backend`, `main` at `5d41054`. Go API; confidential OAuth client; uses `github.com/ovander/backendkit` **v1.5.0**.
- `ovander/ascenda-frontend`, `main` at `e47f87e`. Vue 3 SPA; starts the login and holds the tokens.

The same report is in the root of both repositories. Paths are prefixed `backend/` or `frontend/` for the repository they belong to. `backendkit/` refers to the module source `github.com/ovander/backendkit@v1.5.0`.

**Date:** 2026-09-29. This is a read-only audit: no application code was changed.

**Discovery document:** not fetched. This environment's egress proxy refused `https://socrate.vandermoten.eu/.well-known/openid-configuration` (`CONNECT tunnel failed, response 403`). Every conclusion that depends on the live discovery values is marked **UNKNOWN** and says what to check.

---

## 1. Verdict

**Compatible with changes: 2 blockers · 7 warnings · 7 unknowns.**

The main interactive login already fits the Socrate v1.3.0 contract:
- Authorization Code with S256 PKCE, started by the SPA;
- code exchanged server-side by the confidential backend;
- access tokens validated as RS256 JWTs against JWKS, keyed by `kid`, with an issuer check.

No implicit flow, password grant or HS256 appears anywhere.

The two blockers are in flows around that login:

1. **Refresh-token rotation is broken.** The SPA reads the refresh response in a shape the backend never sends. The rotated refresh token is therefore thrown away, and the user is logged out at every access-token expiry.
2. **Magic-link sign-in starts an authorization request without PKCE.** Socrate v1.3.0 rejects it.

Both are small, contained code fixes. The unknowns are mostly configuration facts to confirm against the live discovery document. One of them decides whether existing users keep their workspaces after the switch: **U5, continuity of the `sub` claim**.

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
| W2 | WARNING | Token validation / authorization | `backendkit/jwtauth/middleware.go:191-196`; `backend/cmd/server/bootstrap.go:145`; admin role `backend/internal/middleware/tenant.go:79` | No `aud` check. On a Socrate instance that serves several apps (same `iss`), an access token minted for **another client** is accepted here. Platform-admin rights come from the top-level `role` claim, so an `admin` of another app may become an Ascenda platform admin (audit finding S-M1). `exp` is not required either. | Upgrade `backendkit` (≥ v1.8 adds `jwtauth.WithAudience`; ≥ v1.10 requires `exp` and adds leeway) and pass `WithAudience(SOCRATE_CLIENT_ID)`, once U3 confirms Socrate puts the `client_id` in `aud`. Take the admin role from `app_roles[client_id]` rather than the top-level `role`. |
| W3 | WARNING | Revocation / logout | `frontend/src/stores/auth.ts:50`; `backend/internal/handler/auth_handler.go:281-283` | The SPA sends `{refreshToken}`; the backend requires `{token}` (`validate:"required"`), answers 400, and the SPA ignores the error. The refresh token is therefore **never revoked** at logout, and its rotation chain stays valid until it expires. There is also no RP-initiated logout, so the Socrate SSO session survives an Ascenda logout. | Send `{ token: refreshToken }`. Also send `token_type_hint=refresh_token` if discovery lists it. If discovery has `end_session_endpoint`, redirect to it with `id_token_hint` and a registered `post_logout_redirect_uri`. |
| W4 | WARNING | PKCE (req. 2) | `backend/internal/handler/auth_handler.go:49-69`; route `router.go:102` | `POST /auth/login` returns an authorize URL without `code_challenge` or `state`. Nothing in the frontend calls it, but the route is live and would produce an authorization request that Socrate v1.3.0 rejects. | Remove the endpoint, or make it generate PKCE S256 + `state`. |
| W5 | WARNING | PKCE hardening | `backend/internal/handler/auth_handler.go:73-75,116-118` | `/auth/callback` treats `codeVerifier` as optional and forwards the code without one. Socrate enforces PKCE, so this does not break compatibility, but the client should not permit a PKCE-less exchange. | Make `codeVerifier` required (43–128 unreserved characters). Pin `redirect_uri` to the configured value instead of accepting it from the request body (`auth_handler.go:104-107`). |
| W6 | WARNING | JWKS handling | `backendkit/jwtauth/middleware.go:215-240` | An unknown `kid` triggers a synchronous JWKS fetch on **every** request, with no cooldown. Rotation works (a new `kid` is fetched on first sight), but a stream of tokens with random `kid`s turns into a stream of requests to Socrate. | Upgrade `backendkit` (≥ v1.10: refetch cooldown + negative cache, `WithMinRefetchInterval`). |
| W7 | WARNING | M2M rate limits | `backendkit/socrate/client.go:257-300`; loop at `backend/internal/service/user_service.go:157-166` | The `client_credentials` token is cached, so there is no hot-loop minting in the normal case. But when minting **fails** nothing is cached and there is no back-off. The per-user enrichment loop in `ListUsers` then calls `/oauth/token` once per user until its 2 s deadline, against a bcrypt-verified, rate-limited endpoint. | Cache the failure briefly (back-off), or skip enrichment after the first token failure in a request. |
| U1 | UNKNOWN | Endpoints (req. 7) | `frontend/src/composables/useAuth.ts:64`; `backend/internal/handler/auth_handler.go:56,119,223,298`; `magic_link_handler.go:97`; `backendkit/socrate/client.go:680,720` | Every OAuth path is hard-coded as `{base}/oauth/{authorize,token,revoke,userinfo}`. If the v1.3.0 discovery document names different paths, every flow breaks. | Compare `authorization_endpoint`, `token_endpoint`, `revocation_endpoint` and `userinfo_endpoint` in discovery with these paths. Ideally, read them from discovery at startup. |
| U2 | UNKNOWN | Client authentication | `backend/internal/handler/auth_handler.go:109-115,217-222,293-297`; `backendkit/socrate/client.go:268-272` | Every token/revoke call sends the secret in the form body (`client_secret_post`). | Confirm `token_endpoint_auth_methods_supported` (and `revocation_endpoint_auth_methods_supported`) includes `client_secret_post`. If only `client_secret_basic` is supported, switch to HTTP Basic. |
| U3 | UNKNOWN | Scopes / audience | `frontend/src/composables/useAuth.ts:58` | The SPA requests `openid email profile api`. `api` is non-standard, and an unsupported scope makes `/authorize` fail with `invalid_scope`. It is also unknown what Socrate puts in `aud` (needed for W2). | Check `scopes_supported` and the client's allowed scopes; decode a v1.3.0 access token and read `aud`, `iss`, `role`, `app_roles`, `exp`. |
| U4 | UNKNOWN | JWKS format (req. 5) | `backendkit/jwtauth/middleware.go:261-264` | Keys are kept only when `kty=="RSA"` **and** `use=="sig"`. If the v1.3.0 JWKS omits `use`, no key is loaded and every request gets 401. | `curl -s <jwks_uri> \| jq '.keys[] \| {kid,kty,use,alg}'`: every signing key must carry `"use":"sig"`. |
| U5 | UNKNOWN | Issuer change / user identity (req. 7) | `backend/internal/middleware/tenant.go:144,172,215`; `backend/internal/repo/user_repo.go:30-60` | Users are matched only by `external_id = sub`. If `socrate.vandermoten.eu` is a **new** user database rather than the same one under a new hostname, every `sub` changes. Existing users then match no record and get **403** in production, losing access to their workspace. | Confirm with the Socrate operator whether `sub` values survive. If not, before cut-over, remap `users.external_id` to the new subjects by e-mail, or blank `external_id` so that step 2 of the tenant resolution (claim by e-mail) relinks each account on first login. This requires identical e-mails in the new IdP. |
| U6 | UNKNOWN | Socrate admin API | `backendkit/socrate/client.go:95-101,183-243,616-680,802-815`; `backend/internal/config/config.go:56,157` | Registration, invitations, admin user management, profile enrichment and magic-link e-mails use Socrate's proprietary admin API. It sits on a separate port, which defaults to `https://socrate.vandermoten.eu:8081` when `SOCRATE_ADMIN_URL` is unset. That API is outside the OAuth contract, and the port may not be public. | Confirm the admin API (same paths) is available on v1.3.0 and set `SOCRATE_ADMIN_URL` explicitly. Set `SOCRATE_APP_ID` too: without it the app-ID lookup needs an admin JWT and fails for service calls (`client.go:196-216`). |
| U7 | UNKNOWN | Current backend configuration | `/opt/apps/ascenda/env/.env` on the VPS (not in the repo) | The live values of `SOCRATE_BASE_URL` (also used as the expected `iss`), `SOCRATE_JWKS_URL`, `SOCRATE_ADMIN_URL`, `SOCRATE_APP_ID` and `SOCRATE_REDIRECT_URL` could not be read. | Read them on the VPS during the migration (checklist step 4). |
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
4. **Confirm the unknowns against discovery**, from a machine that can reach the OP:
   ```bash
   curl -s https://socrate.vandermoten.eu/.well-known/openid-configuration | jq '{issuer, authorization_endpoint, token_endpoint, revocation_endpoint, userinfo_endpoint, end_session_endpoint, jwks_uri, scopes_supported, token_endpoint_auth_methods_supported, code_challenge_methods_supported, grant_types_supported}'
   curl -s "$(curl -s https://socrate.vandermoten.eu/.well-known/openid-configuration | jq -r .jwks_uri)" | jq '.keys[] | {kid,kty,use,alg}'
   ```
   - `issuer` must be exactly `https://socrate.vandermoten.eu`.
   - The endpoints must be `…/oauth/*` (U1).
   - `client_secret_post` must be supported (U2).
   - `api` must be in the scopes, or be removed from `useAuth.ts:58` (U3).
   - Keys must carry `use: "sig"` (U4).
   - Record the current VPS values (U7): `sudo grep -E '^SOCRATE_' /opt/apps/ascenda/env/.env | sed -E 's/(SECRET=).*/\1***/'`.
5. **Settle user identity (U5).** Find out whether `sub` values carry over. If they don't, prepare and test the `external_id` remap (by e-mail, or by blanking it for the claim-by-e-mail path) and take a database backup before cut-over.
6. **Register the client on `socrate.vandermoten.eu`:**
   - confidential client;
   - grants: `authorization_code`, `refresh_token`, `client_credentials`;
   - PKCE S256 required;
   - redirect URI registered exactly: `https://ascenda.vandermoten.eu/callback`, plus any dev URIs such as `http://localhost:5173/callback`;
   - if RP-initiated logout is added, a post-logout redirect URI;
   - allowed scopes: `openid profile email` (+ `api` if kept);
   - admin-API access for the service account.

   Record the new `client_id`, `client_secret` and the numeric app ID.
7. **Backend configuration** (`/opt/apps/ascenda/env/.env`):
   ```
   SOCRATE_BASE_URL=https://socrate.vandermoten.eu          # also the expected iss: no trailing slash
   SOCRATE_JWKS_URL=<jwks_uri from discovery>
   SOCRATE_CLIENT_ID=<new client_id>
   SOCRATE_CLIENT_SECRET=<new secret>                         # env file only, root-owned; never committed
   SOCRATE_APP_ID=<numeric app id>
   SOCRATE_ADMIN_URL=<admin API base on v1.3.0>               # do not rely on the :8081 default
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
   - Pass `jwtauth.WithAudience(SOCRATE_CLIENT_ID)` once U3 confirms the `aud` value.
   - Derive the platform-admin role from `app_roles[client_id]`.
10. **Recommended follow-up (W1):** move token handling into a BFF (`backendkit/bff`), so the SPA holds only an httpOnly session cookie and refresh rotation runs server-side, coalesced per session.
11. **Deploy the backend and the frontend together, then run the test plan.** Keep the old IdP client active until T1–T6 pass, so you can roll back by restoring both env files.

---

## 5. Test plan (post-migration)

| # | Flow | Steps | Expected |
|---|---|---|---|
| T1 | Login (Authorization Code + PKCE) | Sign in from `/login`. In devtools, inspect the `/oauth/authorize` request, then the `/auth/callback` call. | Authorize carries `code_challenge_method=S256` and a random `state`; the OP accepts it; the callback returns 200; `/api/v1/users/me` returns the **existing** account and workspace (not a 403, and no newly provisioned user). This confirms U5. |
| T2 | Token validation via JWKS | Call an API route with a fresh access token; then with a token whose signature is altered; with a token for another client of the same issuer (after W2); with an expired token. | Fresh token 200; the altered, other-client and expired tokens 401. Backend logs show `JWKS keys refreshed` with a non-zero `key_count`. |
| T3 | Refresh-rotation survival | Stay signed in past the access-token lifetime (or shorten it on the OP) while using the app; repeat over at least three expiries. Trigger several parallel API calls at expiry. | Each expiry produces **one** `/auth/refresh` (single-flight), the stored refresh token **changes** each time, and the session continues with no logout. Before the B1 fix this test logs the user out. |
| T4 | Forced re-auth on reuse | Capture a refresh token, use it once (for example via `curl` against `/auth/refresh`), then let the app refresh with the same, now consumed, token. | The OP answers `invalid_grant` and revokes the chain; the app logs out and returns to sign-in; it does not retry the old token. |
| T5 | Logout | Sign out; replay the old refresh token at `/auth/refresh`. If RP-initiated logout was added, open `/login` again. | Logout returns 204 (not 400), and the replay fails with `invalid_grant`, which proves revocation (W3). With end-session, the OP asks for credentials again. |
| T6 | Magic link (if kept) | Request a link from the landing page, open it. | The authorize request carries PKCE; the user lands signed in on the requested page. |
| T7 | Machine-to-machine | Invite a user, register a new account, open *Team* (profile enrichment) and the admin user list. | One `client_credentials` token per ~expiry window in the OP logs, not one per user or request; admin-API calls succeed on the configured `SOCRATE_ADMIN_URL`. |
| T8 | Key rotation | Rotate the signing key on the OP (or wait for a scheduled rotation) while signed in. | Tokens signed with the new `kid` validate after one JWKS re-fetch; there is no outage. |
