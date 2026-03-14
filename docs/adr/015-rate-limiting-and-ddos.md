# ADR-015: Rate Limiting and DDoS Mitigation Strategy

**Status:** Accepted
**Date:** 2026-03-14

## Context

The dashboard is a web application that will be exposed to network traffic. Even when internal-only (v1), rate limiting prevents accidental abuse. For v2 (public-facing with OAuth), DDoS protection becomes more important. Additionally, the dashboard polls GitHub APIs and must avoid self-imposed denial of service from excessive API calls.

## Decision

Layered rate limiting strategy across two phases:

### v1 (internal-only)

- Dashboard behind reverse proxy (Caddy or nginx) with: 60 req/min per IP, 1MB request size limit, connection limit per IP.
- Artifact polling: hardcoded 5-min interval, not triggerable by external requests.
- GitHub API: exponential backoff on 429 responses, circuit breaker (3 consecutive failures triggers a 30-min pause).

### v2 (public-facing with OAuth)

- **Unauthenticated endpoints** (login page, auth callback, health check only): 20 req/min per IP. All other routes require authentication and return 401.
- **Authenticated endpoints:** 300 req/min per user.
- **On-demand artifact pull debounced:** max 1 pull per repo per 5 minutes regardless of user refreshes.
- **Self-DDoS protection for GitHub API:**
  - Stagger polling across repos (do not poll all repos simultaneously).
  - Conditional requests (ETags) -- 304 responses do not count against rate limit.
  - Adaptive polling: repos with no CI activity in 24h polled every 30 min instead of 5 min.
  - Budget monitoring: track API calls used vs. rate limit budget, throttle if approaching 80%.
    - Note: `GITHUB_TOKEN` in GitHub Actions is limited to 1,000 req/hr/repository. PATs get 5,000/hr. GitHub App installation tokens get 5,000/hr minimum, scaling to 12,500 based on repo count. v1 using `GITHUB_TOKEN` has a 1,000/hr budget; recommend PAT or GitHub App for higher limits.

## Alternatives Considered

- **No rate limiting (trust internal network):** Risky even internally -- a misconfigured script or monitoring tool could overload the dashboard. Rejected.
- **Cloud-based DDoS protection (Cloudflare, AWS WAF):** Overkill for v1 internal deployment. Consider for v2 if dashboard goes public beyond org network.
- **Rate limiting in application code only:** Works but reverse proxy handles it more efficiently and before requests hit the application. Use both layers.

## Consequences

**Positive:**
- Protects dashboard availability from accidental and intentional abuse.
- Prevents self-DDoS against GitHub API.
- Layered approach (proxy + application) provides defense in depth.
- Adaptive polling reduces API usage for quiet repos.

**Negative:**
- Rate limits may need tuning based on real usage patterns.
- Circuit breaker on GitHub API means dashboard may serve stale data during GitHub outages. Acceptable -- stale analytics is better than no analytics.
