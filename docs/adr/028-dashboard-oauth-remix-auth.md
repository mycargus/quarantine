# ADR-028: Dashboard OAuth via remix-auth

**Status:** Accepted
**Date:** 2026-04-05

## Context

The v2 dashboard requires GitHub OAuth for user login (US-2.1). ADR-008 specifies "GitHub OAuth via remix-auth + remix-auth-github." The initial GitHub App plan described a custom OAuth implementation (manual state parameter generation, PKCE code challenge, manual token exchange, custom session management). This contradicts ADR-008 and introduces significant security risk.

## Decision

Use `remix-auth` with the `remix-auth-github` strategy for dashboard OAuth. No custom OAuth implementation. The library handles OAuth redirect URL construction, state parameter (CSRF protection), authorization code exchange, and user profile fetching.

The application is responsible for:
- Session storage (SQLite-backed, server-side).
- User access token and refresh token management (store, refresh before expiry).
- Route protection (redirect unauthenticated users to login).
- User-to-installation mapping (filter repos by user access via `GET /user/installations`).

Verify `remix-auth` and `remix-auth-github` compatibility with Remix 3 before implementation. If incompatible, evaluate alternatives or contribute compatibility patches.

## Alternatives Considered

- **Custom OAuth implementation:** Rejected. OAuth is notoriously error-prone: CSRF vulnerabilities, token leakage, redirect URI attacks, timing attacks on state validation. Implementing OAuth from scratch is a tremendous security risk with no benefit over battle-tested libraries.
- **Alternative OAuth libraries (e.g., `arctic`, `lucia`):** Viable but `remix-auth-github` is the canonical solution for Remix + GitHub. ADR-008 already specifies it. Switching adds unnecessary divergence from existing documentation.

## Consequences

- (+) Battle-tested OAuth implementation reduces security risk.
- (+) Consistent with ADR-008 specification.
- (+) Less application code to maintain and audit.
- (+) Community-maintained: security patches flow from library updates.
- (-) Dependency on library compatibility with Remix 3 (must verify).
- (-) Less control over OAuth flow details (PKCE support depends on library implementation).
- (-) Application still owns session management and token refresh -- these are not trivial.
