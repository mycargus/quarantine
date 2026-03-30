# Contracts

> Last updated: 2026-03-30
>
> Every boundary where one component produces data that another component
> consumes. A contract breaks when the producer changes the data shape or
> behavior without the consumer adapting. This document captures both explicit
> contracts (defined by schemas or specs) and implicit contracts (conventions
> baked into code).
>
> **This document describes what contracts exist, not project status.** Bugs,
> known issues, and test gaps belong in user scenarios
> (`docs/scenarios/`) and milestone acceptance criteria
> (`docs/milestones/`). The "Tested" column and "Current validation"
> sections describe how a contract is verified, not what's broken.

## Summary

Quick reference for every producer-consumer boundary. See the detailed
sections below for full context on each contract.

- **Explicit** contracts have a schema or spec as the source of truth.
- **Implicit** contracts are conventions baked into code with no formal
  definition.
- **Protocol** describes the data format for data-at-rest contracts or the
  convention/rule for behavioral contracts.

| Contract | Type | Location | Protocol | Producer | Consumer | Purpose | Tested |
|----------|------|----------|----------|----------|----------|---------|--------|
| [`quarantine.yml`](#1-quarantineyml-cli-configuration) | Explicit | Repo root | YAML | User (`quarantine init`) | CLI | CLI configuration: framework, retries, repo, labels, notifications, storage, excludes | Go unit tests (`config_test.go`); not validated against JSON schema |
| [`quarantine.json`](#2-quarantinejson-quarantine-state) | Explicit | `quarantine/state` branch via Contents API | JSON | CLI | CLI | Active quarantine state: which tests are quarantined, associated issues | Go unit tests (`state_test.go`, `state_merge_test.go`); not validated against JSON schema |
| [`results.json`](#3-resultsjson) | Explicit | Local, then GitHub Artifact | JSON | CLI | Dashboard | Test run results: statuses, retries, flaky detection, summary | Go unit tests (`result_test.go`); dashboard validates against schema at runtime via ajv |
| [JUnit XML](#4-junit-xml) | Implicit | Local (e.g. `junit.xml`) | XML | Test runner (Jest, RSpec, Vitest) | CLI parser | Raw test results from the framework | Go unit tests (`parser_*_test.go`) with XML fixtures in `testdata/junit-xml/` |
| [Contents API](#5-contents-api) | Explicit | GitHub API | REST / JSON | CLI | GitHub | Read/write `quarantine.json` on state branch via CAS | Prism contract tests planned; no test files yet |
| [Issues API](#6-issues-api) | Explicit | GitHub API | REST / JSON | CLI | GitHub | Create GitHub Issues for flaky tests (HTTP shape; see [Issue creation](#13-issue-creation) for content conventions) | Prism contract tests planned; no test files yet |
| [Search API](#7-search-api) | Explicit | GitHub API | REST / JSON | CLI | GitHub | Find existing issues to avoid duplicates (HTTP shape; see [Issue dedup labels](#14-issue-dedup-labels) for label conventions) | Prism contract tests planned; no test files yet |
| [Comments API](#8-comments-api) | Explicit | GitHub API | REST / JSON | CLI | GitHub | Post and update PR comments (HTTP shape; see [PR comment format](#12-pr-comment-format) for content conventions) | Prism contract tests planned; no test files yet |
| [Refs API](#9-refs-api) | Explicit | GitHub API | REST / JSON | CLI | GitHub | Check/create `quarantine/state` branch | Prism contract tests planned; no test files yet |
| [Repository API](#10-repository-api) | Explicit | GitHub API | REST / JSON | CLI | GitHub | Verify repo exists and token has access | Prism contract tests planned; no test files yet |
| [Artifacts API](#11-artifacts-api) | Explicit | GitHub API | REST / JSON | GitHub | Dashboard | List and download test result artifacts | Prism contract tests planned; no test files yet |
| [PR comment format](#12-pr-comment-format) | Implicit | GitHub PR via Comments API | Markdown with `<!-- quarantine-bot -->` marker | CLI | Humans, CLI (update detection) | Content convention for PR comments (built on [Comments API](#8-comments-api)) | Not tested |
| [Issue creation](#13-issue-creation) | Implicit | GitHub Issues API | Markdown body + label array | CLI | Humans, CLI (dedup search) | Content convention for flaky test issues (built on [Issues API](#6-issues-api)) | Not tested |
| [Issue dedup labels](#14-issue-dedup-labels) | Implicit | GitHub Issues + Search API | `["quarantine", "quarantine:{hash}"]` | CLI | CLI (search) | Label convention for deduplication (built on [Search API](#7-search-api)) | Dedup logic tested (`run_issues_dedup_test.go`); label format not tested |
| [Test ID format](#15-test-id-format) | Implicit | `parser.go` | `file_path::classname::name` string | CLI parser | State (map key), issues (hash input), results (display) | Stable test identity across runs for quarantine lookup and issue dedup | Parser tests assert construction; no cross-component test |
| [Artifact naming](#16-artifact-naming-convention) | Implicit | CI workflow YAML, `github.server.ts` | `quarantine-results-{run_id}` name prefix | CI workflow (`actions/upload-artifact`) | Dashboard (`filterArtifactsByPrefix()`) | Dashboard identifies which artifacts contain quarantine results | Not tested |
| [CLI exit codes](#17-cli-exit-codes) | Implicit | `run.go` | Integer: 0=pass, 1=failures, 2=infra error | CLI | CI pipeline, scripts, users | CI determines build pass/fail from exit code | Extensively tested (`run_*_test.go`) |
| [Rate limit headers](#18-rate-limit-headers) | Implicit | `init_ops.go` | `X-RateLimit-Remaining` / `X-RateLimit-Limit` response headers | GitHub API | CLI | Warn before rate limit exhaustion | Not tested |

## Schemas

Validation definitions that enforce contracts. Schemas don't have producers
and consumers — they define the expected shape that both sides agree on.

| Schema | Location | Protocol | Validates | Used at | Tested |
|--------|----------|----------|-----------|---------|--------|
| `test-result.schema.json` | `schemas/` | JSON Schema (draft 2020-12) | `results.json` | Dashboard runtime (ajv); Go marshal test (planned, ADR-025); negative regression test (planned, ADR-025) | Dashboard validates at runtime; Go marshal validation planned |
| `quarantine-state.schema.json` | `schemas/` | JSON Schema (draft 2020-12) | `quarantine.json` | Negative regression test (planned, ADR-025) | Single-component; `issue_number`/`issue_url` to be made optional (ADR-025) |
| `quarantine-config.schema.json` | `schemas/` | JSON Schema (draft 2020-12) | `quarantine.yml` | Negative regression test (planned, ADR-025) | Documentation artifact; CLI validates via Go code, not schema |
| `github-api-artifacts.json` | `schemas/` | OpenAPI 3.x | GitHub Artifacts API responses | Prism contract tests (planned) | Not validated yet; `test/contract/` has no test files |

## Contract Details

### 1. quarantine.yml (CLI configuration)

**Why:** Users configure the CLI's behavior: which test framework, how many
retries, which repo, which labels, etc. The config file is the contract between
the user and the CLI.

**Where:** `quarantine.yml` in the repository root. Created by
`quarantine init`.

**When:** CLI reads it at the start of every command. Validated immediately
after parsing.

**Parties:**
- Producer: User (via `quarantine init` or manual editing)
- Consumer: CLI (`config.Load()` in `cli/internal/config/config.go`)
- Schema: `schemas/quarantine-config.schema.json`

**Current validation:**
- CLI validates via Go code (`config.Validate()`), not against the JSON schema.
  The Go validation and JSON schema are independent implementations of the same
  rules and could drift.
- Unknown keys are detected via two-pass YAML decoding and emitted as warnings
  (not errors).

---

### 2. quarantine.json (quarantine state)

**Why:** The CLI needs persistent state to know which tests are quarantined
across CI runs. The state file is stored on a dedicated Git branch and accessed
via the GitHub Contents API, using SHA-based compare-and-swap for optimistic
concurrency (ADR-012).

**Where:** Stored as `quarantine.json` on the `quarantine/state` branch (or
whatever branch `storage.branch` in config specifies). Read and written via the
GitHub Contents API.

**When:** CLI reads it at the start of `quarantine run` to know which tests to
exclude or mark. CLI writes it after detecting new flaky tests. Concurrent CI
runs may race; CAS with retry on 409 resolves conflicts.

**Parties:**
- Producer: CLI (`State.MarshalAt()` in `cli/internal/quarantine/state.go`)
- Consumer: CLI (`ParseState()` in `cli/internal/quarantine/state.go`)
- Transport: GitHub Contents API (`cli/internal/cas/cas.go`)
- Schema: `schemas/quarantine-state.schema.json`

**Current validation:**
- No runtime schema validation. CLI trusts its own output.
- Golden fixtures in `testdata/quarantine-state/` are not validated against the
  schema.
- Scenario 66 in `docs/scenarios/v1/10-github-api-edge-cases.md` covers a
  struct-schema mismatch (not yet tested).

---

### 3. results.json

**Why:** The CLI and dashboard are independently developed components that never
talk to each other directly. The CLI produces test run results; the dashboard
consumes them via GitHub Artifacts. The JSON schema is the shared agreement
that keeps them compatible.

**Where:** CLI writes to `.quarantine/results.json` locally. CI uploads it as a
GitHub Artifact named `quarantine-results-{run_id}`. Dashboard downloads the
artifact ZIP, extracts the JSON, validates it against the schema, and ingests
it into SQLite.

**When:** CLI produces it at the end of every `quarantine run`. Dashboard
consumes it during artifact polling.

**Parties:**
- Producer: CLI (`result.Build*()` in `cli/internal/result/result.go`)
- Consumer: Dashboard (`ingest.server.ts:validateTestResult()` +
  `ingest.server.ts:ingestArtifact()`)
- Schema: `schemas/test-result.schema.json`

**Current validation:**
- Dashboard validates incoming JSON against the schema at runtime via ajv.
- CLI does not validate its own output against the schema.
- Golden fixtures in `testdata/expected/` represent expected output but are not
  validated against the schema.
- Scenario 72 in `docs/scenarios/v1/09-test-runner-edge-cases.md` covers a
  parser status value not in the schema enum (not yet tested).

---

### 4. JUnit XML

**Why:** The CLI needs to understand test results from multiple frameworks.
JUnit XML is the de facto interchange format, but there is no official schema.
Each framework produces slightly different XML.

**Where:** Test runners write XML locally (e.g. `junit.xml`). CLI reads it via
the `junitxml` config glob.

**When:** After the test runner completes, CLI parses the XML to build test
results.

**Parties:**
- Producer: Test runner (Jest via `jest-junit`, RSpec via
  `rspec_junit_formatter`, Vitest built-in)
- Consumer: CLI parser (`cli/internal/parser/parser.go`)

**Contract details:**
- Root element: `<testsuites>` (Jest, Vitest) or `<testsuite>` (RSpec)
- Required attributes: `testcase.classname`, `testcase.name`
- Optional attributes: `testcase.file` (Jest if configured, RSpec always),
  `testcase.time`
- Status determined by child elements: `<failure>`, `<error>`, `<skipped>`, or
  none (passed)
- Framework-specific: Vitest uses `testsuite.name` as file path; RSpec uses
  `testcase.file`

**Current validation:**
- Parser unit tests with XML fixtures per framework
  (`testdata/junit-xml/{jest,rspec,vitest}/`)
- Missing: `<error>` element handling test, missing-attribute edge cases

---

### 5. Contents API

**Why:** The CLI stores `quarantine.json` on a dedicated Git branch and
accesses it via the GitHub Contents API. This is the transport layer for the
quarantine state contract.

**Where:** `cli/internal/github/contents_ops.go`

**When:** Every `quarantine run` reads state at the start and writes it after
detecting flaky tests.

**Parties:**
- Consumer: CLI
- Provider: GitHub Contents API

**Endpoints:**
- `GET /repos/{owner}/{repo}/contents/{path}?ref={branch}` — read state file
- `PUT /repos/{owner}/{repo}/contents/{path}` — write state file (CAS via SHA)

**Response shape consumed:**
- `content`: base64-encoded file content
- `sha`: required for CAS writes; included in PUT body to detect conflicts

**Error handling:**
- 404: branch or file doesn't exist (see scenarios 64–65 in
  `docs/scenarios/v1/10-github-api-edge-cases.md`)
- 409: concurrent write conflict; CLI retries with fresh SHA
- 422: file exceeds 1MB size limit

**Current validation:** Prism contract tests planned; no test files yet.
Scenarios 64–65 cover base64 decoding and branch-not-found detection edge
cases (not yet tested).

---

### 6. Issues API

**Why:** The CLI creates GitHub Issues to track flaky tests. Each flaky test
gets its own issue with structured labels for deduplication.

**Where:** `cli/internal/github/issues_ops.go`

**When:** After detecting a flaky test, if no existing issue is found via
the Search API.

**Parties:**
- Consumer: CLI
- Provider: GitHub Issues API

**Endpoints:**
- `POST /repos/{owner}/{repo}/issues` — create a new issue

**Request shape produced:**
- `title`: structured test name
- `body`: Markdown with test details
- `labels`: array (see [Issue creation](#13-issue-creation) for content
  conventions and [Issue dedup labels](#14-issue-dedup-labels) for label
  structure)

**Response shape consumed:**
- `number`: issue number (stored in quarantine state)
- `html_url`: issue URL (stored in quarantine state)

**Error handling:**
- 410 Gone: GitHub Issues disabled on the repo

**Current validation:** Prism contract tests planned; no test files yet.

---

### 7. Search API

**Why:** The CLI searches for existing issues before creating new ones to avoid
duplicates. The search uses label-based queries against the GitHub Search API.

**Where:** `cli/internal/github/issues_ops.go`

**When:** After detecting a flaky test, before creating an issue.

**Parties:**
- Consumer: CLI
- Provider: GitHub Search API

**Endpoints:**
- `GET /search/issues?q={query}` — search for open/closed issues by label

**Request shape produced:**
- Query: `repo:{owner}/{repo} is:issue is:open label:quarantine label:quarantine:{hash}`
- See [Issue dedup labels](#14-issue-dedup-labels) for label structure and hash
  computation.

**Response shape consumed:**
- `total_count`: number of matching issues
- `items[].number`: issue number
- `items[].html_url`: issue URL

**Contract details:**
- Pagination: max 1000 results (10 pages x 100/page)
- Dedup logic requires `total_count > 0` AND `items` non-empty

**Current validation:** Mutation tests cover `total_count > 0` condition
(`issues_test.go`). Prism contract tests planned; no test files yet.

---

### 8. Comments API

**Why:** The CLI posts summary comments on PRs and updates them on subsequent
runs. Uses the GitHub Issues/Comments API (PRs are issues in GitHub's model).

**Where:** `cli/internal/github/issues_ops.go`

**When:** At the end of `quarantine run` if the run is associated with a PR and
`notifications.github_pr_comment` is enabled.

**Parties:**
- Consumer: CLI
- Provider: GitHub Issues/Comments API

**Endpoints:**
- `GET /repos/{owner}/{repo}/issues/{number}/comments` — list existing comments
- `POST /repos/{owner}/{repo}/issues/{number}/comments` — create new comment
- `PATCH /repos/{owner}/{repo}/issues/comments/{id}` — update existing comment

**Response shape consumed:**
- `id`: comment ID (for updates)
- `body`: comment body (scanned for marker; see [PR comment format](#12-pr-comment-format)
  for content conventions)

**Current validation:** Prism contract tests planned; no test files yet.

---

### 9. Refs API

**Why:** The CLI needs to check whether the `quarantine/state` branch exists
and create it during `quarantine init`.

**Where:** `cli/internal/github/init_ops.go`

**When:** During `quarantine init` and at the start of `quarantine run`.

**Parties:**
- Consumer: CLI
- Provider: GitHub Git Refs API

**Endpoints:**
- `GET /repos/{owner}/{repo}/git/ref/heads/{ref}` — check if branch exists
- `POST /repos/{owner}/{repo}/git/refs` — create branch

**Request shape produced (create):**
- `ref`: must be `refs/heads/{name}` (full ref path, not just branch name)
- `sha`: commit SHA to branch from

**Response shape consumed:**
- `ref`: ref path
- `object.sha`: commit SHA

**Error handling:**
- 404: branch doesn't exist (returns `exists=false`, not an error)

**Current validation:** Prism contract tests planned; no test files yet.

---

### 10. Repository API

**Why:** The CLI verifies the repository exists and the token has sufficient
permissions during `quarantine init`.

**Where:** `cli/internal/github/init_ops.go`

**When:** During `quarantine init`.

**Parties:**
- Consumer: CLI
- Provider: GitHub Repository API

**Endpoints:**
- `GET /repos/{owner}/{repo}` — get repository info

**Response shape consumed:**
- `id`: repository ID
- `full_name`: `owner/repo`
- `default_branch`: default branch name
- `private`: visibility

**Error handling:**
- 403 Forbidden: token lacks permission
- 404 Not Found: repo doesn't exist

**Current validation:** Prism contract tests planned; no test files yet.

---

### 11. Artifacts API

**Why:** The dashboard polls GitHub for test result artifacts produced by CLI
runs. This is the only GitHub API the dashboard uses.

**Where:** `dashboard/app/lib/github.server.ts`

**When:** During artifact polling (periodic or on-demand).

**Parties:**
- Consumer: Dashboard
- Provider: GitHub Artifacts API

**Endpoints:**
- `GET /repos/{owner}/{repo}/actions/artifacts?per_page=100` — list artifacts
- `GET {archive_download_url}` — download artifact ZIP (follows 302 redirect)

**Request shape produced:**
- `If-None-Match` header with cached ETag for conditional requests

**Response shape consumed:**
- `total_count`: number of artifacts
- `artifacts[].id`: artifact ID
- `artifacts[].name`: artifact name (filtered by prefix)
- `artifacts[].archive_download_url`: download URL
- `artifacts[].created_at`: ISO 8601 timestamp
- `artifacts[].expires_at`: ISO 8601 timestamp

**Contract details:**
- 304 Not Modified: returned when ETag matches; dashboard skips processing
- ZIP extraction: dashboard extracts the first JSON file from the ZIP
- Pagination: not implemented; only first 100 artifacts are fetched

**Current validation:** Vendored spec exists (`schemas/github-api-artifacts.json`).
Prism contract tests planned; no test files yet.

---

### 12. PR comment format

**Why:** The CLI posts a summary comment on PRs with flaky test results. On
subsequent runs, it updates the existing comment rather than creating a new
one. The comment marker is the detection mechanism. This is a content
convention built on top of the [Comments API](#8-comments-api) HTTP contract.

**Where:** Posted via the Comments API. Comment body starts with
`<!-- quarantine-bot -->` HTML comment.

**When:** At the end of `quarantine run` if `notifications.github_pr_comment`
is enabled and the run is associated with a PR.

**Parties:**
- Producer: CLI (`run_notifications.go`)
- Consumer: CLI on subsequent runs (finds existing comment by marker),
  humans reading the PR

**Contract details:**
- Marker `<!-- quarantine-bot -->` must be the first line for detection
- Body is structured Markdown: summary table, sections for flaky/quarantined/
  failed tests, footer with CLI version
- Update replaces the entire comment body (not append)

**Current validation:** None. No test verifies the marker is present, the
comment format, or the update-vs-create logic.

---

### 13. Issue creation

**Why:** Each flaky test gets a GitHub Issue so the team can track and resolve
it. The issue format must be consistent and machine-parseable for dedup. This
is a content convention built on top of the [Issues API](#6-issues-api) HTTP
contract.

**Where:** `cli/cmd/quarantine/run_notifications.go`

**When:** After detecting a flaky test and confirming no existing issue via
the Search API.

**Parties:**
- Producer: CLI
- Consumer: Humans (read the issue), CLI (searches by labels on subsequent
  runs)

**Contract details:**
- Title: structured test name
- Body: Markdown with test details, failure message, CI run link
- Labels: exactly two — the base label (default `"quarantine"`) and the hash
  label (`quarantine:{hash}`). See [Issue dedup labels](#14-issue-dedup-labels).
- 410 Gone handling: if Issues are disabled on the repo, CLI warns and
  continues

**Current validation:** Not tested. No test asserts the issue title format,
body structure, or label array.

---

### 14. Issue dedup labels

**Why:** The CLI creates GitHub Issues for flaky tests and uses label-based
search to avoid creating duplicates. The label structure is the deduplication
key. This is a label convention built on top of the [Issues API](#6-issues-api)
and [Search API](#7-search-api) HTTP contracts.

**Where:** CLI creates issues with labels in `run_notifications.go`. CLI
searches for issues by labels in `issues_ops.go`.

**When:** After detecting a flaky test, CLI searches for an existing issue
before creating a new one.

**Parties:**
- Producer: CLI (creates issues with labels)
- Consumer: CLI (searches by labels)
- External: GitHub Search API

**Contract details:**
- Two labels required: the base label (default `"quarantine"`) and the hash
  label (`quarantine:{8-char-hex}`)
- Hash is deterministic: first 8 hex characters of `SHA-256(test_id)`
- Custom base label configurable via `labels` in `quarantine.yml`
- If label structure changes, existing issues won't be found, creating
  duplicates

**Current validation:**
- Dedup logic tested in `run_issues_dedup_test.go`
- Label structure itself not validated (no test asserts the exact label format)

---

### 15. Test ID format

**Why:** Test identity must be stable across CI runs for quarantine state
lookup, issue deduplication, and result tracking. The test ID is the primary
key for quarantine state (`quarantine.json`) and the input to the issue dedup
hash.

**Where:** Constructed in `parser.go` as `file_path::classname::name`.
Used as map key in quarantine state, input to SHA-256 hash for issue labels,
and display identifier in results and PR comments.

**When:** Every time the CLI parses JUnit XML.

**Parties:**
- Producer: CLI parser
- Consumers: Quarantine state (map key), issue dedup (hash input), results
  (display), PR comments (display)

**Contract details:**
- Format: `{file_path}::{classname}::{name}`
- `file_path` extraction is framework-specific and fragile:
  - Jest: `testcase.file` attribute (if present) or `testsuite.name`
  - RSpec: `testcase.file` attribute (required)
  - Vitest: `testsuite.name` (used as file path)
- If a framework changes how it populates these attributes, test IDs shift,
  breaking quarantine state lookups and creating duplicate issues.

**Current validation:**
- Parser tests assert test ID construction per framework
- No cross-component test verifying the same test ID is used for state lookup
  and issue dedup

---

### 16. Artifact naming convention

**Why:** The dashboard needs to find CLI-produced artifacts among all artifacts
in a repository. The naming convention is how it identifies which artifacts
contain quarantine results.

**Where:** CLI writes to `.quarantine/results.json`. The CI workflow
(`actions/upload-artifact@v4`) uploads it with a name like
`quarantine-results-{run_id}`. Dashboard filters artifacts by name prefix.

**When:** CLI produces the file; CI uploads it; dashboard polls for it.

**Parties:**
- Producer: CI workflow (names the artifact)
- Consumer: Dashboard (`ingest.server.ts:filterArtifactsByPrefix()`)

**Contract details:**
- Name prefix: `quarantine-results-` (implied by README and dashboard code)
- Not enforced by schema or configuration — purely conventional
- Dashboard currently fetches first 100 artifacts only (no pagination)

**Current validation:** None.

---

### 17. CLI exit codes

**Why:** CI pipelines and scripts depend on exit codes to determine whether the
build should pass or fail. The exit code contract is the most critical implicit
contract — a wrong exit code can either break builds unnecessarily or silently
pass broken builds.

**Where:** `cli/cmd/quarantine/run.go`

**When:** At the end of every `quarantine run`.

**Parties:**
- Producer: CLI
- Consumer: CI pipeline (GitHub Actions `if: steps.*.outcome`), shell scripts,
  users

**Contract details:**
- `0`: All tests passed, or all failures are quarantined/flaky (build
  protected)
- `1`: Genuine test failures remain after retries
- `2`: Quarantine infrastructure failure in `--strict` mode

**Current validation:** Extensively tested in `run_*_test.go`. Well-covered.

---

### 18. Rate limit headers

**Why:** The CLI is designed for `GITHUB_TOKEN` (1,000 req/hr), not PAT
(5,000/hr). It monitors rate limit headers to warn before exhaustion.

**Where:** `cli/internal/github/init_ops.go` reads rate limit headers from
response.

**When:** After every GitHub API call.

**Parties:**
- Provider: GitHub API (response headers)
- Consumer: CLI

**Contract details:**
- Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`,
  `X-RateLimit-Reset`
- Warning emitted if `remaining * 10 < limit` (under 10% remaining)
- If GitHub changes header names or semantics, warnings stop working silently

**Current validation:** None for header parsing. Rate limit design documented
in architecture.

---

## Adding a new contract

When you introduce a new boundary where one component produces data that
another consumes, add it to this document:

1. Add a row to the [Summary](#summary) table.
2. Add a detailed section under [Contract Details](#contract-details) with
   Why, Where, When, Parties, and Current validation.
3. If the contract has a formal schema, add it to the [Schemas](#schemas) table.
4. If the contract is implicit, consider whether it should be promoted to
   explicit (by adding a schema or spec).

Signs you've introduced a new contract:
- Code parses or destructures data from another component or external service.
- Code produces output that another component or external service will consume.
- Code depends on a naming convention, header value, or string format that
  isn't enforced by a type system.

---

*References: [test-strategy.md](test-strategy.md),
[architecture.md](../planning/architecture.md),
[error-handling.md](error-handling.md),
[ADR-012 concurrency](../adr/012-concurrency-strategy.md),
[ADR-024 contract testing](../adr/024-contract-testing-tool.md),
[ADR-025 schema validation](../adr/025-schema-validation-strategy.md).*
