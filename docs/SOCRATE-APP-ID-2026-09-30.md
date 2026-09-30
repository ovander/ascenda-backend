# Socrate app ID for Ascenda: change request, closed

**Opened and closed:** 2026-09-30. **From:** Ascenda owner. **To:** Socrate team.
**Status:** closed, answered. Ascenda's app ID is `3`. Two gaps remain that the ID does not fix;
they are tracked as Socrate changes 1 and 2 below.

| | |
|---|---|
| Application | Ascenda (`ascenda.vandermoten.eu`) |
| OAuth client_id | `cVqYPgs3uGZYgw1x6F3aOQ` |
| App ID | `3` |
| Issuer | `https://socrate.vandermoten.eu` |
| Admin API | SSH tunnel from the apps VPS, `127.0.0.1:18082` → Socrate `127.0.0.1:8082` |
| Client library | `github.com/ovander/backendkit` v1.15.0, package `socrate` |
| Ascenda setting | `SOCRATE_APP_ID` |

## Why Ascenda needs it

backendkit's `socrate.Client` builds the admin API paths from the numeric app ID
(`/api/apps/{appID}/…`). Without `SOCRATE_APP_ID` it looks the ID up in `GET /api/admin/apps`,
which only a human administrator's JWT may call: a service-account token gets `401`, so every call
Ascenda makes as itself fails. `SOCRATE_APP_ID` was empty on Ascenda before and after the move to
the new Socrate.

## Answers from the Socrate team

| Question | Answer |
|---|---|
| Numeric app ID of `cVqYPgs3uGZYgw1x6F3aOQ`; is it stable? | **`3`**, the row's primary key on the new Socrate. Restarts, restores and future carry-overs don't change it (carry-overs never rewrite existing apps; new IDs start above 1011). It is not the legacy Ascenda ID (`6`, client `VowmS…`), which was not carried over. Check on the Socrate VPS: `sudo -u postgres psql -X -At -c "SELECT id FROM apps WHERE client_id='cVqYPgs3uGZYgw1x6F3aOQ'" socrate` prints `3`. |
| Is the ID public; can the client_id be used instead? | It is an identifier, not a secret. The `/api/apps/{id}/…` paths take only the numeric ID; no endpoint accepts the client_id. |
| What may the service account call? | Only `/api/apps/{id}/service/*`. The `/api/apps/{id}/users…` routes require an app admin's user JWT. A service token's subject is `app:3`, which is rejected with `401 invalid token claims` (Socrate `middleware/auth.go:89`). |
| Should a service token get `401` on `GET /api/admin/apps`? | Yes, by design: the admin endpoints serve human global admins only. There is no self-lookup for a service account, hence the `SOCRATE_APP_ID` setting. |
| Magic-link URL for this app? | There is no per-app magic-link URL. The e-mail always links to `https://socrate.vandermoten.eu/api/auth/magic-link/verify?token=…&client_id=…` (Socrate `magic_link_service.go:172`). That endpoint is POST-only, so that e-mail scanners can't spend the token, and clicking the link gets `405`, for every app. |

## Ascenda features with app ID 3

| Ascenda feature | backendkit call | Socrate route | Status | Unblocked by |
|---|---|---|---|---|
| Sign-in (password or SSO via `/bff/login`), session, refresh, sign-out | OAuth calls | `/oauth/*` | works | n/a (no app ID needed) |
| Workspace invitations | `InviteUserAsService` | `POST /api/apps/3/service/users` | works | n/a |
| Magic-link sign-in | `SendMagicLink` | `POST /api/apps/3/service/magic-link` | e-mail sent, link answers 405 | Socrate change 1 |
| Self-service sign-up | `RegisterUser` | `POST /api/apps/3/users` | fails (401; Ascenda answers 500) | Socrate change 2 and a backendkit release |
| Names and e-mails in team lists | `GetUserAsService` | `GET /api/apps/3/users/{id}` | skipped (fails silently, logged) | Socrate change 2 and a backendkit release |
| Platform-admin user management | `ListUsers`, `GetUser`, `CreateUser`, `UpdateUserRole`, `DeleteUser`, … | `/api/apps/3/users…` | app admins only | The signed-in admin must be an app admin of app 3 on Socrate |

## Follow-up changes

| Change | Owner | What Ascenda needs from it |
|---|---|---|
| **1. Per-app magic-link landing URL**, with `token` (and `client_id`) appended; new migration and an admin-console field. | Socrate | Ascenda's value: `https://ascenda.vandermoten.eu/magic-link`. The page already reads `?token=`, removes it from the history and POSTs it to Ascenda's `/bff/magic-link/verify`, so the token is redeemed on the server and never reaches the browser. The proposed Socrate-hosted fallback page must **not** apply to Ascenda: it would redeem the link at Socrate and hand the tokens to the browser. |
| **2. Service routes** for registration and a profile look-up (`GET /api/apps/{id}/service/users/{user_id}`), plus a backendkit minor release moving `RegisterUser` and `GetUserAsService` onto them. | Socrate, backendkit | Keep the contracts: `RegisterUser` returns the new `UserID` and `ErrUserAlreadyExists` on a duplicate (Ascenda answers 409); `GetUserAsService` returns a `*socrate.User`. Ascenda then needs only a backendkit bump. |
| **3. Optional:** `GET /api/service/self`, so a service account can read its own app ID. | Socrate, backendkit | Would make `SOCRATE_APP_ID` unnecessary; nothing blocks on it. |

## Ascenda actions

- Set `SOCRATE_APP_ID=3` in `/opt/apps/ascenda/env/.env` on the apps VPS and restart the service
  (sessions are in memory, so everyone signs in again once).
- Grant app-admin rights on app 3 at Socrate to the people who manage users in Ascenda's platform
  admin.
- After change 1: set the landing URL above on app 3 and test a magic link end to end.
- After change 2: bump backendkit in Ascenda (one-line PR) and test self-service sign-up.
- Optional until then: route sign-up through `InviteUserAsService` in a small Ascenda PR. The new
  user would get an invitation-style e-mail, and duplicate-account handling must be checked first.

## Original request, as sent

We asked for the numeric app ID of `cVqYPgs3uGZYgw1x6F3aOQ` and confirmation that it is stable; a
way for app owners to get it without admin rights (a console field, a self-lookup endpoint for the
client's service account, or the client_id accepted in `/api/apps/{app}/…` paths); and
confirmation that the service account may call the registration, invitation, magic-link and
profile endpoints.

One assumption in the request was wrong: we expected sign-up and profile look-ups to work once the
ID was set. They use user-JWT routes, so they need change 2 as well.
