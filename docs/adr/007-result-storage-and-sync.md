# ADR-007: GitHub Artifacts for Result Storage and Pull-Based Dashboard Sync

**Status:** Accepted
**Date:** 2026-03-14

## Context

Test run results must be stored durably so the dashboard can ingest them for analytics. The dashboard may be unreachable during CI runs. Must support eventual consistency with zero data loss.

## Decision

Two-part decision:

1. **Result storage:** CLI uploads test run results as GitHub Artifacts (one artifact per CI run, named `quarantine-result-{run-id}.json`). 90-day retention, immutable, API-accessible.
2. **Data sync:** Pull model. The dashboard pulls results from GitHub Artifacts API. Hybrid polling: background sync every 5 minutes per org (staggered across repos) + on-demand pull when user views a project page (debounced: max 1 pull per repo per 5 min). Conditional requests (ETags/If-Modified-Since) to minimize API usage.

The CLI never needs to know the dashboard exists or its URL. The dashboard discovers results autonomously.

## Alternatives Considered

- **Push model (CLI POSTs to dashboard):** CLI must know dashboard URL and have API key, config required per repo, if dashboard moves all repos must update. Rejected for higher friction.
- **GitHub Actions cache for results:** 7-day TTL too short for historical analysis, not accessible from outside Actions. Rejected.
- **Database writes from CLI:** Adds infrastructure dependency to CI-critical path. Rejected.

## Consequences

- (+) Zero data loss -- results are durable in GitHub Artifacts regardless of dashboard availability.
- (+) CLI is completely decoupled from dashboard.
- (+) Adding a new repo to monitoring requires zero dashboard configuration -- dashboard auto-discovers results.
- (+) 90-day retention covers most analysis needs.
- (-) Dashboard must poll GitHub API, consuming rate limit budget. Mitigated by staggered polling, conditional requests, and adaptive frequency for inactive repos.
- (-) Latency between CI run and dashboard visibility is up to 5 minutes. Acceptable for analytics use case.
- (-) For Jenkins (v2), GitHub Artifacts are not available -- Jenkins has its own artifact storage, requiring a provider abstraction in the CLI.
