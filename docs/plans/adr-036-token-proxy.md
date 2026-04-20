# Plan: v2 CLI Auth — GITHUB_TOKEN Default + Dashboard Token Proxy

> **Status:** approved
> **Created:** 2026-04-20
> **Source:** [ADR-036](../adr/036-v2-cli-auth-github-token-default.md),
> [v2-auth-token-proxy.md](../specs/v2-auth-token-proxy.md)

## Context

v1 requires users to create a PAT and store it as a CI secret. v2 eliminates
this friction: `GITHUB_TOKEN` (auto-provisioned by GitHub Actions) is the
default. For repos that hit the 1,000 req/hr limit, the dashboard provides an
optional token proxy — the CLI sends `GITHUB_TOKEN` to the dashboard, which
mints a scoped installation token with 5,000-12,500 req/hr. If the proxy
fails, the CLI falls back to `GITHUB_TOKEN`. The build never breaks.

## Requirements

| ID | Summary | Source | FR/NFR | Priority |
|----|---------|--------|--------|----------|
| R-1 | GITHUB_TOKEN as default CLI auth | ADR-036 #decision | Revises FR-1.11.1 | must |
| R-2 | Token resolution: QUARANTINE_GITHUB_TOKEN → proxy → GITHUB_TOKEN → exit 2 | ADR-036 #token-resolution-order | NEW | must |
| R-3 | Dashboard POST /api/ci-token endpoint | ADR-036 #apici-token-security-model | NEW | must |
| R-4 | Two-phase token verification (soft prefix + behavioral) | ADR-036 #apici-token-security-model | NEW | must |
| R-5 | Proxy timeout: 3s default/minimum, via QUARANTINE_DASHBOARD_TIMEOUT | ADR-036 #dashboard-proxy-timeout | NFR-2.1.1 | must |
| R-6 | HTTPS enforcement for QUARANTINE_DASHBOARD_URL | ADR-036 #https-enforcement | NEW (NFR-2.3.x) | must |
| R-7 | Dashboard URL env-var only (QUARANTINE_DASHBOARD_URL) | ADR-036 #dashboard-env-vars | SEC-4 | must |
| SEC-1 | Token handling hygiene: never log, never persist, discard | v2-auth-token-proxy #sec-1 | NEW (NFR-2.7.x) | must |
| SEC-2 | Verification cache: SHA-256 keyed, 15s TTL, with eviction | v2-auth-token-proxy #sec-2 | NEW (NFR-2.8.x) | must |
| SEC-3 | App permissions MUST NOT exceed CLI requirements | v2-auth-token-proxy #sec-3 | Extends NFR-2.7.1 | must |
| SEC-5 | Localhost HTTPS exception disabled when CI=true | v2-auth-token-proxy #sec-5 | NEW (NFR-2.3.x) | must |
| SEC-6 | Rate limiting: per-repo + per-token-fingerprint (no per-IP) | v2-auth-token-proxy #sec-6 | Extends NFR-2.3.3 | must |
| SEC-6a | Scoped token minting with repositories parameter | v2-auth-token-proxy #sec-6a | NEW | must |
| SEC-6b | SSRF protection: behavioral check target URL hardcoded | v2-auth-token-proxy #sec-6b | NEW | must |
| SEC-7 | CLI skips quarantine on fork PRs | v2-auth-token-proxy #sec-7 | NEW (NFR-2.3.x) | must |
| SEC-8 | Revocation procedure documented | v2-auth-token-proxy #sec-8 | — | should |
| SEC-9 | Event payload parsing hardening | v2-auth-token-proxy #sec-9 | NEW | must |
| UX-1 | Missing permissions block: actionable 403 error + GHA annotation | v2-auth-token-proxy #ux-1 | NEW (NFR-2.11.x) | must |
| UX-2 | Error messages differentiated by token resolution step | v2-auth-token-proxy #ux-2 | NFR-2.11.4 | must |
| UX-3 | Proxy failure warning with URL + fallback rate limit | v2-auth-token-proxy #ux-3 | NFR-2.11.4 | must |
| UX-4 | Notice when QUARANTINE_GITHUB_TOKEN skips proxy | v2-auth-token-proxy #ux-4 | — | should |
| UX-5 | Notice when timeout clamped to 3s minimum | v2-auth-token-proxy #ux-5 | — | should |
| UX-6 | Context-aware missing-token message (GHA vs non-GHA) | v2-auth-token-proxy #ux-6 | NFR-2.11.4 | must |
| UX-8 | Dashboard proxy discoverability via rate limit warning | v2-auth-token-proxy #ux-8 | — | should |
| ERR-1 | "No token" changes from degraded mode to exit 2 | v2-auth-token-proxy #4 | Revises error-handling.md | must |
| ERR-2 | New error category: Dashboard Proxy Errors | v2-auth-token-proxy #4 | Extends error-handling.md | must |
| ERR-3 | Proxy-minted token 401 gets distinct message | v2-auth-token-proxy #4 | Extends error-handling.md | must |
| OPS-1 | App permission verification at dashboard startup | v2-auth-token-proxy #ops-1 | NEW (NFR-2.9.x) | must |
| OPS-2 | pull_request_target warning in setup guide | v2-auth-token-proxy #ops-2 | — | should |
| UX-7 | Quick-start docs: init → permissions → run | v2-auth-token-proxy #ux-7 | — | should (doc review) |
| E2E-1 | CLI + dashboard proxy round-trip E2E test | v2-auth-token-proxy #e2e-1 | — | must |

**Excluded:** R-8 (bot identity — automatic, not implemented).
**Deferred:** OPS-3 (GHES note), OPS-4 (actions:read note), SEC-4 item 3 (CLI URL change warning).

## Milestones

### M17: CLI v2 Token Resolution

**Dependencies:** M15 (github-app mode complete)

**Scope — included:**
- R-1, R-2: GITHUB_TOKEN default, four-step token resolution order
- R-5, R-6, R-7: Proxy timeout, HTTPS enforcement, env-var-only dashboard URL
- SEC-5: Localhost exception restricted to CI
- SEC-7: Fork PR detection → skip quarantine
- SEC-9: Event payload parsing hardening
- UX-1 through UX-6, UX-8: All error messages and notices
- ERR-1, ERR-2, ERR-3: Error handling spec updates
- OPS-2: pull_request_target warning (doc review)
- UX-7: Quick-start docs update (doc review)

**Scope — excluded:**
- Dashboard proxy endpoint (M18)
- E2E round-trip testing (M19)
- GHES rate limit note, actions:read note (deferred)

**Acceptance criteria:**
1. `GITHUB_TOKEN` is used by default when no other token is configured (R-1, FR-1.11.1 revised)
2. Token resolution follows four-step order and stops at first success (R-2)
3. `QUARANTINE_DASHBOARD_URL` must use HTTPS; rejected otherwise (R-6)
4. Localhost HTTPS exception allowed only when `CI` is not set (SEC-5)
5. `QUARANTINE_DASHBOARD_TIMEOUT` defaults to 3s, minimum 3s, values below clamped with notice (R-5, UX-5)
6. Dashboard URL only from `QUARANTINE_DASHBOARD_URL` env var — no config file option (R-7)
7. Fork PR detected → quarantine skipped, raw test command executed (SEC-7)
8. Malformed/missing `GITHUB_EVENT_PATH` → fork detection skipped, quarantine runs normally (SEC-9)
9. 403 on write → actionable "add permissions: block" error with SSO/IP fallback hint + GHA `::error` annotation (UX-1)
10. Each token resolution failure path produces a distinct, actionable error message (UX-2)
11. Proxy failure warning includes URL, reason, fallback rate limit + GHA `::warning` annotation (UX-3)
12. No token available → exit 2 (not degraded mode) (ERR-1)
13. Error-handling spec updated: degraded mode trigger 3 changed, Category 4 added, proxy-minted 401 distinct (ERR-1, ERR-2, ERR-3)
14. `quarantine doctor` mentions `QUARANTINE_DASHBOARD_URL` upgrade path when rate limit is low (UX-8)
15. CI integration docs updated with `pull_request_target` warning and full onboarding sequence (OPS-2, UX-7 — verified by doc review)

**Scenario outlines:**

Token resolution:
- Given QUARANTINE_GITHUB_TOKEN set → uses it, skips proxy + GITHUB_TOKEN → R-2
- Given proxy configured + returns valid token → uses proxy token → R-2
- Given proxy fails (timeout/4xx/5xx) → falls back to GITHUB_TOKEN → R-2
- Given no proxy, GITHUB_TOKEN set → uses GITHUB_TOKEN → R-1, R-2
- Given no token anywhere → exit 2 with error → R-2, ERR-1

Config + HTTPS:
- Given QUARANTINE_DASHBOARD_URL is https → accepted → R-6
- Given QUARANTINE_DASHBOARD_URL is http (not localhost) → rejected → R-6
- Given QUARANTINE_DASHBOARD_URL is http://localhost, CI not set → accepted → R-6
- Given QUARANTINE_DASHBOARD_URL is http://localhost, CI=true → rejected → SEC-5
- Given QUARANTINE_DASHBOARD_TIMEOUT is 1s → clamped to 3s with notice → R-5, UX-5
- Given QUARANTINE_DASHBOARD_TIMEOUT is invalid (e.g., "abc") → default 3s used → R-5

Fork PR detection:
- Given pull_request event, head repo != base repo → skip quarantine, exec raw command → SEC-7
- Given pull_request event, head repo == base repo → quarantine runs normally → SEC-7
- Given push event → fork detection does not apply → SEC-7
- Given GITHUB_EVENT_PATH is malformed JSON → skip detection, quarantine runs → SEC-9
- Given GITHUB_EVENT_PATH unset → skip detection, quarantine runs → SEC-9
- Given event payload missing head.repo.full_name → skip detection → SEC-9

Error messages:
- Given GITHUB_TOKEN resolved, first write returns 403 → permissions block error + GHA annotation → UX-1
- Given QUARANTINE_GITHUB_TOKEN returns 401 → "invalid or expired" message → UX-2
- Given proxy-minted token returns 401 → "Dashboard-minted token rejected" → ERR-3
- Given proxy times out → warning with URL + timeout duration + GHA annotation → UX-3
- Given QUARANTINE_GITHUB_TOKEN set + QUARANTINE_DASHBOARD_URL set → "proxy skipped" notice → UX-4
- Given QUARANTINE_DASHBOARD_URL set, no GITHUB_TOKEN, GITHUB_ACTIONS=true → "check permissions block" → UX-6
- Given QUARANTINE_DASHBOARD_URL set, no GITHUB_TOKEN, not in GHA → "set token or run in GHA" → UX-6

**Spec references:**

| What | Reference |
|------|-----------|
| Token resolution (v1) | cli-spec.md (token resolution section) |
| Exit codes | error-handling.md#exit-codes |
| Degraded mode triggers | error-handling.md#degraded-mode-triggers |
| GHA annotations | error-handling.md#degraded-mode-communication |
| Error prefixes | non-functional-requirements.md NFR-2.11.4 |
| CLI overhead budget | non-functional-requirements.md NFR-2.1.1 |
| v2 auth requirements | v2-auth-token-proxy.md |
| ADR-036 decision | adr/036-v2-cli-auth-github-token-default.md |

---

### M18: Dashboard Token Proxy

**Dependencies:** M15 (installation token infrastructure exists)

**Scope — included:**
- R-3, R-4: POST /api/ci-token endpoint, two-phase verification
- SEC-1: Token handling hygiene
- SEC-2: Verification cache (SHA-256 keyed, 15s TTL, with eviction)
- SEC-3: App permission parity (hard requirement)
- SEC-6: Rate limiting (per-repo + per-fingerprint, no per-IP)
- SEC-6a: Scoped token minting with repositories parameter
- SEC-6b: SSRF protection on behavioral check
- SEC-8: Revocation procedure (doc review)
- OPS-1: App permission verification at startup

**Scope — excluded:**
- CLI token resolution changes (M17)
- E2E round-trip testing (M19)
- Reuse of existing InstallationTokenProvider without modification (SEC-6a explicitly prohibits this)

**Acceptance criteria:**
1. POST /api/ci-token accepts tokens, returns scoped installation tokens (R-3)
2. Known-bad prefixes (ghp_, github_pat_, ghu_, gho_) rejected with 404 immediately; unknown prefixes fall through to behavioral check (R-4)
3. Behavioral check calls GET /installation/repositories on hardcoded GitHub API base URL (R-4, SEC-6b)
4. Received GITHUB_TOKEN never logged, never persisted, discarded after verification (SEC-1)
5. Verification cache keyed by SHA-256(token)[0:16]:owner/repo, 15s TTL, expired entries evicted (SEC-2)
6. Installation token minting passes repositories: ["<repo>"] to scope token to single repo (SEC-6a)
7. Installation token cache per repo, 1hr TTL, proactive refresh at <5min (R-3)
8. Rate limiting: 10/min per repo, 10/min per token fingerprint; 429 with Retry-After on excess (SEC-6)
9. Rate limit counters evict expired entries to prevent memory leak (SEC-6)
10. App permissions beyond SEC-3 table logged as warning at startup (OPS-1)
11. App permissions exactly matching SEC-3 table: no warning (OPS-1)
12. 404 returned for non-matching repos — no info leakage (R-3)
13. Revocation procedure documented in operational runbook (SEC-8 — doc review)

**Scenario outlines:**

Endpoint + verification:
- Given valid ghs_ token for repo with App installed → returns scoped installation token → R-3, R-4
- Given ghp_-prefixed token → 404 immediately (known-bad prefix) → R-4
- Given token with unrecognized prefix → falls through to behavioral check → R-4
- Given ghs_ token for repo NOT in any installation → 404 → R-3
- Given no Authorization header → 404 → R-3

Token hygiene + caching:
- Given a valid request → received GITHUB_TOKEN never appears in logs → SEC-1
- Given same token, same repo within 15s → verification cache hit → SEC-2
- Given same token, different repo → verification cache miss → SEC-2
- Given different token, same repo → verification cache miss → SEC-2
- Given same token, same repo after 15s → verification cache miss (expired) → SEC-2

Token minting:
- Given verified request, no cached installation token → mints with repositories: ["<repo>"] → SEC-6a
- Given cached installation token with >5min remaining → returns cached token → R-3
- Given two concurrent requests for same repo → second receives cached token, no double-mint → R-3

Rate limiting:
- Given 11th request for same repo in one minute → 429 with Retry-After → SEC-6
- Given 11th request with same token fingerprint in one minute → 429 → SEC-6

Permission verification:
- Given App has administration:write (beyond SEC-3 table) → startup warning names the excess permission → OPS-1
- Given App has exactly required permissions → no warning → OPS-1

SSRF protection:
- Given behavioral check → target URL is hardcoded GitHub API base, not influenced by request input → SEC-6b

**Spec references:**

| What | Reference |
|------|-----------|
| Installation token provider (existing) | non-functional-requirements.md NFR-2.8.1 |
| Rate limiting | non-functional-requirements.md NFR-2.3.3 |
| App auth security | non-functional-requirements.md NFR-2.7.x |
| Token proxy requirements | v2-auth-token-proxy.md#1-security-requirements |
| Dashboard error handling | error-handling.md#category-3-dashboard-errors |
| ADR-036 security model | adr/036-v2-cli-auth-github-token-default.md#apici-token-security-model |
| GitHub API inventory | github-api-inventory.md |

---

### M19: v2 Auth E2E

**Dependencies:** M17 + M18

**Scope — included:**
- E2E-1: Full round-trip and fallback tests

**Scope — excluded:**
- CLI or dashboard code changes (those are complete in M17/M18)

**Acceptance criteria:**
1. E2E test verifies full round-trip: CLI → proxy → installation token → API calls succeed with quarantine[bot] identity (E2E-1)
2. E2E test verifies fallback: dashboard unreachable → CLI uses GITHUB_TOKEN within timeout window (E2E-1)
3. E2E test verifies fallback: dashboard returns 404 (repo not in installation) → CLI uses GITHUB_TOKEN (E2E-1)

**Scenario outlines:**
- Given CLI with QUARANTINE_DASHBOARD_URL, fixture repo with App installed → obtains proxy token, API calls use quarantine[bot] identity → E2E-1
- Given CLI with QUARANTINE_DASHBOARD_URL, dashboard unreachable → falls back to GITHUB_TOKEN → E2E-1
- Given CLI with QUARANTINE_DASHBOARD_URL, repo not in App installation → falls back to GITHUB_TOKEN → E2E-1

**Spec references:**

| What | Reference |
|------|-----------|
| E2E test conventions | test/e2e/README.md |
| E2E requirements | v2-auth-token-proxy.md#5-e2e-test-coverage |
| Test strategy | test-strategy.md#test-layers |

## Spec updates required

- **error-handling.md**: Change degraded mode trigger 3 to exit 2; add Category 4 (Dashboard Proxy Errors); add proxy-minted token 401 message variant
- **cli-spec.md**: Update token resolution section for v2 order
- **non-functional-requirements.md**: Add NFR entries for SEC-2, SEC-5, SEC-6, SEC-6a, SEC-6b, SEC-7, SEC-9, UX-1, OPS-1
- **github-api-inventory.md**: Add GET /installation/repositories (verification call)
- **functional-requirements.md**: Revise FR-1.11.1 (GITHUB_TOKEN default); add FR for token resolution order, proxy endpoint, fork PR skip

## Risks and open questions

- **Risk:** The existing `InstallationTokenProvider` does not pass `repositories` when minting tokens. M18 must modify it or create a separate code path. If reused incorrectly, tokens grant access to all repos in the installation (privilege escalation). Mitigated by SEC-6a as explicit acceptance criterion.
- **Risk:** `ghs_` prefix is observed behavior, not a GitHub guarantee. If GitHub changes token formats, the soft prefix gate falls through to the behavioral check (by design). No action needed but worth monitoring.
- **Risk:** Rate limit counter Maps can grow unboundedly. SEC-6 requires eviction. The existing `createIpRateLimiter`/`createUserRateLimiter` middleware has the same issue — fix as part of M18.
- **Open question:** Session secret defaults to hardcoded string in `app/app.ts`. Not directly related to /api/ci-token but shared router. Dashboard should refuse to start without SESSION_SECRET in production. Address in M18 or separately.

## Review summary

| Reviewer | Verdict | Key findings |
|----------|---------|-------------|
| Acceptance test | needs-revision → addressed | Missing cache scenarios added (4 quadrants); concurrent minting scenario added; doc-only items marked; OPS-1 boundary clarified; invalid timeout scenario added |
| Architecture | approve | M17 fork detection is natural split seam if scope feels large; per-token-fingerprint rate limit reconciled in ADR; Phase 6 index.md needs update when milestones added |
| UX | approve | 403 message now includes SSO/IP fallback; fork PR message says "Tests run normally"; timeout clamp uses env var name; dashboard proxy discoverability via UX-8 |
| Security | needs-revision → addressed | Prefix check now soft gate; cache TTL reduced to 15s; repositories parameter explicit (SEC-6a); SSRF protection added (SEC-6b); per-IP rate limiting removed |
