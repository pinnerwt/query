# Cookie-Based Auth Design

**Goal:** Replace localStorage JWT with HttpOnly cookie for XSS protection and cross-tab session persistence.

**Architecture:** Server sets `HttpOnly; SameSite=Strict` cookie named `session` containing the JWT. Middleware reads from cookie, rotates token when `iat` is >1 hour old. Frontend uses `credentials: 'include'` — no JavaScript token handling.

## Changes

- `internal/auth/` — Cookie helpers (set/clear), middleware reads cookie + rotates hourly
- `cmd/server/main.go` — Login/register set cookie, new `POST /api/auth/logout`, pass secure flag
- `frontend/src/lib/api.ts` — `credentials: 'include'`, remove token header
- `frontend/src/lib/auth.tsx` — Remove localStorage, auth state from `/api/auth/me` only
- `frontend/src/pages/Login.tsx` — Don't extract token from response

## Security

- `HttpOnly` prevents XSS token theft
- `SameSite=Strict` prevents CSRF
- `Secure` flag set when base-url is HTTPS (omitted in dev)
- Token rotation every hour limits stolen cookie lifetime
- 24h expiry unchanged
