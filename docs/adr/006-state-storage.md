# ADR-006: Quarantine State Storage on GitHub Branch

**Status:** Accepted
**Date:** 2026-03-14

## Context

quarantine.json holds the list of quarantined tests. It must be readable at the start of every CI run and writable when flaky tests are detected. Must handle concurrent CI builds. Must work with or without branch protection rules.

## Decision

Store quarantine.json on a dedicated `quarantine/state` Git branch (never merged). Read/write via GitHub Contents API with optimistic concurrency (SHA-based compare-and-swap). On 409 Conflict, re-read, merge changes, retry up to 3 times.

For branch-protected repos, two fallback plans:

- **Plan B1:** GitHub App with branch protection bypass permission (the proper mechanism, used by Dependabot/Renovate).
- **Plan B2:** GitHub Actions cache with versioned keys (`quarantine-state-v{timestamp}`). Eventually consistent -- concurrent writes create separate versions, next run reads latest and implicitly merges.

The CLI config allows choosing the backend: `storage.backend: branch` (default) or `storage.backend: actions-cache`.

## Alternatives Considered

- **GitHub Actions cache as primary:** Works but 7-day eviction if not accessed, no audit trail. The branch approach gives git history as a free audit log.
- **Database on server:** Adds infrastructure dependency to CI-critical path.
- **File in main branch:** Pollutes main with automated commits, merge conflicts with developer work.
- **GitHub repository variables:** Too small, no structured data support.

## quarantine.json Lifecycle

- The file stores only ACTIVE quarantine state (tests currently quarantined).
- When a test is unquarantined (issue closed), its entry is removed from quarantine.json.
- Historical quarantine data is preserved in the dashboard's SQLite database (ingested from GitHub Artifacts).
- This keeps quarantine.json small and well under the GitHub Contents API 1 MB file limit (~2,500 entries max, but pruning keeps it far below this).
- The 1 MB limit is an explicit constraint of the Contents API; exceeding it would require switching to the Git Blobs API.

## Consequences

- (+) Zero infrastructure -- GitHub IS the storage.
- (+) Git history on the branch provides free audit log of quarantine events.
- (+) Optimistic concurrency handles concurrent builds safely.
- (+) Works with any GitHub token that has contents:write.
- (-) GitHub API latency on every CI run (~100-500ms).
- (-) Branch protection requires additional configuration (App bypass or cache fallback).
- (-) Actions cache fallback loses the audit trail benefit.
