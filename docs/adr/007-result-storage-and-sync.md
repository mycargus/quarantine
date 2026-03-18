# ADR-007: Local File Output for Results with Pull-Based Dashboard Sync

**Status:** Amended
**Date:** 2026-03-17 (originally accepted 2026-03-14)

## Context

Test run results must be stored durably so the dashboard can ingest them for analytics. The dashboard may be unreachable during CI runs. Must support eventual consistency with zero data loss.

## Decision

Two-part decision:

1. **Result storage:** The CLI writes test run results to a local JSON file (e.g., `.quarantine/results.json`). The user's GitHub Actions workflow is responsible for uploading this file as a GitHub Artifact using `actions/upload-artifact`. The CLI performs file I/O only -- it does not call the GitHub Artifacts REST API. Artifact naming (including matrix-safe naming for parallel jobs) is handled by the workflow and documented in setup guides.

   This separation keeps the CLI simple and avoids Artifacts REST API complexity. REST API upload is a v2 concern for non-GitHub Actions CI environments.

2. **Data sync:** Pull model. The dashboard pulls results from GitHub Artifacts API. Hybrid polling: background sync every 5 minutes per org (staggered across repos) + on-demand pull when user views a project page (debounced: max 1 pull per repo per 5 min). Conditional requests (ETags/If-Modified-Since) to minimize API usage.

The CLI never needs to know the dashboard exists or its URL. The dashboard discovers results autonomously.

### v2 Multi-CI Poller Viability

Research confirmed that per-provider dashboard pollers are a viable v2 strategy for supporting non-GitHub CI environments. All major CI platforms have artifact APIs that can support an `ArtifactPoller` interface pattern:

- **GitLab CI (P1):** Best API -- clean artifact listing and download via Jobs API. Direct binary download, no redirect chains.
- **Jenkins (P2):** Viable with caveats -- artifact listing per build via REST API. Authentication complexity varies by installation. No built-in pagination for large histories.
- **Buildkite (P3):** Clean REST API with artifact listing and download per build. Requires API token with `read_artifacts` scope.
- **CircleCI (P4):** Verbose but functional -- artifact listing per job via v2 API. Requires project-level API token.

The dashboard can add provider-specific pollers in v2 without changing the CLI or the result JSON schema.

## Alternatives Considered

- **CLI uploads artifacts directly (original decision):** CLI calls GitHub Artifacts REST API. Adds API complexity to the CLI, requires understanding of artifact upload protocols, and doesn't generalize to non-GHA environments. Replaced by local file output.
- **Push model (CLI POSTs to dashboard):** CLI must know dashboard URL and have API key, config required per repo, if dashboard moves all repos must update. Rejected for higher friction.
- **GitHub Actions cache for results:** 7-day TTL too short for historical analysis, not accessible from outside Actions. Rejected.
- **Database writes from CLI:** Adds infrastructure dependency to CI-critical path. Rejected.

## Consequences

- (+) CLI is maximally simple -- file I/O only, no Artifacts API calls.
- (+) Zero data loss -- results are durable in GitHub Artifacts regardless of dashboard availability.
- (+) CLI is completely decoupled from dashboard.
- (+) Adding a new repo to monitoring requires zero dashboard configuration -- dashboard auto-discovers results.
- (+) 90-day artifact retention covers most analysis needs.
- (+) Separation of concerns: CLI writes files, workflow handles upload, dashboard handles download.
- (-) Requires users to add an `actions/upload-artifact` step to their workflow. Mitigated by providing copy-paste workflow snippets in documentation.
- (-) Dashboard must poll GitHub API, consuming rate limit budget. Mitigated by staggered polling, conditional requests, and adaptive frequency for inactive repos.
- (-) Latency between CI run and dashboard visibility is up to 5 minutes. Acceptable for analytics use case.
- (-) For non-GHA CI environments (v2), users would need a different artifact upload mechanism, or the CLI would need direct API upload support.
