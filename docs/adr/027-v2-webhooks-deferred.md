# ADR-027: Webhooks Deferred to v3

**Status:** Accepted
**Date:** 2026-04-05

## Context

The initial GitHub App plan included webhook processing for v2: real-time unquarantine on `issues.closed`, instant artifact ingestion on `workflow_run.completed`, and installation lifecycle events. This would require:

- A publicly accessible webhook endpoint on the dashboard (v1 dashboard is internal-only behind a reverse proxy).
- Webhook secret management (`QUARANTINE_WEBHOOK_SECRET`).
- HMAC-SHA256 signature verification with constant-time comparison.
- Idempotency tracking via `X-GitHub-Delivery` GUID in SQLite.
- A SQLite-backed async job queue (10-second webhook response timeout requires heavy work to be queued).
- Payload size enforcement (25 MB max).

This is substantial infrastructure for features that have polling-based alternatives.

## Decision

Defer all webhook processing to v3. Register the GitHub App with webhooks **disabled** (no webhook URL, no webhook secret). The dashboard uses API polling for both installation discovery (15-minute background loop) and artifact ingestion (existing 5-minute debounced polling).

When v3 considers webhooks, an architectural review should address public endpoint exposure, DDoS considerations, and whether the latency improvement (polling ~5 min to webhook ~30s) justifies the complexity. That review should produce its own ADR.

See `docs/plans/webhooks.md` for the deferred work scope.

## Alternatives Considered

- **Include webhooks in v2:** Requires solving public endpoint exposure (the v1 dashboard is internal-only), adding a webhook secret to CI configuration, and building a job queue. High complexity for marginal latency improvement.
- **Separate webhook receiver service:** Reduces dashboard complexity but adds deployment complexity (two services instead of one). Considered overkill for v2.
- **Webhooks for installation events only (partial):** Still requires public endpoint and signature verification. The installation discovery polling loop is simple and sufficient.

## Consequences

- (+) Simpler v2: no webhook endpoint, no webhook secret, no job queue, no signature verification, no idempotency tracking.
- (+) Dashboard can remain internal-only for v2 if desired.
- (+) Fewer secrets to manage (no `QUARANTINE_WEBHOOK_SECRET`).
- (+) No public endpoint attack surface in v2.
- (+) App registration is simpler (no webhook URL configuration).
- (-) Artifact ingestion delayed by polling interval (~5 min) instead of near-real-time (~30s).
- (-) Unquarantine timing unchanged: still happens on next CLI run, not on issue close.
- (-) v3 will need architectural review for public endpoint exposure.

## Amendment (2026-04-10): Reverted

~~Allow `workflow_run.completed` webhook processing in v2.~~ Reverted after risk
review. The infrastructure required for even one webhook type (public endpoint,
webhook secret, HMAC verification, idempotency tracking) is the same as for all
webhook types. The state consolidation optimization is achievable via a scheduled
GitHub Actions workflow + a `quarantine state consolidate` CLI command, which
requires zero webhook infrastructure. See ADR-032.
All webhooks remain deferred to v3.
