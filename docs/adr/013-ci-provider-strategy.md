# ADR-013: GitHub Actions as v1 CI Provider

**Status:** Accepted
**Date:** 2026-03-14

## Context

Quarantine must work in any CI provider (Jenkins, GitHub Actions, GitLab CI, Bitbucket Pipelines, etc.). However, some features (GitHub Artifacts, Actions cache) are GitHub Actions-specific. Need to decide scope for v1 vs later versions.

## Decision

v1 targets GitHub Actions as the fully-featured CI provider. The CLI binary itself runs anywhere (it is just a Go binary), but GitHub Actions-specific features (artifact upload, Actions cache fallback for branch-protected repos) are only available in GitHub Actions for v1.

In non-Actions environments, the CLI still works for core functionality (run tests, detect flakiness, update quarantine.json via GitHub API, create issues, post PR comments) but cannot upload artifacts or use Actions cache.

v2 adds: Jenkins artifact storage integration, GitLab CI integration, Bitbucket Pipelines integration. The CLI will auto-detect the CI environment and use the appropriate storage backend.

## Alternatives Considered

- **CI-agnostic from v1:** Would require abstracting artifact/cache storage immediately, increasing scope. Rejected to keep v1 focused.
- **GitHub Actions only (no other CI ever):** Limits market. Rejected as a long-term strategy.
- **Jenkins first (user's employer uses both):** GitHub Actions has better API integration for the pull model. Jenkins support added in v2.

## Consequences

**Positive:**
- v1 scope is focused and achievable.
- Full feature set in GitHub Actions (artifacts, cache, optimistic concurrency).
- Core quarantine functionality still works in any CI.

**Negative:**
- Jenkins users at employer will not get artifact-based dashboard sync until v2. Mitigated by: Jenkins builds can still use core CLI features; only dashboard analytics is deferred.
- CI provider abstraction must be designed eventually.
