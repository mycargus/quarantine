# ADR-032: Per-Suite State Isolation — Separate State Files, Notifications, and Issue Dedup

**Status:** Proposed
**Date:** 2026-04-11

## Context

Multi-suite support (see `docs/plans/multi-suite-support.md`) requires quarantine
to manage quarantine state, PR comments, and GitHub issue dedup independently
per test suite. A repository may have multiple suites (backend, frontend, e2e)
running as independent CI steps, possibly in parallel.

Without explicit isolation, several problems arise:

1. **CAS contention:** If all suites share a single `quarantine.json`, parallel
   `quarantine run` invocations would contend on the same file. One suite's
   CAS update blocks another's, turning what should be independent operations
   into a coordination problem.

2. **PR comment overwrite:** A single PR comment updated by each suite in turn
   would cause the last writer to overwrite earlier suites' content, losing
   visibility into the other suites.

3. **Issue dedup collision:** The existing `quarantine:{test_hash}` label format
   uses only the test hash, meaning a test with the same name in two suites
   would share one GitHub Issue even if the suites track separate quarantine
   state.

4. **Rename coupling:** If suite names were embedded in the test ID (e.g.,
   `backend::file::classname::name`), renaming a suite would orphan all
   quarantined tests for that suite — their state file entries would no longer
   match.

ADR-006 established that state is stored on a dedicated `quarantine/state`
branch via the GitHub Contents API. ADR-012 established CAS (SHA-based
compare-and-swap) for concurrency safety. ADR-009 established the PR comment
marker and issue dedup label. This ADR extends all three decisions for the
multi-suite case.

## Decision

### 1. Separate state file per suite

**Each suite has its own state file** at
`.quarantine/<suite-name>/state.json` on the `quarantine/state` branch.
`quarantine run backend` reads and writes only its own state file.
`quarantine run frontend` reads and writes only its own state file.

CAS (compare-and-swap via GitHub Contents API SHA, per ADR-012) operates per
file. Parallel suite runs never contend for the same file.

**State file lifecycle:** The file is created on the first `quarantine run
<suite>` for that suite, not during `quarantine init`. If the state file does
not exist, quarantine creates it with an empty test map. This defers file
creation until a suite actually runs.

**Test ID uniqueness:** Test IDs (`file_path::classname::name`, per ADR-020)
are not assumed to be globally unique across suites. Scoping state files by
suite name eliminates this requirement. Two suites may track the same test ID
independently.

**State branch bootstrap:** `quarantine init` creates the `quarantine/state`
branch with an initial commit containing a `README.md`. This gives the Contents
API a commit to reference — an empty branch with no commits cannot have files
written to it.

### 2. One PR comment per suite

**Each `quarantine run <suite>` posts or updates its own PR comment**, identified
by the HTML marker `<!-- quarantine:<suite-name> -->`. Comments from different
suites coexist on the same PR without interference.

Comment identification uses the suite name directly (e.g.,
`<!-- quarantine:backend -->`), making the comment's source unambiguous to
both quarantine and to humans reading the PR.

### 3. Suite-scoped issue dedup label

**The issue dedup label format is `quarantine:<suite-name>:<test_hash>`**,
where `test_hash` is the first 8 hex characters of `SHA-256(test_id)`.

This replaces the prior format `quarantine:{test_hash}` (which was not suite-
scoped). A test that is flaky in two different suites creates separate GitHub
Issues with separate labels — one per suite.

**Label length constraint:** GitHub labels have a 50-character maximum.
The format uses: `quarantine:` (11 chars) + suite name + `:` (1 char) +
hash (8 chars) = 20 + suite name length. Suite names are capped at 30
characters (ADR-010), keeping the total under 50.

**Known edge case — overlapping suites:** If the same test file appears in
two suites (overlapping `command` configurations), the same flaky test creates
separate Issues for each suite. This is intentional: each suite independently
tracks and manages its own quarantine state. Users who configure overlapping
suites accept this behavior.

### 4. Dashboard state enumeration

The dashboard reads the state branch by:
1. Listing the `.quarantine/` directory on the state branch (1 API call).
2. Reading each suite's `state.json` (N API calls, one per suite).

If `.quarantine/` does not exist yet (no suite has ever run), the dashboard
handles the 404 gracefully — no state to display. For a repo with 5 suites
on a 5-minute poll, this is approximately 72 calls/hr — well within
`GITHUB_TOKEN` rate limits (1,000/hr) at v1 scale.

A v2 optimization (`quarantine state consolidate`) writes a single
consolidated `state.json` from all per-suite files, reducing dashboard API
calls from 1 + N to 1 per poll. All webhooks remain deferred to v3 per
ADR-027.

## Alternatives Considered

- **Single `quarantine.json` with per-suite sections (e.g., `tests.backend`, `tests.frontend`).** Keeps all state in one file. Rejected because CAS operates on the entire file: a parallel `quarantine run frontend` update would conflict with `quarantine run backend` even though they write to different sections. Resolving conflicts requires merging JSON objects, which adds complexity without reducing risk.

- **Suite name prefix in test ID globally (e.g., `backend::file::classname::name`).** Makes test IDs globally unique without per-suite files. Rejected because it couples the quarantine entry's identity to the suite name in config — renaming a suite from "backend" to "api" would orphan all its quarantined tests. Per-suite files scope the namespace at the file level without embedding the suite name in the test ID.

- **Combined PR comment via a post-workflow aggregation step.** A separate job runs after all suites complete and produces one combined PR comment. More readable for humans. Rejected for v1 because it requires coordination between independent CI steps (either shared state or a separate aggregation job), and introduces a failure mode where the aggregation job fails but the suites succeeded. Deferred to v2.

- **Global issue dedup without suite scope.** Keep `quarantine:{test_hash}` and share one GitHub Issue across suites for the same test. Rejected because separate suites may quarantine the same test independently for different reasons (e.g., flaky in integration but stable in unit). Merging their tracking into one issue conflates distinct failure modes and makes the issue's resolution ambiguous (close when fixed in one suite? both?).

## Consequences

**Positive:**

- (+) Parallel suite runs are fully independent at the state level — no CAS contention, no locking, no coordination.
- (+) Suite removal does not affect other suites' state files.
- (+) PR comments from each suite coexist on the same PR, giving full visibility into all suite results.
- (+) Suite-scoped issue dedup keeps the issue tracker accurate — one issue per flaky test per suite.
- (+) Test ID uniqueness is only required within a suite, simplifying the uniqueness contract (ADR-020).
- (+) Dashboard enumeration scales linearly with suite count (1 + N API calls), which is acceptable at v1 scale.

**Negative:**

- (-) Multiple state files means the dashboard makes 1 + N API calls per poll instead of 1. Mitigated by v2 state consolidation.
- (-) A PR with multiple suites has multiple PR comments rather than one combined view. Mitigated by per-comment suite name identification; deferred to v2 for aggregation.
- (-) A test that is flaky in two suites creates two GitHub Issues. This is intentional but may be surprising. The behavior is consistent with the principle that each suite manages its own state.
- (-) The `quarantine/state` branch bootstrap requires an initial commit with a `README.md` (a `quarantine/state` branch with no commits cannot have files created on it via the Contents API). This is an implementation constraint, not a user-visible behavior.
