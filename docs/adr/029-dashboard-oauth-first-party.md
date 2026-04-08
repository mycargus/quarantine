# ADR-029: Dashboard OAuth via First-Party Remix Auth

**Status:** Accepted
**Date:** 2026-04-07
**Supersedes:** [ADR-028](028-dashboard-oauth-remix-auth.md)

## Context

ADR-028 specified `remix-auth` (community, sergiodxa) with `remix-auth-github`
for dashboard OAuth. Since that decision:

1. `remix-auth` v4 targets React Router v7, not Remix 3. It depends on
   `createCookieSessionStorage` from `react-router`, which does not exist in
   Remix 3.
2. `remix-auth-github` v3 depends on `remix-auth` v4 -- also incompatible.
3. Remix 3 shipped first-party auth packages (2026-03-25):
   - `@remix-run/auth` v0.1.0 -- providers for GitHub, Google, Microsoft,
     Okta, Auth0, Facebook, X, and custom OIDC.
   - `@remix-run/auth-middleware` v0.1.0 -- `auth()` and `requireAuth()`
     middleware for route protection.
   - `@remix-run/session` v0.4.1 -- session management with cookie, fs, and
     memory backends.
   - `@remix-run/session-middleware` v0.2.0 -- session middleware for the
     Remix 3 router.

ADR-005 already anticipated this: *"`remix/auth` ships providers for GitHub ...
No third-party auth library needed."*

## Decision

Use Remix 3's first-party auth packages for dashboard OAuth. No community
`remix-auth` or custom OAuth implementation.

**Packages:**

| Package | Version | Purpose |
|---------|---------|---------|
| `@remix-run/auth` | 0.1.0 | `createGitHubAuthProvider`, `startExternalAuth`, `finishExternalAuth`, `completeAuth` |
| `@remix-run/auth-middleware` | 0.1.0 | `auth()` identity resolution, `requireAuth()` route protection |
| `@remix-run/session` | 0.4.1 | `Session` class |
| `@remix-run/session-middleware` | 0.2.0 | `session()` middleware |

**GitHub provider configuration:**

```typescript
createGitHubAuthProvider({
  clientId: env.QUARANTINE_APP_CLIENT_ID,
  clientSecret: env.QUARANTINE_APP_CLIENT_SECRET,
  redirectUri: new URL('/auth/github/callback', env.QUARANTINE_APP_ORIGIN),
})
```

**Application responsibilities:**

- Session management: `createCookieSessionStorage` from `@remix-run/session/cookie-storage`
  (the root `@remix-run/session` exports only the `Session` class) with `maxAge: 28800` (8 hours). The encrypted cookie holds the access token
  and user profile. No server-side session table. No refresh tokens.
- Session lifetime matches GitHub access token lifetime (8 hours). When the
  cookie expires, the user re-authenticates via OAuth. This eliminates refresh
  token rotation handling entirely.
- User-to-installation filtering via `GET /user/installations` then
  `GET /user/installations/{id}/repositories` per installation.

**Route protection pattern:**

```typescript
// Router-level: resolve auth state on every request
middleware: [
  session(sessionCookie, sessionStorage),
  auth({ schemes: [createSessionAuthScheme({ ... })] }),
]

// Route-level: protect specific routes
middleware: [requireAuth()]
// Returns 401 by default for unauthenticated requests
```

## Alternatives Considered

- **`remix-auth` v4 + `remix-auth-github` v3 (ADR-028):** Incompatible with
  Remix 3. Built for React Router v7.
- **`better-auth`:** Framework-agnostic TypeScript auth library. Has a Remix
  integration guide but targets Remix v2 / React Router v7 patterns
  (loaders/actions from `@remix-run/node`). Heavier dependency than the
  first-party packages.
- **Custom OAuth implementation:** Rejected for the same reasons as ADR-028.

## Consequences

- (+) First-party packages maintained by the Remix team -- guaranteed Remix 3
  compatibility.
- (+) Consistent with ADR-005 which already specifies first-party auth.
- (+) Built-in PKCE support (S256 code challenge) in the GitHub provider.
- (+) `requireAuth()` middleware returns 401 by default -- matches FR-1.15.7
  with minimal code.
- (+) Community `remix-auth` and `remix-auth-github` removed as dependencies.
- (-) `@remix-run/auth` v0.1.0 is pre-1.0. API may change. Pin exact versions.
- (-) No token refresh -- users re-authenticate every 8 hours.
  Acceptable for a developer dashboard.
