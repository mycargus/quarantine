# ADR-018: Competitive Positioning Strategy

**Status:** Accepted
**Date:** 2026-03-14

## Context

The flaky test management space has established players (Trunk Flaky Tests, Captain by RWX, BuildPulse) and adjacent competitors (Datadog, Buildkite Test Engine). Quarantine needs a clear positioning strategy to differentiate. The project's primary goal is learning and resume value; monetization is secondary.

## Decision

Compete on three axes: simplicity of integration, self-hosting capability, and GitHub-native architecture. Do NOT compete on breadth of framework/CI support or feature count.

Positioning:

1. **Simplest integration:** One CLI prefix (`quarantine run -- <cmd>`), one config file, one env var. Zero code changes, zero test runner plugins, zero framework-specific setup. Trunk and Captain require more configuration steps.
2. **Self-hostable:** The dashboard runs as a customer-owned container (or directly on Node.js). No SaaS dependency. Trunk and Captain are SaaS-only. Enterprise customers wanting data ownership or compliance have no alternative.
3. **GitHub-native:** State on a GitHub branch, results in GitHub Artifacts, issues via GitHub API. No external database, no proprietary storage. The customer's existing GitHub infrastructure IS the backend. This is architecturally unique.

What we deliberately do NOT compete on:

- Framework breadth (Trunk supports 30+ frameworks; we start with 3).
- CI provider breadth (Trunk supports 7+ CI platforms; we start with GitHub Actions).
- AI/ML features (Trunk has AI failure grouping; we have simple retry-based detection).
- Feature count (no test partitioning, no test selection, no performance analytics).

Why this positioning works:

- Simplicity and self-hosting are underserved in the market.
- GitHub Actions has zero native flaky test features -- the largest underserved CI platform.
- The target customer (employer) uses GitHub and values self-hosted solutions.
- Starting narrow and deep (3 frameworks, 1 CI, done well) is better than broad and shallow.
- Framework and CI support can be expanded in v2+ without architectural changes.

## Alternatives Considered

- **Compete on breadth (support many frameworks from day one):** Rejected because it increases v1 scope dramatically without proportional value for the target customer.
- **Compete on AI features (smart detection, auto-fix suggestions):** Rejected as premature -- the project's primary goal is learning, not ML research. Simple retry-based detection is sufficient and understandable.
- **Compete on price (undercut Trunk/Captain):** Rejected as irrelevant since the project is primarily a learning exercise, and the self-hosted model already makes it effectively free for the customer.

## Consequences

- (+) Clear, defensible position in the market.
- (+) v1 scope remains achievable for a solo developer.
- (+) Self-hosting is a genuine gap no competitor fills.
- (+) GitHub-native architecture is technically novel and impressive for a portfolio project.
- (-) Small addressable market in v1 (only GitHub Actions + RSpec/Jest/Vitest users). Acceptable given the primary goal is learning, not market capture.
- (-) Feature comparison with Trunk looks unfavorable on a checklist. Mitigated by competing on a different axis entirely (simplicity, self-hosting).
