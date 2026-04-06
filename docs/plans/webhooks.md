# Plan: Webhook Processing

> **Status:** Deferred to v3. Pending architectural review of public endpoint exposure.
>
> **Decision record:** ADR-027

## Context

v2 uses API polling for both installation discovery (15-minute background loop) and artifact ingestion (5-minute debounced polling). Webhooks would reduce artifact ingestion latency from ~5 minutes to ~30 seconds and enable real-time unquarantine on issue close.

## Prerequisites

Before implementation, an ADR is needed for **public endpoint exposure**. The v1 dashboard is internal-only (behind a reverse proxy). Webhooks require a publicly accessible URL. Options to evaluate:

1. Move the entire dashboard to public deployment with OAuth protecting all non-webhook routes.
2. Deploy a separate lightweight webhook receiver that writes to a shared SQLite database.
3. Use a webhook proxy service (e.g., smee.io for dev, but not suitable for production).

## Scope

**App registration changes:**
- Enable webhooks on the existing App registration.
- Configure webhook URL pointing to the dashboard (or separate receiver).
- Generate and store a webhook secret (`QUARANTINE_WEBHOOK_SECRET`).
- Subscribe to events: `installation`, `issues`, `workflow_run`.

**Webhook endpoint (`/api/webhooks/github`):**
- HMAC-SHA256 signature verification via `X-Hub-Signature-256` with constant-time comparison (`crypto.timingSafeEqual`).
- Invalid/missing signatures return 401 immediately.
- Respond 200 within 10 seconds (GitHub's timeout).
- Reject payloads exceeding 25 MB.

**Idempotency:**
- Track `X-GitHub-Delivery` GUID in SQLite. Duplicate deliveries return 200 without reprocessing.

**Async job queue:**
- Heavy work (artifact download, DB writes) queued via SQLite-backed job queue.
- No external message broker (per ADR-011: GitHub IS the backend).
- Job schema, retry policy, worker model, dead-letter handling to be designed.

**Event handlers:**
- `installation` + `created`: record installation, begin polling repos.
- `installation` + `deleted`: stop polling, remove installation record. Preserve historical data.
- `installation` + `suspended`/`unsuspended`: pause/resume polling.
- `issues` + `closed`: if issue has quarantine labels, record unquarantine event for dashboard display. Note: actual `quarantine.json` update still happens on next CLI run.
- `workflow_run` + `completed`: check for quarantine artifacts, download and ingest immediately. Supplements polling.

**Observability:**
- Log webhook events: event type, action, delivery GUID, processing outcome.
- Log failed signatures with requesting IP and timestamp (without signature values).

## Test Plan

**Unit tests:**
- `verifySignature(payload, signatureHeader, secret)` returns true for valid HMAC-SHA256
- Returns false for: tampered payload, wrong secret, empty header, missing `sha256=` prefix
- Use GitHub's published test vector
- Event routing: `routeWebhookEvent(eventType, action, payload)` returns typed action enum

**Integration tests:**
- POST with valid signature -> 200, installation recorded
- Invalid signature -> 401, no DB changes
- Duplicate `X-GitHub-Delivery` -> 200, no duplicate records

**Contract tests:**
- Validate webhook fixture payloads against vendored webhook event schemas
- Uses `ajv` against `schemas/github-webhook-events.json`

**E2E tests (stretch):**
- Real webhook delivery requires smee.io proxy or exposed endpoint. May be local-only.

## Open Questions

1. **Public endpoint architecture:** Which approach for exposing the webhook endpoint? (Needs ADR.)
2. **Smee.io in CI:** Smee is unauthenticated and unsuitable for CI. Webhook E2E tests may need to be local-only.
3. **Job queue design:** Schema, retry policy, polling interval, concurrency model, dead-letter handling.
4. **Webhook retry semantics:** GitHub does not automatically retry failed App webhook deliveries. Manual redelivery available via `GET /app/hook/deliveries` + `POST .../attempts`. Should the dashboard implement periodic reconciliation?
