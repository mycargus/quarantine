# ADR-038: Allow `quarantine run` to Create State Branch on First Invocation

**Status:** Proposed
**Date:** 2026-04-29
**Amends:** [ADR-019](019-required-initialization.md)

## Context

ADR-019 decided that `quarantine run` must require prior initialization — the
`quarantine/state` branch must already exist — and that implicit branch creation
in `run` was rejected because:

1. Permission errors would surface during a real CI run (not at setup time).
2. Error messages would be ambiguous: is the failure from the test suite or
   from quarantine infrastructure?

M20 (ADR-037) introduced a two-phase `quarantine init` flow. After phase 1, the
user has a valid `.quarantine/config.yml` with explicit `github.owner` /
`github.repo`. The only remaining step before CI is token validation and branch
creation (init phase 2). However, in Jenkins / non-GitHub-Actions environments
the user may prefer:

1. Run `quarantine init` phase 1 locally (write config, commit it).
2. Set `QUARANTINE_GITHUB_TOKEN` as a CI credential.
3. Let the first `quarantine run` in CI create the state branch and proceed.

This eliminates the need for a human with repo write access (or a CI-side init
invocation) before the first run. The original ADR-019 concerns are addressed
differently in M20's context:

**Concern 1 — permission errors surface in CI.** Now handled as degraded mode.
If branch creation fails, `run` emits a `[quarantine] WARNING:` and continues
without quarantine awareness. The build is never broken by branch creation failure.

**Concern 2 — ambiguous error messages.** With M20's config-sourced `owner/repo`,
a `[quarantine] WARNING: Cannot create state branch 'quarantine/state'` message
is unambiguously a quarantine infrastructure issue, not a test failure. It is
actionable (check token scope or network access).

Additionally, M20's `quarantine doctor` is now the canonical pre-flight check
for users who want to validate setup before the first CI run — filling the role
ADR-019 assigned to `init`.

## Decision

`quarantine run` MUST create the `quarantine/state` branch if it does not exist,
rather than exiting with "Quarantine is not initialized." Branch creation follows
the same idempotent pattern as `init` phase 2:

1. `GET /repos/{owner}/{repo}/git/ref/heads/{default_branch}` — fetch HEAD SHA.
2. `POST /repos/{owner}/{repo}/git/refs` — create the state branch.
3. If the response is 422 (branch already exists — concurrent creation race),
   treat as "branch exists, continue." No error.

If branch creation fails for any other reason (403, 5xx, network error), `run`
falls into degraded mode: emit `[quarantine] WARNING:` and continue without
quarantine state. The build is never broken.

`quarantine init` phase 2 retains its branch-creation behavior. It remains a
valid, explicit setup step that also validates the token and produces a
"setup complete" summary. Both `init` and `run` create the branch idempotently;
concurrent creation is safe.

## Alternatives Considered

- **Keep ADR-019 as-is (require init phase 2).** Adds friction in Jenkins
  environments where the user must run `quarantine init` with a writable token
  before the first CI build. Rejected: M20 already gives users a natural
  hand-edit moment (phase 1); requiring an additional explicit step reduces
  adoption without a safety benefit (degraded mode already handles creation
  failures gracefully).

- **`quarantine doctor` creates the branch.** Doctor is a read-only verification
  tool per ADR-037. Adding write operations is out of scope.

## Consequences

- (+) Zero-friction path: write config (phase 1), commit, set token in CI,
  first `quarantine run` bootstraps everything else.
- (+) Jenkins / non-GitHub-Actions users do not need `quarantine init` phase 2.
- (+) Concurrent first-runs (multiple CI shards starting simultaneously before
  the branch exists) are handled gracefully via 422 → continue.
- (-) Branch creation may fail during a real CI run. Mitigated: degraded mode
  keeps builds green; the warning message is actionable.
- (-) The state branch will not have the README.md that `init` phase 2 writes.
  Cosmetic — README is a convenience, not a functional requirement.
- (-) `quarantine init` phase 2 becomes optional for branch creation, reducing
  the perceived need for it. Users who skip it will not see the "setup complete"
  summary and will not get the README.md.
