# Plan: CLI-Native GitHub App Authentication

> **Status:** Deferred. Not needed until non-GitHub-Actions CI support is added.
>
> **Trigger:** The milestone that adds Jenkins, GitLab CI, or other non-GitHub-Actions CI support. In those environments, `actions/create-github-app-token` is not available, so the CLI must generate installation tokens itself.
>
> **Decision record:** ADR-026

## Context

v2 CLI auth uses `actions/create-github-app-token` (ADR-026). This works because v2 targets GitHub Actions only. When the CLI runs in Jenkins, GitLab CI, or other environments, there is no equivalent action to pre-generate tokens. The CLI must handle JWT generation and token exchange internally.

## Scope

**New env vars (CLI):**

| Env Var | Description |
|---------|-------------|
| `QUARANTINE_APP_CLIENT_ID` | App client ID (JWT `iss` claim) |
| `QUARANTINE_APP_PRIVATE_KEY` | PEM contents (alternative to path) |
| `QUARANTINE_APP_PRIVATE_KEY_PATH` | Path to PEM file |
| `QUARANTINE_APP_INSTALLATION_ID` | GitHub installation ID (numeric). Required for CLI-native auth since the CLI targets a single repo/installation, unlike the dashboard which discovers installations dynamically. |

**Token resolution order (updated):**

```
1. QUARANTINE_GITHUB_TOKEN env var               -> StaticTokenProvider (PAT)
2. QUARANTINE_APP_CLIENT_ID + key + ID all set   -> InstallationTokenProvider (App)
3. GITHUB_TOKEN env var                           -> StaticTokenProvider (Actions default)
4. none                                           -> degraded mode (warning, not fatal)
```

**Components to build:**

1. **`GenerateJWT(clientID string, privateKeyPEM []byte, now time.Time) (string, error)`** -- Pure function. RS256 JWT with `iss` = client ID, `iat` = now-60s, `exp` = now+9min. Uses `golang-jwt/jwt`.

2. **`TokenProvider` interface** -- `Token(ctx context.Context) (string, error)`. Two implementations:
   - `StaticTokenProvider`: wraps a string (PAT or `GITHUB_TOKEN`).
   - `InstallationTokenProvider`: generates JWT, calls `POST /app/installations/{id}/access_tokens`, caches token, refreshes when < 5 min before expiry.

3. **`ResolveTokenProvider(envReader) (TokenProvider, error)`** -- Checks env vars in priority order, returns the appropriate provider.

4. **`Client` struct changes** -- `token string` field replaced by `TokenProvider`. `newRequestWithContext` calls `provider.Token(ctx)` instead of using a static field.

**Key design constraints:**

- `golang-jwt/jwt` for JWT generation (no `go-github` dependency).
- Installation token requests omit `permissions` body parameter (inherit App defaults).
- Token caching: at most one `POST /access_tokens` per ~55 minutes per installation.
- Auth failure -> degraded mode (build never broken).
- Pure function for JWT generation (accepts `time.Time` for deterministic testing).

## Test Plan

**Unit tests:**
- `GenerateJWT` produces valid RS256 JWT with correct claims
- Invalid PEM, public key, empty clientID return descriptive errors
- `ResolveTokenProvider` returns correct provider per env var priority
- Test keys generated in-test via `crypto/rsa.GenerateKey`

**Integration tests:**
- `httptest.Server` serves `POST /app/installations/{id}/access_tokens`
- Token caching: second call reuses cached token
- Token refresh: after simulated expiry, next call triggers exchange
- Network error: returns error, CLI degrades

**Contract tests:**
- `POST /app/installations/{id}/access_tokens` -> 201, 401, 404
- Uses existing `newPrismClient()` / `preferHeader()` helpers

## Dependencies

- Dashboard already implements JWT generation and token exchange in TypeScript (v2). The Go implementation follows the same spec but is independent code.
- Requires `golang-jwt/jwt` dependency added to `go.mod`.
