# ADR-037: Decouple Git Origin From GitHub Target; Config Is Source of Truth

**Status:** Proposed
**Date:** 2026-04-29
**Amends:** [ADR-013](013-ci-provider-strategy.md), [ADR-019](019-required-initialization.md)

## Context

ADR-013 stated that "the CLI binary itself runs anywhere … the CLI's core
functionality is CI-agnostic." In practice this is not true today: the CLI
hard-rejects any git remote whose host is not `github.com`
(`cli/internal/git/remote.go:36`). `quarantine init` aborts before writing any
config if the working tree's `origin` remote points to a non-GitHub host
(`cli/cmd/quarantine/init.go:63`), and `quarantine run` inherits the same
constraint through `resolveOwnerRepo` (`cli/cmd/quarantine/run.go:653-659`).

This blocks the following real use case: a project hosted in Gerrit, tested in
Jenkins, with a GitHub mirror that carries no GitHub Actions workflows. The
project wants to use quarantine now (Gerrit+Jenkins) and continue using it
through an 8-month migration onto GitHub while Jenkins remains the CI runner.

The underlying issue is a conflation of two independent concerns:

1. **Where the working tree came from** (the git remote's host — could be
   Gerrit, GitLab, Bitbucket, or GitHub).
2. **Where quarantine writes state and creates issues** (always a GitHub
   repository).

These are the same host only when a project is hosted on GitHub. Quarantine
does not need them to be the same host. The Contents API and Issues API calls
that carry out quarantine's work target a specific GitHub `owner/repo`; they
do not use the working tree's remote URL at all.

This ADR removes the accidental coupling. It also removes git-remote scanning
from the CLI's owner/repo **resolution** path: the configuration file is the
sole source of truth for the GitHub target. Any state where owner/repo cannot
be resolved is a hard error (exit 2), not a partial-success path — partial
state breaks quarantine's contract that "every run produces a deterministic
outcome."

A narrow exception is preserved for `init` phase 1: it may inspect git
remotes to surface candidate GitHub targets as **non-authoritative YAML
comments** in the partial config (a bootstrapping hint). Those comments
have no resolution semantics — the user must copy a value into
`github.owner` / `github.repo` for it to take effect — and `run` / `doctor`
never read remotes at all. This preserves the "config is sole source" rule
for runtime behavior while keeping a useful ergonomic hint at the moment of
first setup.

**Scope:** v1 only (CLI performs all GitHub writes). v2's server-side write
architecture (ADR-036 amendment) is orthogonal; dashboard ingestion from
Jenkins-uploaded artifacts is a separate decision.

## Decision

**The working tree's git origin is no longer required to be a github.com URL.
The GitHub owner and repo are read exclusively from
`.quarantine/config.yml` for runtime resolution. Git-remote scanning is
removed from `run` and `doctor`. `init` phase 1 retains remote inspection
for the sole purpose of emitting non-authoritative YAML comments listing
detected github.com candidates in the partial config; those comments do not
participate in resolution.**

### Owner/repo resolution

The CLI resolves the GitHub target with this two-step rule:

1. Read `github.owner` and `github.repo` from `.quarantine/config.yml`.
2. If either is missing, exit 2 with a `Error [config]:` diagnostic.

This rule applies identically to `quarantine init` (after the partial-config
write step described below), `quarantine run`, and `quarantine doctor`. There
is no remote-scan fallback. There are no new flags.

### Config schema

`github.owner` and `github.repo` are **required** fields under the shared
`github` block in `.quarantine/config.yml`. Missing or empty values are
schema-invalid and produce a config error (exit 2).

### `quarantine init` — two-phase setup

**Phase 1 — first run (no existing config; remote-derived hints, no
auto-resolution):**

Init creates `.quarantine/config.yml` populated with everything it can
determine (framework detection, suite stubs, defaults) but with the `github`
block left empty. If `git remote -v` succeeds and any remote points at a
github.com URL, init appends those candidates as YAML comments under the
`github` block — purely as a hint. The fields themselves remain empty:

```yaml
github:
  owner: # set to your GitHub organization or user
  repo:  # set to your GitHub repository name
  # detected GitHub remotes (review before using):
  #   origin   -> mhargiss/quarantine
  #   upstream -> mycargus/quarantine
```

If no remotes are github.com URLs, or `git remote -v` fails for any reason,
init writes the empty `github` block with no hint comments and proceeds.
Remote inspection is best-effort and never blocks phase 1.

The hint comments are advisory. Phase 2 reads only `github.owner` and
`github.repo`; comments are ignored. This is intentional — surfacing the
fork-vs-upstream choice (when both remotes exist) forces the user to pick
rather than silently inheriting whichever remote happens to be `origin`.

Init does NOT attempt to create the `quarantine/state` branch in this phase
(owner/repo unknown). It exits 2 with:

```
Error [config]: github.owner and github.repo are required.
.quarantine/config.yml has been created. Edit it to set:

  github:
    owner: <your-github-org-or-user>
    repo:  <your-github-repo-name>

Then re-run 'quarantine init' to complete setup.
```

**Phase 2 — re-run after hand-edit (config has owner/repo):**

On re-run, init reads the existing config, finds valid `github.owner` and
`github.repo`, validates the GitHub token, creates the `quarantine/state`
branch (idempotent per NFR-2.2.4 — skips if already present), and exits 0.

This two-phase flow makes init **never produce an inconsistent partial state
that exits 0**. Either setup is complete (exit 0) or it is incomplete and the
user knows exactly what to do (exit 2 with hand-edit instructions).

### `quarantine doctor` — verification

Doctor's responsibility is to validate the quarantine setup: config is
well-formed and the configured GitHub target is reachable with the configured
token. Doctor is not a token diagnostic — it does not introspect token scopes
or predict the outcome of write operations. Token scope problems surface at
`run` time as a specific `403` from GitHub on the actual failing endpoint,
which is more actionable than an inferred warning.

Doctor's M20 changes are minimal:

1. Read `github.owner` and `github.repo` from `.quarantine/config.yml`. If
   either is missing or empty, exit 2 with `Error [config]:` and make no
   GitHub API call.
2. If both are present, perform the existing reachability check:
   `GET /repos/{owner}/{repo}`. A 200 response is a pass. Any 4xx response
   is reported with GitHub's own error message and exits 2.
3. Do NOT inspect the working tree's git origin URL.
4. Do NOT introspect `response.permissions` or `response.has_issues` — those
   would be token-scope diagnostics, outside doctor's responsibility.

### Auth in non-GitHub-Actions environments

Outside GitHub Actions, `GITHUB_TOKEN` is not auto-provisioned. Jenkins and
other CI runners must supply `QUARANTINE_GITHUB_TOKEN` (a PAT) explicitly.
This is consistent with ADR-008's v1 auth decision and requires no code
change; it is a documentation and onboarding concern.

### PR comments in non-GitHub-Actions environments

PR comment posting requires a GitHub PR number. In GitHub Actions this is read
from `GITHUB_EVENT_PATH`. Outside GitHub Actions, the existing `--pr N` flag
on `quarantine run` provides this. No new env vars or flags are added.

When no PR number is available (neither `GITHUB_EVENT_PATH` nor `--pr N`),
`quarantine run` skips PR comment posting silently. This is the existing
behavior; no change is needed.

### Amended scope boundary (replaces ADR-013 v1 CI boundary)

> **Full features** (PR comments on GitHub PRs, GitHub Actions run-link in
> Issue body, dashboard artifact ingestion) **require GitHub Actions.**
>
> **Core CLI features** (quarantine state branch CAS, Issue create/dedup,
> JUnit XML parsing, retry logic, results.json output) **work on any CI
> runner** that has `QUARANTINE_GITHUB_TOKEN` set and network access to
> GitHub, regardless of the working tree's git origin host.

### Documentation

A `docs/guides/jenkins-integration.md` guide is added as part of this work.
It covers: token setup (`QUARANTINE_GITHUB_TOKEN` as a Jenkins credential),
Jenkinsfile pipeline snippet, the two-phase init flow for projects with
non-GitHub origins, PR comment usage (`--pr N`), known limitations (no
dashboard ingestion, no auto-provisioned token), and a note on SHA provenance
when the working tree's commits differ from the GitHub mirror.

## Alternatives Considered

- **Keep auto-discovery via git-remote scanning (authoritative).**
  Earlier iterations of this ADR proposed scanning all `git remote -v` entries
  for github.com URLs and writing the result directly into `github.owner` /
  `github.repo`. Rejected: the working tree's remotes do not reliably reflect
  where quarantine should write. A Gerrit-hosted project may have a github.com
  remote that is a personal fork, not the team's mirror. A `github.com` origin
  may be a stale URL after a repo rename. Treating remotes as authoritative
  invites silent misconfiguration. Config is the only source of truth that
  the user actively chooses. The decision retains a narrow non-authoritative
  use of remote scanning in `init` phase 1 (hint comments only); see the
  Decision section.

- **Add `--owner` / `--repo` flags to `quarantine init`.**
  Direct flags would let init bootstrap without hand-editing. Rejected: hand-
  editing the config is one extra step but produces a clear, version-controlled
  artifact the user can review before committing. CLI flags scattered across
  shell history are easier to lose track of. Init is a one-time local
  command, not a scripted operation.

- **Allow init to write a partial config and exit 0 with a warning.**
  Earlier iterations proposed this. Rejected: a partial state that exits 0
  contradicts quarantine's invariant that "exit code reflects the run outcome."
  CI scripts that check `$?` would treat the partial state as success and
  proceed; the user's first hint of trouble would be a confusing failure
  during a real test run. Exit 2 on partial state is the only safe default.

## Consequences

**Positive:**

- (+) Unblocks Gerrit+Jenkins users for the entire migration arc (Gerrit+
  Jenkins → GitHub+Jenkins → GitHub+GitHub Actions).
- (+) Config becomes the single source of truth for the GitHub target.
  Reasoning about where state and issues will land requires reading exactly
  one file.
- (+) Init's exit code is unambiguous: 0 means "fully set up," 2 means
  "manual step required, here's what to do." No silent partial states.
- (+) `quarantine doctor` becomes the authoritative pre-flight check
  regardless of CI environment — one command to verify setup.
- (+) Removes implementation complexity: no remote-scanning code, no
  ambiguity-disambiguation code, no fetch-vs-push URL deduplication.
- (+) No new flags, no new env vars — the surface area of the CLI does not
  grow.

**Negative:**

- (-) Even GitHub-native projects (origin is github.com) must complete two
  phases of `init` to bootstrap. The first phase writes a partial config and
  exits 2; the user adds owner/repo by hand; the second phase finishes setup.
  This is more friction than the previous auto-detect-from-origin behavior,
  but the previous behavior is being removed because it is unreliable in
  mirror/fork contexts.
- (-) Doctor verifies reachability only, not token scopes. A token with read
  access but missing write scope passes doctor and fails at run time on the
  first write call (e.g., 403 on `PATCH /repos/.../contents/...`). The runtime
  error names the exact failing endpoint, which is more actionable than an
  inferred warning, but the failure surfaces later than it would with a
  permission-introspecting doctor.
- (-) `QUARANTINE_GITHUB_TOKEN` (PAT) is required in Jenkins; no
  auto-provisioned token. PATs are long-lived secrets that must be rotated
  manually (pre-existing v1 limitation from ADR-008, not introduced by this
  ADR).
- (-) PR comments and dashboard artifact ingestion are unavailable in Jenkins
  (for now). Users see quarantine state and issues on GitHub but no PR
  annotations until the project migrates to GitHub Actions.
- (-) SHA recorded in quarantine artifacts may refer to a Gerrit commit SHA
  that does not exist on the GitHub mirror (Gerrit rebases change SHAs).
  Quarantine records whatever `git rev-parse HEAD` returns; this is acceptable
  for flakiness tracking and is documented in the Jenkins guide.
