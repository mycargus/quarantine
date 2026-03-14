# ADR-011: GitHub-Native CLI + Standalone Dashboard Architecture

**Status:** Accepted
**Date:** 2026-03-14

## Context

Need to decide the overall system architecture -- where the SaaS boundary sits and what depends on what. The core question is how tightly coupled the CI-critical path (flaky detection, quarantine management, issue creation, PR comments) should be to any server infrastructure.

Three models were evaluated:

- **Model A: GitHub-native (no server).** CLI stores all state in GitHub (branch, artifacts, issues). No dashboard.
- **Model B: Thin self-hosted server (Go binary + SQLite).** Everything goes through the server.
- **Model C: Hybrid.** GitHub-native CLI (CI-critical path depends only on GitHub) + standalone dashboard server (non-critical, for analytics).

## Decision

Model C. The CI-critical path (flaky detection, quarantine management, issue creation, PR comments) depends only on the CLI and GitHub. The dashboard is a separate component that provides analytics and visibility by pulling data from GitHub Artifacts. The dashboard being down has zero impact on CI.

Key principle: the CLI never needs to know the dashboard exists. The dashboard discovers and ingests data autonomously via GitHub APIs.

## Alternatives Considered

- **Model A (no server):** No dashboard, no analytics, no cross-repo visibility. Dashboard was deemed core to the value proposition for showing CI stability at a glance. Could serve as a minimal v0 but is insufficient for the target customer.
- **Model B (server in critical path):** CLI depends on server availability. Server outage breaks CI. Unacceptable for adoption -- nobody wants a third-party dependency that can break their CI. Also complicates self-hosting requirements.
- **Traditional SaaS (founder hosts everything):** Founder's primary goal is learning/resume, not managing SaaS infrastructure. Self-hosted dashboard shifts ops burden to customer.

## Consequences

**Positive:**
- CI-critical path has zero infrastructure dependencies beyond GitHub.
- Dashboard outage is invisible to CI.
- Clean separation of concerns.
- Dashboard can be deployed later without changing CI integration.
- Self-hosted by customer as a Docker container.

**Negative:**
- Two separate systems to develop and maintain.
- Data flows through GitHub as intermediary, adding latency and API usage.
- Dashboard has an eventually-consistent view of quarantine state (up to 5-min delay).
