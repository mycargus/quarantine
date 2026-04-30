# Plan: v2 CLI Auth — GITHUB_TOKEN Default + Server-Side Writes

> **Status:** approved
> **Created:** 2026-04-20
> **Amends:** Original plan (token proxy) replaced after security review
> **Source:** [ADR-036 amendment](../adr/036-v2-cli-auth-github-token-default.md#amendment-2026-04-20-server-side-writes-replace-token-proxy),
> [v2-auth-token-proxy.md](../specs/v2-auth-token-proxy.md) (SEC-7, SEC-9 only)

## Context

v1 requires users to create a PAT and store it as a CI secret. v2 eliminates
this friction entirely: `GITHUB_TOKEN` (auto-provisioned by GitHub Actions)
is used for read-only operations (state branch). The dashboard processes
artifacts and performs all GitHub writes (state updates, issue creation, PR
comments) using its own App installation tokens. No `permissions:` block,
no secrets, no env vars in the user's CI workflow.

The original plan proposed a dashboard token proxy (`/api/ci-token`) for
rate-limit upgrades. Security and architecture review (2026-04-20) identified
that: (1) the `permissions:` block has an unacceptable all-or-nothing cost,
(2) sending `GITHUB_TOKEN` to a third party is a novel pattern with no
industry precedent, and (3) all CLI writes are post-hoc reporting that can
be performed server-side.

## Requirements

| ID | Summary | Source | Priority |
|----|---------|--------|----------|
| R-1 | GITHUB_TOKEN as default CLI auth (read-only) | ADR-036 amendment | must |
| R-2 | Token resolution: QUARANTINE_GITHUB_TOKEN -> GITHUB_TOKEN -> exit 2 | ADR-036 amendment | must |
| R-3 | v2 mode: CLI skips all write API calls, writes results to disk only | ADR-036 amendment | must |
| R-4 | Dashboard processes artifacts and performs state/issue/comment writes | ADR-036 amendment | must |
| R-5 | Results JSON contains all data needed for dashboard writes, including `cli_mode` field (`"v1"` or `"v2"`) so dashboard knows whether writes were already performed | ADR-036 amendment | must |
| R-6 | Dashboard creates state branch on first artifact processing if absent | ADR-036 amendment | must |
| SEC-3 | App permissions MUST NOT exceed CLI requirements (active for dashboard writes) | v2-auth-token-proxy SEC-3 | must |
| SEC-7 | CLI skips quarantine on fork PRs | v2-auth-token-proxy SEC-7 | must |
| SEC-9 | Event payload parsing hardening | v2-auth-token-proxy SEC-9 | must |
| SEC-10 | Dashboard sanitizes artifact string fields before rendering into GitHub markdown | New | must |
| SEC-11 | Dashboard cross-checks artifact `repo` field against download source repo | New | must |
| ERR-1 | No token -> exit 2 (not degraded mode) | ADR-036 amendment | must |
| UX-1 | 403 on state read -> actionable error message (mode-aware: v1 vs v2) | ADR-036 | must |
| UX-6 | Context-aware missing-token message (GHA vs non-GHA) | v2-auth-token-proxy UX-6 | must |

**Removed (token proxy retired):** R-3/R-4 (proxy endpoint), R-5/R-6/R-7
(proxy config), SEC-1 through SEC-6b, SEC-8, UX-2 through UX-5, UX-7, UX-8,
ERR-2, ERR-3, OPS-1 through OPS-4, E2E-1.

## Milestones

### M17: CLI v2 Read-Only Mode

**Dependencies:** M16 (App E2E verified). M17 has no code dependency on M16
but is sequenced after it for organizational clarity. M17 and M18 MAY be
developed in parallel since they touch disjoint codebases (Go CLI vs
TypeScript dashboard).

**Scope -- included:**
- R-1, R-2: GITHUB_TOKEN default, simplified two-step token resolution
- R-3: v2 write skip -- when `QUARANTINE_GITHUB_TOKEN` is NOT set, skip all
  GitHub write calls (state update, issue creation, PR comment); write results
  to disk only
- R-5: results.json includes `cli_mode: "v2"` field (or `"v1"` when PAT is
  used) so dashboard knows whether writes were already performed
- SEC-7: Fork PR detection -> skip quarantine, exec raw command
- SEC-9: Event payload parsing hardening (malformed/missing GITHUB_EVENT_PATH)
- ERR-1: No token -> exit 2 (not degraded mode)
- UX-1: 403 on state read -> mode-aware actionable error
- UX-6: Context-aware missing-token message (GHA vs non-GHA)
- v2 state branch missing: CLI treats as empty state (no exclusions), runs all
  tests. Does NOT exit 2 — the dashboard creates the state branch on first
  artifact processing.
- Verbose logging: `--verbose` shows which token path was taken
  (e.g., `"Token resolved via GITHUB_TOKEN (v2 mode)"`)
- Spec updates: error-handling.md (exit 2 for no-token), cli-spec.md (v2
  token resolution, v2 write behavior) — *verified by doc review*
- Doc updates: CI integration guide with zero-config workflow snippet —
  must include: (a) artifact naming convention
  `quarantine-results-<suite>-${{ github.run_id }}` (the dashboard's
  ingest filter requires the `quarantine-results-` prefix; mismatch causes
  silent ingestion failure), (b) bot identity change notice (issues/comments
  show `<app-name>[bot]` in v2 vs PAT owner in v1), (c) note that the
  dashboard must be running for quarantine to take effect in v2 mode —
  *verified by doc review*

**Scope -- excluded:**
- Dashboard write processing (M18)
- E2E round-trip testing (M19)
- Token proxy (retired)

**Acceptance criteria:**
1. `GITHUB_TOKEN` is used by default when no other token is configured (R-1)
2. Token resolution follows two-step order: QUARANTINE_GITHUB_TOKEN ->
   GITHUB_TOKEN -> exit 2 (R-2)
3. When QUARANTINE_GITHUB_TOKEN is NOT set, CLI makes zero GitHub write API
   calls -- no state updates, no issue creation, no PR comments (R-3)
4. `.quarantine/results.json` written to disk with full data needed for
   dashboard processing, including `cli_mode` field (R-5)
5. CLI logs artifact upload note in v2 mode:
   `[quarantine] NOTE: Results written to .quarantine/results.json. Upload as
   an artifact for dashboard processing. See: <docs-url>`
   Suppressed when `--quiet` is set. (R-3)
6. Fork PR detected (`pull_request` event, head != base repo) -> quarantine
   skipped, raw test command executed (SEC-7)
7. Fork detection also checks `pull_request_target` event name (SEC-7)
8. Same-repo PR (`pull_request` or `pull_request_target`, head == base repo) ->
   quarantine runs normally (SEC-7)
9. Malformed/missing GITHUB_EVENT_PATH -> fork detection skipped, quarantine
   runs normally (SEC-9)
10. No token available -> exit 2 with context-aware message (ERR-1, UX-6):
    - In GHA: `"No GitHub token available. Ensure 'actions/checkout' runs
      before 'quarantine run'. If the problem persists, set
      QUARANTINE_GITHUB_TOKEN."`
    - Non-GHA: `"No GitHub token found. Set QUARANTINE_GITHUB_TOKEN or
      GITHUB_TOKEN."`
11. 403 on state branch read -> mode-aware actionable error (UX-1):
    - v1 (PAT): `"QUARANTINE_GITHUB_TOKEN lacks read access to the state
      branch (403). Check the token has 'contents: read' scope."`
    - v2 (GITHUB_TOKEN): `"Cannot read quarantine state (403). Check for
      SAML SSO enforcement or IP allowlist restrictions on this repository."`
12. v1 PAT mode: QUARANTINE_GITHUB_TOKEN set -> CLI does all reads AND writes
    (backward compatible, no behavioral change)
13. v2 state branch missing (404) -> CLI proceeds with empty state, runs all
    tests with no exclusions (R-6)
14. `--verbose` shows token resolution path:
    `"Token resolved via GITHUB_TOKEN (v2 mode)"` or
    `"Token resolved via QUARANTINE_GITHUB_TOKEN (v1 mode)"` (R-2)
15. `make cli-build && make cli-test && make cli-lint` pass
16. Spec and doc updates verified by doc review

**Scenario outlines:**

Token resolution:
- Given QUARANTINE_GITHUB_TOKEN set -> v1 mode, CLI reads and writes -> R-1, R-2
- Given GITHUB_TOKEN set (no PAT) -> v2 mode, CLI reads only -> R-2, R-3
- Given no token -> exit 2 with error -> R-2, ERR-1
- Given no token + GITHUB_ACTIONS=true -> GHA-specific error message -> UX-6
- Given no token + not in GHA -> generic error message -> UX-6

v1-to-v2 migration:
- Given user previously used QUARANTINE_GITHUB_TOKEN, now removes it, GITHUB_TOKEN available -> v2 mode, CLI reads only, writes results to disk -> R-2, R-3

v2 write skip:
- Given v2 mode + flaky test detected -> results.json written (with cli_mode: v2), no issue created, no state update, no PR comment -> R-3, R-5
- Given v2 mode + quarantine state read succeeds -> exclusions applied correctly -> R-1
- Given v2 mode + quarantine state read fails (404, no state branch) -> all tests run, no exclusions -> R-6
- Given v2 mode + quarantine state read fails (403) -> v2-specific error message -> UX-1

Fork PR detection:
- Given pull_request event, head repo != base repo -> skip quarantine -> SEC-7
- Given pull_request_target event, head repo != base repo -> skip quarantine -> SEC-7
- Given pull_request event, head repo == base repo -> quarantine runs normally -> SEC-7
- Given pull_request_target event, head repo == base repo -> quarantine runs normally -> SEC-7
- Given push event -> fork detection does not apply -> SEC-7
- Given GITHUB_EVENT_PATH malformed -> skip detection, quarantine runs -> SEC-9
- Given GITHUB_EVENT_PATH unset -> skip detection, quarantine runs -> SEC-9

---

### M18: Dashboard Write Processing

**Dependencies:** M15 (installation token infrastructure), M6/M7 (artifact
processing pipeline). M17 and M18 MAY be developed in parallel since they
touch disjoint codebases (Go CLI vs TypeScript dashboard).

**Scope -- included:**
- R-4: Artifact processing pipeline extended to perform GitHub writes
- R-6: State branch creation -- if state branch does not exist on first
  artifact processing, create it with initial empty state
- State update: read current state from state branch, merge new flaky tests,
  write via CAS (maximum 3 retries on 409 conflict, exponential backoff
  with jitter) using dashboard's App installation token
- Issue creation: for each new flaky test in artifact, check dedup via search
  API, create issue with title/body/labels matching CLI v1 format. Respect
  `issue_skipped_reason` field from results JSON (ADR-022): when set to
  `"new_file_in_pr"` or `"new_test_in_pr"`, skip issue creation for that test.
- Unquarantine: batch check closed issues via Search API **during artifact
  processing** (as part of the state merge step, before writing state).
  Remove tests whose tracking issues are closed.
- PR comment: post or update quarantine summary comment on the PR using the
  suite-specific marker (`<!-- quarantine:<suite-name> -->`). If `pr_number`
  is null (non-PR build), skip the PR comment step entirely. In v2 mode,
  append a note: `"Note: Quarantine changes take effect on the next CI run,
  not this one. The dashboard processes results after this run completes."`
  (stale-state communication — explains both the what and the why)
- R-5: Results JSON schema validation -- ensure suite_name, pr_number,
  commit_sha, branch, and per-test data (retry details, failure messages)
  are present and sufficient for rendering. `cli_mode` is OPTIONAL in the
  schema for backward compat with pre-M17 artifacts. Dashboard skip logic:
  when `cli_mode` is `"v1"` OR missing, skip all writes (CLI already
  performed them or this is a legacy artifact — safe default). Only when
  `cli_mode == "v2"` does the dashboard perform writes.
- SEC-3: App permission parity -- the dashboard's App MUST NOT have permissions
  beyond what the CLI requires (contents:rw, issues:rw, pull-requests:w,
  actions:r). Verify at startup, log warning for excess permissions.
- SEC-10: Sanitize `failure_message`, `name`, and `test_id` fields before
  rendering into GitHub issue/comment markdown. Escape characters that could
  be interpreted as markdown/HTML injection.
- SEC-11: Cross-check that the artifact's `repo` field matches the
  `owner/repo` the artifact was downloaded from. Reject mismatches with a
  warning.
- Resource bounds: reject artifacts with `tests` arrays exceeding 100,000
  entries. Truncate string fields exceeding 64KB before rendering.
- Issue dedup: search for existing open issue with
  `quarantine:<suite>:<hash>` label before creating
- State CAS: SHA-based compare-and-swap with maximum 3 retries on 409
  conflict, exponential backoff with jitter to reduce thundering-herd effects
  from concurrent artifact processing
- State merge: quarantine-wins semantics (per ADR-012)
- Validation ordering: schema validation (required fields, types) MUST run
  before the SEC-11 repo cross-check, so a missing `repo` field is caught
  as a schema error rather than a cross-check failure
- Write error handling: all writes are best-effort; failures logged as
  warnings, never crash the processing pipeline
- Write processing rate limiting: not required for v2 (polling-based
  ingestion naturally throttles). Revisit when webhook-triggered processing
  is added in v3 (see `docs/plans/webhooks.md`).
- Rendering parity contract tests: issue body and PR comment rendering
  verified against shared fixtures. Definition of "matches": given identical
  input data, dashboard output must be **byte-for-byte identical** to CLI v1
  output (excluding trailing whitespace). Shared fixtures in `testdata/`.

**Scope -- excluded:**
- CLI code changes (M17)
- E2E round-trip testing (M19)
- Webhook-triggered processing (v3)

**Acceptance criteria:**
1. Dashboard processes a quarantine artifact and writes updated state to the
   state branch via CAS with maximum 3 retries (R-4)
2. Dashboard creates state branch with initial empty state if absent on first
   artifact processing (R-6)
3. Dashboard creates issues for new flaky tests with correct title, body,
   labels matching CLI v1 format (R-4)
4. Dashboard dedup: no duplicate issue created when one already exists (R-4)
5. Dashboard respects `issue_skipped_reason`: tests with `"new_file_in_pr"` or
   `"new_test_in_pr"` skip issue creation (R-4, ADR-022)
6. Dashboard posts/updates PR comment with quarantine summary (R-4)
7. When `pr_number` is null (non-PR build), PR comment step is skipped (R-4)
8. Dashboard uses suite-specific markers and labels (per ADR-032) (R-4)
9. Results JSON missing required fields -> artifact skipped with warning (R-5)
10. When `cli_mode` is `"v1"` OR missing, dashboard skips all writes (CLI
    already performed them, or pre-M17 legacy artifact — safe default).
    Only `cli_mode == "v2"` triggers writes. (R-5)
11. State CAS conflict -> retry with merge (quarantine wins), max 3 retries,
    exponential backoff with jitter (R-4)
12. Issue creation failure -> logged, state still updated (R-4)
13. PR comment failure -> logged, does not affect state or issues (R-4)
14. Closed-issue check (during artifact processing): quarantined tests with
    closed tracking issues are removed from state (R-4)
15. `failure_message`, `name`, and `test_id` fields sanitized before markdown
    rendering -- no HTML/markdown injection possible (SEC-10)
16. Artifact `repo` field cross-checked against download source repo; mismatch
    -> artifact rejected with warning (SEC-11)
17. Artifacts with >100,000 test entries rejected; string fields exceeding
    64KB truncated before rendering (resource bounds)
18. App permissions beyond SEC-3 table logged as warning at startup; verified
    via `GET /app` API call comparing configured vs required permissions (SEC-3)
19. Issue body and PR comment output matches CLI v1 rendering byte-for-byte
    (excluding trailing whitespace) for the same input data (contract test)
    (R-4)
20. `make dash-test && make dash-lint && make dash-typecheck` pass

**Scenario outlines:**

State write:
- Given artifact with new flaky test, state branch exists -> state updated via CAS -> R-4
- Given artifact with new flaky test, state branch does not exist -> state branch created, initial state written -> R-6
- Given CAS 409 conflict -> re-read, merge (quarantine wins), retry (max 3) -> R-4
- Given CAS exhausted after 3 retries -> warning logged, artifact processing continues -> R-4

Issue creation:
- Given new flaky test, no existing issue -> issue created with v1-format title/body/labels -> R-4
- Given new flaky test, existing open issue with dedup label -> no duplicate issue created -> R-4
- Given new flaky test with issue_skipped_reason = "new_file_in_pr" -> no issue created -> R-4, ADR-022
- Given issue creation returns 410 (issues disabled) -> skip all issue creation for this run -> R-4
- Given issue creation fails (5xx) -> warning logged, state still updated -> R-4

PR comment:
- Given artifact with pr_number set -> PR comment posted/updated with suite marker -> R-4
- Given artifact with pr_number null -> PR comment step skipped entirely -> R-4
- Given existing quarantine comment on PR -> comment updated (PATCH), not duplicated -> R-4
- Given PR comment fails -> warning logged, does not affect state or issues -> R-4

Unquarantine:
- Given quarantined test with closed tracking issue -> removed from state during processing -> R-4
- Given quarantined test with open tracking issue -> remains in state -> R-4

Validation + security:
- Given artifact with cli_mode = "v1" -> all writes skipped -> R-5
- Given artifact with cli_mode missing (pre-M17 legacy) -> all writes skipped (treated as v1) -> R-5
- Given artifact missing required fields (other than cli_mode) -> artifact skipped with warning -> R-5
- Given artifact repo field != download source repo -> artifact rejected -> SEC-11
- Given artifact with >100,000 test entries -> artifact rejected -> SEC-11
- Given failure_message containing markdown injection -> sanitized before rendering -> SEC-10

**Spec references:**

| What | Reference |
|------|-----------|
| State merge semantics | cli/internal/quarantine/state.go |
| Issue body rendering | cli/cmd/quarantine/run_notifications.go |
| PR comment rendering | cli/cmd/quarantine/run_notifications.go |
| CAS write logic | cli/internal/cas/cas.go |
| Issue dedup | cli/internal/github/issues_ops.go |
| Results JSON schema | schemas/test-result.schema.json |
| State schema | schemas/quarantine-state.schema.json |
| Per-suite isolation | ADR-032 |
| Issue skip (new-to-PR) | ADR-022 |
| App permission parity | SEC-3 in v2-auth-token-proxy.md |

---

### M19: v2 Server-Side Writes E2E

**Dependencies:** M17 + M18, plus pre-seeding (see Pre-seeding below)

**Test approach:** M19 spawns the dashboard with `source: github-app` mode
(using the M16 App credentials) so it discovers and monitors both fixture
repos via `GET /installation/repositories`. The test then triggers sync
(via `GET /` like the existing `dashboard-sync.test.js`), reads dashboard
HTML to confirm ingestion happened, and queries the GitHub API to verify
write outcomes per fixture.

**Scope -- included:**
- E2E tests observe **pre-seeded fixture data** from two fixture repos.
  The App is installed on both, so the dashboard discovers and monitors
  both via the App installation:
  - `mycargus/quarantine-app-test-fixture`: v2 workflow (GITHUB_TOKEN only,
    uploads artifact). Dashboard processes artifacts and performs all writes
    (state, issues, PR comments) using App installation tokens.
  - `mycargus/quarantine-test-fixture`: existing v1 workflow (PAT, CLI
    writes, uploads artifact). Dashboard polls and ingests the artifact
    but skips writes because `cli_mode: "v1"`.
- E2E: v2 flow -- observe that the dashboard created state updates, issues,
  and PR comments for app-test-fixture runs (App bot identity)
- E2E: v1 cli_mode skip -- two-part verification:
  (a) Positive signal — dashboard's project detail page for the v1 fixture
      shows ingested test runs (proves the dashboard saw the artifact, not
      that it never reached the repo)
  (b) Negative signal — zero issues with `creator.type == "Bot"` exist on
      the v1 fixture; zero PR comments authored by the App bot; all
      CLI-created issues/comments have `creator.type == "User"`
- E2E: CLI exit code correctness -- v2 fixture CI runs exit 0 (all pass) or
  exit 1 (test failures) regardless of dashboard processing state

**Pre-seeding (one-time, before M19 E2E tests are meaningful):**
1. After M17 CLI is released, both fixture repos' workflows pull `latest`
   on next daily cron — so `cli_mode` will appear in artifacts produced
   from that day forward.
2. Trigger `workflow_dispatch` on both fixture repos to seed at least one
   M17-era run each. Wait for M18 dashboard to poll and ingest both.
3. Verify pre-seeding is complete:
   - `quarantine-app-test-fixture` has at least one issue created by the
     App bot AND at least one entry in its `quarantine/state` branch
     authored by the App bot (Git commit author).
   - `quarantine-test-fixture` has CLI-authored issues/comments only
     (PAT owner identity), and the dashboard's project page renders test
     runs for it (proving ingestion happened without writes).

**E2E environment variables:**

| Var | Used for |
|-----|----------|
| `QUARANTINE_GITHUB_TOKEN` | PAT with read access to BOTH fixture repos (existing var, scope expanded if needed) |
| `QUARANTINE_TEST_OWNER` / `QUARANTINE_TEST_REPO` | v1 fixture (existing) |
| `QUARANTINE_GH_APP_TEST_OWNER` / `QUARANTINE_GH_APP_TEST_REPO` | v2 fixture (M16-defined) |
| `QUARANTINE_APP_PRIVATE_KEY` / `QUARANTINE_APP_CLIENT_ID` / `QUARANTINE_APP_INSTALLATION_ID` | Dashboard App credentials (M16-defined) — required to spawn dashboard in `source: github-app` mode |

**CI job placement:** M19 tests live in `test/e2e/v2-server-side-writes.test.js`
and run in the M16-introduced `e2e-app` job (which has App credentials).
They MUST NOT run in the existing `e2e` job (which lacks App credentials
and would throw on missing required env vars per the existing E2E pattern).
Update `test/e2e/.env.example` to document all required vars.

**Scope -- excluded:**
- CLI or dashboard code changes (those are complete in M17/M18)
- Webhook-triggered processing (v3)
- Real-time artifact processing observation (not needed with pre-seeded data)
- Pre-seeding the fixture data itself — that is an operational step, not a
  test deliverable

**Acceptance criteria:**
1. E2E: at least one recent v2 run on `quarantine-app-test-fixture` has
   produced dashboard-written artifacts:
   - State branch: at least one commit on `quarantine/state` authored by
     the App bot (Git commit author email contains `[bot]`); per-suite
     state file at `.quarantine/<suite>/state.json` contains at least one
     quarantined test entry
   - Issues: at least one open issue with `quarantine` label has
     `creator.type == "Bot"` and `creator.login` matches the App's bot
     name (e.g., `quarantine-ci[bot]`)
   - PR comments: at least one comment on the v2 fixture's PR proxy has
     `<!-- quarantine:<suite> -->` marker AND `user.type == "Bot"`
2. E2E: `quarantine-test-fixture` (v1) verification (cli_mode skip):
   - Dashboard ingestion confirmed: `GET /projects/{v1_owner}/{v1_repo}` on
     a freshly-spawned dashboard renders the "Showing X of Y quarantined
     tests" phrase with non-zero values (proves the artifact was ingested
     into SQLite — dashboard saw it)
   - Skip confirmed: ZERO open or closed issues on the v1 fixture have
     `creator.type == "Bot"` (all are `creator.type == "User"`)
   - Skip confirmed: ZERO comments on the v1 fixture's PR proxy have
     `user.type == "Bot"`
3. E2E: at least one recent v2 workflow run on `quarantine-app-test-fixture`
   has `conclusion == "success"` (proves quarantine exclusion worked when
   dashboard-written state was applied); workflow conclusion is independent
   of dashboard processing state (verified via `GET /repos/.../actions/runs`)
4. E2E: existing `make e2e-test` (v1 fixture, single-token tests) continues
   to pass; new tests live under the `e2e-app` job and are gated by App
   credential env vars

**Spec references:**

| What | Reference |
|------|-----------|
| E2E test conventions | test/e2e/README.md |
| Existing dashboard E2E pattern | test/e2e/dashboard-sync.test.js |
| Test strategy | test-strategy.md#test-layers |
| M16 App credential setup | docs/milestones/m16.md |
| GITHUB_APP_SETUP.md | App installation on both fixtures |

## Spec updates required

- **error-handling.md**: Change degraded mode trigger 3 (no token) to exit 2.
  Note: this is a behavioral change from v1 where no-token entered degraded
  mode. Document migration impact. — *verified by doc review*
- **cli-spec.md**: Update token resolution to two-step order; document v2
  write-skip behavior; document v2 state-branch-missing handling (empty state,
  not exit 2) — *verified by doc review*
- **github-api-inventory.md**: Note that write operations are dashboard-side
  in v2 mode — *verified by doc review*
- **schemas/test-result.schema.json**: `cli_mode` field added (`"v1"` or `"v2"`) ✓

## Risks

- **Dashboard required for quarantine management in v2.** Without the dashboard,
  the CLI still runs tests and detects flaky tests, but state is never updated.
  Acceptable because v2 is the "GitHub App + dashboard" phase.
- **Stale state window.** If the dashboard is slow to process an artifact, the
  next CLI run reads stale state. A flaky test detected in run N might not be
  excluded until run N+2. CAS merge handles concurrent updates correctly.
  The v2 PR comment (posted by dashboard) should note that quarantine takes
  effect on the next run, not the current one.
- **Write logic migration.** Issue body rendering, PR comment rendering, state
  merge, CAS, and dedup must be reimplemented in TypeScript. Contract tests
  against shared fixtures verify byte-for-byte parity (excluding trailing
  whitespace) with CLI v1 output.
- **`quarantine init` in v2.** Init is run by users locally (never in CI) and
  creates `.quarantine/config.yml` plus the state branch. In v2, users running
  init locally may authenticate with a PAT or `gh auth` — init is not
  affected by the read-only CI constraint. If init has not been run, the
  dashboard creates the state branch on first artifact processing (R-6).
  The CLI handles a missing state branch by running all tests (R-6).

## Review summary

### Re-review 3 (2026-04-22) — M19 E2E scrutiny

| Reviewer | Verdict | Blockers | Observations |
|----------|---------|----------|--------------|
| Acceptance test | needs-revision -> addressed | 4 | 5 |
| Architecture | approve | 0 | 5 |
| UX | approve | 0 | 5 |
| Security | approve | 0 | 5 |

Blockers addressed:
- M19 AC#2 negative-assertion strategy: now uses two-part verification
  (positive ingestion signal via dashboard project page rendering, plus
  negative GitHub API checks for `creator.type == "Bot"` count == 0)
- Pre-M17 artifact handling: schema makes `cli_mode` optional; dashboard
  treats missing as v1 (skip writes — safe default). M18 AC#10 and
  scenarios updated.
- M19 env vars: explicit table added. Reuses M16 vars
  (`QUARANTINE_GH_APP_TEST_OWNER`/`REPO`) for the v2 fixture.
- M19 PAT scope: documented requirement that `QUARANTINE_GITHUB_TOKEN`
  has read access to BOTH fixture repos.

Observations addressed:
- M19 AC#1: state branch attribution via Git commit author check spelled out
- M19 AC#3: clarified expectation (at least one recent run with
  `conclusion == "success"`)
- M19 manifest: `make e2e-app-test` reference clarified (M16 prerequisite)
- Pre-seeding checklist added to M19 scope
- Stale-state PR comment wording expanded with "why" clause
- M17 scope: bot identity change documented for CI integration guide
- M17 scope: artifact naming convention (`quarantine-results-<suite>-<run_id>`) precision noted

### Re-review 2 (2026-04-20)

| Reviewer | Verdict | Blockers | Observations |
|----------|---------|----------|--------------|
| Acceptance test | approve | 0 | 5 |
| Architecture | approve | 0 | 4 |
| UX | approve | 0 | 5 |
| Security | approve | 0 | 5 |

All previous blockers verified as resolved. Observations addressed in this revision:
- `cli_mode` field added to `schemas/test-result.schema.json` (cross-cutting)
- M18 AC#15: `test_id` added to sanitization scope (SEC-10 completeness)
- M18 AC#17: 64KB string truncation added to resource bounds criterion
- M18 AC#18: `GET /app` mechanism specified for permission verification
- M17: v1-to-v2 migration scenario added (user removes QUARANTINE_GITHUB_TOKEN)
- M18: stale-state PR comment wording specified
- M18: validation ordering constraint added (schema before SEC-11 cross-check)
- M18: write processing rate limiting noted as v3 concern
- M19: acceptance criteria made more specific (creator.type checks, workflow conclusion API)
- M19: AC#4 added (existing E2E tests unbroken)
- `docs/milestones/index.md`: Phase 6b corrected to show M17/M18 as parallel
- M18 manifest: `artifact-poller.server.ts` reference corrected to `sync.server.ts`

### Review 1

| Reviewer | Verdict | Key findings |
|----------|---------|-------------|
| Acceptance test | needs-revision -> addressed | M18 scenario outlines added; UX-1/UX-6 message text specified; CAS retry count added (3); unquarantine trigger specified (during artifact processing); same-repo PR scenario added; pr_number null scenario added; issue_skipped_reason handling added; rendering parity defined (byte-for-byte); doc items marked "verified by doc review" |
| Architecture | approve | M17/M18 parallelism opportunity noted; dependency normalized to M16; CAS backoff/jitter specified; quarantine[bot] identity clarified (App bot identity from dashboard installation token) |
| UX | needs-revision -> addressed | ERR-1/UX-6 v2 message text specified; UX-1 v2 vs v1 messages specified; artifact upload note includes doc link + --quiet suppression; init in v2 clarified (local-only, dashboard creates state branch); cli_mode field added to results JSON; verbose token resolution logging added; stale-state communication addressed in PR comment |
| Security | approve | SEC-3 relabeled active for dashboard writes; SEC-10 (markdown sanitization) added; SEC-11 (artifact repo cross-check) added; resource bounds added (100K tests, 64KB strings); pull_request_target same-repo scenario added |
