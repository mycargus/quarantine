# GitHub API Interaction Inventory

> Last updated: 2026-03-17
>
> Implementation reference for all GitHub API calls made by the Quarantine CLI
> and dashboard. Covers endpoints, permissions, error handling, retry behavior,
> and degraded-mode fallbacks.
>
> Related docs:
> - `docs/architecture.md` -- system design and data flows
> - `docs/adr/006-state-storage.md` -- quarantine.json storage strategy
> - `docs/adr/008-auth-strategy.md` -- authentication strategy
> - `docs/adr/012-concurrency-strategy.md` -- concurrency and CAS
> - `docs/adr/015-rate-limiting-and-ddos.md` -- rate limiting

## Table of Contents

1. [Authentication and Rate Limits](#authentication-and-rate-limits)
2. [Common Error Handling](#common-error-handling)
3. [CLI API Interactions](#cli-api-interactions)
   - [1. Read quarantine.json from branch](#1-read-quarantinejson-from-branch)
   - [2. Write quarantine.json to branch](#2-write-quarantinejson-to-branch)
   - [3. Create quarantine/state branch](#3-create-quarantinestate-branch)
   - [4. Batch check for closed quarantine issues](#4-batch-check-for-closed-quarantine-issues)
   - [5. Search for existing GitHub issue (dedup check)](#5-search-for-existing-github-issue-dedup-check)
   - [6. Create GitHub issue](#6-create-github-issue)
   - [7. Post PR comment](#7-post-pr-comment)
   - [8. Update existing PR comment](#8-update-existing-pr-comment)
4. [Dashboard API Interactions](#dashboard-api-interactions)
   - [9. List artifacts for a repo](#9-list-artifacts-for-a-repo)
   - [10. Download artifact](#10-download-artifact)
5. [NOT in v1](#not-in-v1)
6. [Rate Limit Budget Analysis](#rate-limit-budget-analysis)
7. [Open Questions](#open-questions)

---

## Authentication and Rate Limits

### Token Resolution

The CLI resolves the GitHub token in this order:

1. `QUARANTINE_GITHUB_TOKEN` environment variable (preferred)
2. `GITHUB_TOKEN` environment variable (fallback)
3. If neither is set: error on `quarantine init`, degraded mode on `quarantine run`

The dashboard uses a GitHub PAT configured as an environment variable.

### Rate Limit Tiers

| Token Type | Limit | Typical Use |
|---|---|---|
| `GITHUB_TOKEN` (Actions) | 1,000 req/hr per repo | Free, but tight budget |
| PAT (classic) | 5,000 req/hr per user | Recommended for v1 |
| Fine-grained PAT | 5,000 req/hr per user | Alternative to classic PAT |
| GitHub App installation | 5,000-12,500 req/hr (scales with repo count) | v2+ |

### Required Token Scopes (v1)

**CLI (PAT):**
- `repo` -- read/write contents (quarantine.json), create issues, post PR comments

**Dashboard (PAT):**
- `actions:read` -- list and download artifacts
- `repo` -- needed if the repository is private (artifacts API requires repo scope for private repos)

**CLI (`GITHUB_TOKEN` in GitHub Actions):**
- Automatically scoped to the triggering repository
- Has `contents: write`, `issues: write`, `pull-requests: write` by default
- Does NOT have cross-repo access
- Rate limited to 1,000 req/hr per repo

### Rate Limit Header Tracking

Every GitHub API response includes these headers:

```
X-RateLimit-Limit: 5000
X-RateLimit-Remaining: 4987
X-RateLimit-Reset: 1713456789
X-RateLimit-Used: 13
X-RateLimit-Resource: core
```

The CLI and dashboard SHOULD:
- Read `X-RateLimit-Remaining` after every API call
- Log a warning to stderr when remaining drops below 10% of the limit
- In GHA, emit `::warning` annotation when remaining drops below 10%

The Search API has a separate rate limit (30 req/min for authenticated users) tracked
via `X-RateLimit-Resource: search`.

---

## Common Error Handling

These error handling patterns apply across all API interactions. Each interaction
section below specifies which patterns apply and any interaction-specific overrides.

### Shared Error Responses

| Status | Meaning | Default Behavior |
|---|---|---|
| 401 Unauthorized | Token invalid, expired, or missing | Log error with diagnostic ("check QUARANTINE_GITHUB_TOKEN"). Enter degraded mode. |
| 403 Forbidden | Token lacks required scope, or secondary rate limit hit | Check `Retry-After` header. If present, treat as rate limit (backoff and retry). Otherwise, log permission error with required scope. Enter degraded mode. |
| 404 Not Found | Resource does not exist | Interaction-specific (see each section). |
| 409 Conflict | Concurrent write conflict | Interaction-specific (see each section). |
| 422 Unprocessable Entity | Malformed request body or validation error | Log error with response body (contains field-level errors). Enter degraded mode. |
| 429 Too Many Requests | Primary rate limit exceeded | Read `Retry-After` header (seconds). Backoff for that duration, then retry once. If no header, backoff 60s. If retry fails, enter degraded mode. |
| 500 Internal Server Error | GitHub server error | Retry once after 2s. If retry fails, enter degraded mode. |
| 502 Bad Gateway | GitHub infrastructure issue | Retry once after 2s. If retry fails, enter degraded mode. |
| 503 Service Unavailable | GitHub maintenance or overload | Read `Retry-After` header if present. Retry once after indicated duration (or 5s). If retry fails, enter degraded mode. |
| Network timeout | DNS failure, connection timeout, read timeout | Retry once after 2s. If retry fails, enter degraded mode. |

### Degraded Mode (CLI)

When the CLI enters degraded mode for any API call:

1. The operation is skipped (not retried further).
2. A warning is logged to stderr: `[quarantine] WARNING: <operation> failed (<reason>). Running in degraded mode.`
3. If `GITHUB_ACTIONS` env var is set, emit: `::warning title=Quarantine degraded mode::<operation> failed (<reason>)`
4. The CLI continues execution. Exit code reflects test results only, never infrastructure failures.
5. Exception: if `--strict` flag is set, infrastructure failures cause exit code 2 instead of degraded mode.

### Degraded Mode (Dashboard)

When the dashboard encounters an API error:

1. The current poll cycle is skipped.
2. The error is logged.
3. The next poll cycle proceeds normally.
4. Circuit breaker: 3 consecutive failures for a repo triggers a 30-minute pause for that repo (ADR-015).

### Retry Strategy

Unless otherwise noted, retries use:
- Max 1 retry for server errors (500/502/503) and timeouts
- Delay: 2 seconds before retry (or `Retry-After` header value if present)
- No retry for client errors (401, 403 without `Retry-After`, 404, 422)
- CAS conflicts (409) have their own retry strategy (see interaction 2)

### GitHub API Secondary Rate Limits

GitHub may return 403 with a `Retry-After` header for secondary rate limits
(triggered by too many concurrent requests or too many requests in a short
window). The CLI and dashboard treat this identically to a 429 response.

---

## CLI API Interactions

### 1. Read quarantine.json from branch

Read the current quarantine state at the start of every `quarantine run`.

| Field | Value |
|---|---|
| **Caller** | CLI |
| **When** | Start of `quarantine run`, before executing tests |
| **Endpoint** | `GET /repos/{owner}/{repo}/contents/{path}` |
| **URL pattern** | `GET /repos/{owner}/{repo}/contents/quarantine.json?ref=quarantine/state` |
| **HTTP method** | GET |
| **Required scope** | `repo` (PAT) or `contents: read` (GITHUB_TOKEN) |

**Request parameters:**

| Parameter | Location | Value |
|---|---|---|
| `owner` | path | From `quarantine.yml` or auto-detected from git remote |
| `repo` | path | From `quarantine.yml` or auto-detected from git remote |
| `path` | path | `quarantine.json` |
| `ref` | query | `quarantine/state` |

**Request headers:**

```
Authorization: Bearer {token}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
```

**Success response (200):**

```json
{
  "type": "file",
  "encoding": "base64",
  "size": 1234,
  "name": "quarantine.json",
  "path": "quarantine.json",
  "content": "eyJ2ZXJzaW9uIjoxLC4uLn0=",
  "sha": "abc123def456...",
  ...
}
```

Handling:
- Base64-decode `content` field.
- Parse as JSON into the quarantine state struct.
- Store the `sha` value for use in subsequent write (interaction 2).
- Validate `version` field. If unsupported version, log warning and proceed with best-effort parsing.

**Error handling:**

| Status | Meaning | Behavior |
|---|---|---|
| 404 | Branch or file does not exist | If branch missing: CLI was not initialized. Print error: `"Quarantine is not initialized for this repository. Run 'quarantine init' first."` Exit 2. If branch exists but file missing: treat as empty quarantine state (no tests quarantined). |
| 401/403 | Auth failure | Fall back to Actions cache. If cache miss, enter degraded mode (run all tests, no exclusions). |
| 429/500/502/503/timeout | Transient | Retry once. If retry fails, fall back to Actions cache. If cache miss, enter degraded mode. |

**Actions cache fallback:**

When the Contents API read fails:
1. Attempt to read from GitHub Actions cache (key: `quarantine-state-latest`).
2. If cache hit: use cached quarantine.json. Log warning that state may be stale.
3. If cache miss: no quarantine state available. Run all tests with no exclusions. Flaky detection via retry still works.

**Degraded mode behavior:**

CLI runs all tests (no quarantine exclusions). Flaky detection via retry still
operates. Newly detected flaky tests cannot be written to quarantine.json but
are still reported in CLI output and results file.

---

### 2. Write quarantine.json to branch

Update quarantine state after detecting new flaky tests or removing unquarantined tests.

| Field | Value |
|---|---|
| **Caller** | CLI |
| **When** | After flaky detection and unquarantine checks, before PR comment |
| **Endpoint** | `PUT /repos/{owner}/{repo}/contents/{path}` |
| **URL pattern** | `PUT /repos/{owner}/{repo}/contents/quarantine.json` |
| **HTTP method** | PUT |
| **Required scope** | `repo` (PAT) or `contents: write` (GITHUB_TOKEN) |

**Request parameters:**

| Parameter | Location | Value |
|---|---|---|
| `owner` | path | From config or git remote |
| `repo` | path | From config or git remote |
| `path` | path | `quarantine.json` |

**Request body:**

```json
{
  "message": "quarantine: update state ({N} tests quarantined)",
  "content": "<base64-encoded quarantine.json>",
  "sha": "<sha from the GET in interaction 1>",
  "branch": "quarantine/state"
}
```

**Request headers:**

```
Authorization: Bearer {token}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
Content-Type: application/json
```

**Success response (200):**

```json
{
  "content": {
    "name": "quarantine.json",
    "path": "quarantine.json",
    "sha": "newsha789...",
    ...
  },
  "commit": {
    "sha": "commitsha...",
    "message": "quarantine: update state (5 tests quarantined)",
    ...
  }
}
```

Handling:
- Confirm write succeeded. The new `sha` can be logged for debugging.
- Update the Actions cache with the new state (key: `quarantine-state-latest`).

**Error handling:**

| Status | Meaning | Behavior |
|---|---|---|
| 409 Conflict | Another build wrote since our read | **CAS retry loop** (see below) |
| 404 | Branch deleted between read and write | Log error. Enter degraded mode. The branch should have been created by `quarantine init`. |
| 422 | Content exceeds 1 MB or invalid base64 | Log error with size diagnostic. Enter degraded mode. If file is approaching 1 MB, log warning suggesting cleanup of stale entries. |
| 401/403 | Auth failure | Enter degraded mode. State update is lost; next build will re-detect. |
| 429/500/502/503/timeout | Transient | Retry once (separate from CAS retries). If retry fails, enter degraded mode. |

**CAS retry loop (409 Conflict):**

Per ADR-012, when a 409 is returned:

1. Re-read quarantine.json from the branch (interaction 1, fresh GET).
2. Merge: take the union of quarantined tests from both versions. **Quarantine wins** -- if a test is quarantined in either version, it stays quarantined.
3. Write again with the new SHA.
4. Repeat up to 3 times total (3 retries after the initial attempt = 4 total write attempts).
5. If all retries fail: log warning, enter degraded mode. The state update is lost but will be re-detected on the next build.

**Merge semantics:**

- New quarantine entries: add to the merged result.
- Removed entries (unquarantined): only remove if absent from BOTH versions.
- Updated fields (e.g., `flaky_count`, `last_flaky_at`): take the higher/later value.

**Degraded mode behavior:**

State update is lost. The CLI still reports results via stdout, results file, and
PR comment. The next build will re-detect the flaky test and attempt the write
again.

---

### 3. Create quarantine/state branch

Create the dedicated branch during `quarantine init`. This is the ONLY code path
that creates the branch.

| Field | Value |
|---|---|
| **Caller** | CLI |
| **When** | During `quarantine init`, after validating token and repo access |
| **Endpoint** | `POST /repos/{owner}/{repo}/git/refs` |
| **URL pattern** | `POST /repos/{owner}/{repo}/git/refs` |
| **HTTP method** | POST |
| **Required scope** | `repo` (PAT) or `contents: write` (GITHUB_TOKEN) |

**Prerequisite:** Get the SHA of the repo's default branch HEAD to use as the
base commit.

**Step 1: Get default branch HEAD SHA**

```
GET /repos/{owner}/{repo}/git/ref/heads/{default_branch}
```

Response:

```json
{
  "ref": "refs/heads/main",
  "object": {
    "sha": "basesha123...",
    "type": "commit"
  }
}
```

**Step 2: Create the branch ref**

**Request body:**

```json
{
  "ref": "refs/heads/quarantine/state",
  "sha": "<basesha from step 1>"
}
```

**Request headers:**

```
Authorization: Bearer {token}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
Content-Type: application/json
```

**Success response (201):**

```json
{
  "ref": "refs/heads/quarantine/state",
  "object": {
    "sha": "basesha123...",
    "type": "commit"
  }
}
```

**Step 3: Create initial empty quarantine.json on the new branch**

Use interaction 2 (PUT Contents API) with `sha` omitted (new file creation)
to write an initial empty state:

```json
{
  "version": 1,
  "updated_at": "<current ISO 8601 timestamp>",
  "tests": {}
}
```

The PUT request for a new file omits the `sha` field from the request body.

**Error handling:**

| Status | Meaning | Behavior |
|---|---|---|
| 422 (with "Reference already exists") | Branch already exists | Not an error. Log info: "quarantine/state branch already exists." Verify quarantine.json exists on the branch. If missing, create it (step 3). |
| 404 | Repo not found or token cannot see repo | Print error: `"Repository {owner}/{repo} not found. Check your GitHub token permissions and repository name."` Exit 2. |
| 401 | Bad token | Print error: `"GitHub authentication failed. Check QUARANTINE_GITHUB_TOKEN."` Exit 2. |
| 403 | Insufficient permissions | Print error: `"GitHub token lacks permission to create branches. Required scope: repo."` Exit 2. |
| 422 (other) | Validation error | Print error with response body. Exit 2. |
| 429/500/502/503/timeout | Transient | Retry once. If retry fails, print error and exit 2. (`quarantine init` does NOT use degraded mode -- it must succeed.) |

**Degraded mode behavior:**

None. `quarantine init` is an explicit setup command. Failures are fatal (exit 2)
with diagnostic messages. The user must fix the issue and re-run.

---

### 4. Batch check for closed quarantine issues

Detect unquarantined tests by finding closed GitHub Issues with the quarantine
label. Uses the Search API for efficiency: one call returns all closed issues
instead of N individual lookups.

| Field | Value |
|---|---|
| **Caller** | CLI |
| **When** | After reading quarantine.json (interaction 1), before executing tests |
| **Endpoint** | `GET /search/issues` |
| **URL pattern** | `GET /search/issues?q={query}` |
| **HTTP method** | GET |
| **Required scope** | `repo` (PAT) or `issues: read` (GITHUB_TOKEN) |

**Request parameters:**

| Parameter | Location | Value |
|---|---|---|
| `q` | query | `repo:{owner}/{repo} is:issue is:closed label:quarantine` |
| `per_page` | query | `100` (max) |
| `page` | query | `1` (paginate if `total_count` > 100) |

**Request headers:**

```
Authorization: Bearer {token}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
```

**Success response (200):**

```json
{
  "total_count": 3,
  "incomplete_results": false,
  "items": [
    {
      "number": 42,
      "title": "[Quarantine] test_payment_processing",
      "state": "closed",
      "labels": [
        { "name": "quarantine" },
        { "name": "quarantine:abc123" }
      ],
      ...
    }
  ]
}
```

Handling:
- Extract issue numbers from the response.
- Compare against `issue_number` fields in quarantine.json entries.
- Any quarantined test whose `issue_number` appears in the closed issues list is
  marked for unquarantine.
- Remove those tests from quarantine.json (reflected in the subsequent write, interaction 2).
- If `incomplete_results` is `true`, log a warning -- results may be truncated by
  GitHub's search index lag.

**Pagination:**

If `total_count` > 100, paginate:
- Request `page=2`, `page=3`, etc.
- Stop when `items` array is empty or all pages retrieved.
- Each page counts as one API call against the Search API rate limit (30 req/min).

**Error handling:**

| Status | Meaning | Behavior |
|---|---|---|
| 401/403 | Auth failure | Enter degraded mode. Skip unquarantine check. All currently quarantined tests remain quarantined. |
| 422 | Malformed query | Log error. Skip unquarantine check. |
| 429 | Search rate limit (30 req/min) | Read `Retry-After`. Wait and retry once. If still limited, skip unquarantine check. |
| 500/502/503/timeout | Transient | Retry once. If retry fails, skip unquarantine check. |

**Rate limit note:**

The Search API has a separate rate limit: 30 requests per minute for authenticated
users. This is independent of the core API rate limit. The CLI makes at most 1
search call per run (plus pagination), so this is unlikely to be hit unless many
builds run in rapid succession.

**Degraded mode behavior:**

Unquarantine check is skipped. All currently quarantined tests remain quarantined
for this build. They will be unquarantined on the next build that can reach the
Search API. Impact: a test stays quarantined for one extra build cycle at most.

---

### 5. Search for existing GitHub issue (dedup check)

Before creating a new issue for a flaky test, check if one already exists to
prevent duplicates.

| Field | Value |
|---|---|
| **Caller** | CLI |
| **When** | After detecting a new flaky test, before creating an issue (interaction 6) |
| **Endpoint** | `GET /search/issues` |
| **URL pattern** | `GET /search/issues?q={query}` |
| **HTTP method** | GET |
| **Required scope** | `repo` (PAT) or `issues: read` (GITHUB_TOKEN) |

**Request parameters:**

| Parameter | Location | Value |
|---|---|---|
| `q` | query | `repo:{owner}/{repo} is:issue is:open label:quarantine label:quarantine:{test_hash}` |
| `per_page` | query | `1` (only need to know if any exist) |

The `test_hash` is a deterministic identifier derived from the `test_id`. This
label ensures uniqueness per test across issue searches.

**Request headers:**

```
Authorization: Bearer {token}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
```

**Success response (200):**

```json
{
  "total_count": 1,
  "items": [
    {
      "number": 42,
      "html_url": "https://github.com/org/repo/issues/42",
      ...
    }
  ]
}
```

Handling:
- If `total_count` > 0: issue already exists. Use the existing issue number and
  URL. Update the quarantine.json entry with this issue info. Do NOT create a new
  issue.
- If `total_count` == 0: no existing issue found. Proceed to create one (interaction 6).

**Secondary dedup via quarantine.json:**

The quarantine.json entry may already contain an `issue_number` and `issue_url`
from a previous build. If present, skip this search entirely -- the issue is
already known. This search is only needed when quarantining a test for the first
time (no `issue_number` in the entry).

**Error handling:**

| Status | Meaning | Behavior |
|---|---|---|
| 401/403 | Auth failure | Skip dedup check. Attempt to create the issue (interaction 6). Worst case: a duplicate issue is created. |
| 422 | Malformed query | Skip dedup check. Attempt to create the issue. |
| 429 | Search rate limit | Wait and retry once. If still limited, skip dedup check. |
| 500/502/503/timeout | Transient | Retry once. If retry fails, skip dedup check. |

**Race condition note (ADR-012):**

A small window exists between the search returning "no existing issue" and the
subsequent issue creation. Two concurrent builds may both pass the dedup check
and both create issues. This is accepted -- a human closes the duplicate in
seconds, and the quarantine.json entry is updated with whichever issue the next
build finds first.

**Degraded mode behavior:**

If the search fails, the CLI proceeds to create the issue. A duplicate may be
created. This is a cosmetic issue, not a correctness issue.

---

### 6. Create GitHub issue

Create a tracking issue for a newly detected flaky test.

| Field | Value |
|---|---|
| **Caller** | CLI |
| **When** | After dedup check (interaction 5) confirms no existing issue |
| **Endpoint** | `POST /repos/{owner}/{repo}/issues` |
| **URL pattern** | `POST /repos/{owner}/{repo}/issues` |
| **HTTP method** | POST |
| **Required scope** | `repo` (PAT) or `issues: write` (GITHUB_TOKEN) |

**Request body:**

```json
{
  "title": "[Quarantine] {test_name}",
  "body": "## Flaky Test Detected\n\n**Test ID:** `{test_id}`\n**Suite:** `{suite}`\n**Name:** `{name}`\n**First detected:** {timestamp}\n**Detected in:** {branch} @ {commit_sha}\n**PR:** #{pr_number}\n\n### Retry Results\n\n| Attempt | Result | Duration |\n|---|---|---|\n| 1 | failed | {duration}ms |\n| 2 | passed | {duration}ms |\n\n### Failure Message\n\n```\n{failure_message}\n```\n\n---\n\n*This issue was automatically created by [Quarantine](https://github.com/mycargus/quarantine). Close this issue to unquarantine the test.*",
  "labels": ["quarantine", "quarantine:{test_hash}"]
}
```

**Request headers:**

```
Authorization: Bearer {token}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
Content-Type: application/json
```

**Success response (201):**

```json
{
  "number": 43,
  "html_url": "https://github.com/org/repo/issues/43",
  "title": "[Quarantine] test_payment_processing",
  ...
}
```

Handling:
- Store `number` and `html_url` in the quarantine.json entry for the test.
- Log: `Created issue #43 for flaky test: test_payment_processing`

**Label auto-creation:**

GitHub automatically creates labels that do not exist when referenced in an issue
creation request. The `quarantine` and `quarantine:{test_hash}` labels will be
created on first use. No separate label creation API call is needed.

**Error handling:**

| Status | Meaning | Behavior |
|---|---|---|
| 410 Gone | Issues disabled on the repository | Log warning. Skip issue creation for this and all subsequent tests in this run. Enter degraded mode for issue management. |
| 422 | Validation error (e.g., title too long, labels issue) | Log error with response body. Skip issue creation. Enter degraded mode. |
| 401/403 | Auth failure | Log error. Skip issue creation. Enter degraded mode. |
| 429/500/502/503/timeout | Transient | Retry once. If retry fails, skip issue creation. Enter degraded mode. |

**Degraded mode behavior:**

Issue is not created. The quarantine.json entry is written without `issue_number`
and `issue_url`. The next build will detect the missing issue info and attempt
to create the issue again (after a dedup check).

---

### 7. Post PR comment

Post a summary comment on the pull request that triggered the build.

| Field | Value |
|---|---|
| **Caller** | CLI |
| **When** | After all state updates, as one of the last operations |
| **Endpoint** | `POST /repos/{owner}/{repo}/issues/{issue_number}/comments` |
| **URL pattern** | `POST /repos/{owner}/{repo}/issues/{pr_number}/comments` |
| **HTTP method** | POST |
| **Required scope** | `repo` (PAT) or `pull-requests: write` (GITHUB_TOKEN) |

**Note:** GitHub's API treats pull requests as issues. PR comments use the Issues
API endpoint with the PR number as the `issue_number`.

**PR number detection:**

1. If `--pr` flag is provided: use that value.
2. Else if `GITHUB_EVENT_PATH` env var is set (GitHub Actions): read the JSON file
   at that path and extract `.pull_request.number` (for `pull_request` events) or
   `.number` (for `pull_request_target` events).
3. If neither is available: skip PR comment. Log info message.

**Request body:**

```json
{
  "body": "<!-- quarantine-bot -->\n## Quarantine Summary\n\n**Build result:** pass (2 failures suppressed)\n\n| Test | Status | Issue |\n|---|---|---|\n| `test_payment_processing` | flaky (passed retry 2/3) | #42 |\n| `test_email_send` | flaky (passed retry 1/3) | #43 |\n\n*Quarantine v{version} -- [docs](https://github.com/mycargus/quarantine)*"
}
```

The `<!-- quarantine-bot -->` HTML comment marker at the start of the body is used
to identify Quarantine's comment for subsequent updates (interaction 8).

**Request headers:**

```
Authorization: Bearer {token}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
Content-Type: application/json
```

**Success response (201):**

```json
{
  "id": 123456789,
  "html_url": "https://github.com/org/repo/issues/99#issuecomment-123456789",
  "body": "<!-- quarantine-bot -->...",
  ...
}
```

Handling:
- Log: `Posted PR comment on #99`

**Important:** Before posting a new comment, always attempt to find and update an
existing Quarantine comment first (interaction 8). Only post a new comment if no
existing one is found.

**Error handling:**

| Status | Meaning | Behavior |
|---|---|---|
| 404 | PR not found (may have been closed/merged between detection and comment) | Log warning. Skip comment. |
| 401/403 | Auth failure | Log warning. Skip comment. |
| 422 | Body too large or validation error | Truncate body and retry once. If still fails, log warning. Skip comment. |
| 429/500/502/503/timeout | Transient | Retry once. If retry fails, log warning. Skip comment. |

**Degraded mode behavior:**

PR comment is not posted. This is a notification-only operation with no impact on
quarantine correctness. Results are still written to the results file and visible
in CI logs.

---

### 8. Update existing PR comment

Find and update a previously posted Quarantine comment on the same PR to prevent
notification noise.

This is a two-step operation: list comments to find the existing one, then update it.

| Field | Value |
|---|---|
| **Caller** | CLI |
| **When** | Before posting a new PR comment (interaction 7) |
| **Required scope** | `repo` (PAT) or `pull-requests: write` (GITHUB_TOKEN) |

**Step 1: List PR comments to find existing Quarantine comment**

| Field | Value |
|---|---|
| **Endpoint** | `GET /repos/{owner}/{repo}/issues/{pr_number}/comments` |
| **HTTP method** | GET |

**Request parameters:**

| Parameter | Location | Value |
|---|---|---|
| `per_page` | query | `100` |
| `page` | query | Paginate as needed |

**Request headers:**

```
Authorization: Bearer {token}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
```

**Success response (200):**

Array of comment objects. Scan for a comment whose `body` starts with
`<!-- quarantine-bot -->`.

```json
[
  {
    "id": 123456789,
    "body": "<!-- quarantine-bot -->\n## Quarantine Summary\n...",
    ...
  }
]
```

Handling:
- If found: proceed to step 2 (update).
- If not found after all pages: proceed to interaction 7 (create new comment).

**Step 2: Update the existing comment**

| Field | Value |
|---|---|
| **Endpoint** | `PATCH /repos/{owner}/{repo}/issues/comments/{comment_id}` |
| **HTTP method** | PATCH |

**Request body:**

```json
{
  "body": "<!-- quarantine-bot -->\n## Quarantine Summary\n\n**Build result:** pass (2 failures suppressed)\n**Updated:** {timestamp}\n\n| Test | Status | Issue |\n|---|---|---|\n| `test_payment_processing` | flaky (passed retry 2/3) | #42 |\n\n*Quarantine v{version} -- [docs](https://github.com/mycargus/quarantine)*"
}
```

**Request headers:**

```
Authorization: Bearer {token}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
Content-Type: application/json
```

**Success response (200):**

```json
{
  "id": 123456789,
  "body": "<!-- quarantine-bot -->...",
  ...
}
```

Handling:
- Log: `Updated PR comment on #99`

**Error handling (both steps):**

| Status | Meaning | Behavior |
|---|---|---|
| 404 | PR or comment not found | Fall through to creating a new comment (interaction 7). |
| 401/403 | Auth failure | Log warning. Skip comment update/creation. |
| 422 | Body too large | Truncate and retry. |
| 429/500/502/503/timeout | Transient | Retry once. If retry fails, fall through to creating a new comment. If creation also fails, skip. |

**Optimization:**

To reduce API calls, the CLI can limit the comment search to the most recent 100
comments. If the Quarantine comment is older than that, it creates a new one
instead. This caps the search at 1 API call in the common case.

**Degraded mode behavior:**

Same as interaction 7. PR comment is best-effort.

---

## Dashboard API Interactions

### 9. List artifacts for a repo

Discover new test result artifacts to ingest.

| Field | Value |
|---|---|
| **Caller** | Dashboard |
| **When** | Background polling (every 5 min per org, staggered) and on-demand when user views a project |
| **Endpoint** | `GET /repos/{owner}/{repo}/actions/artifacts` |
| **URL pattern** | `GET /repos/{owner}/{repo}/actions/artifacts` |
| **HTTP method** | GET |
| **Required scope** | `actions:read` (+ `repo` if private repository) |

**Request parameters:**

| Parameter | Location | Value |
|---|---|---|
| `owner` | path | From dashboard project configuration |
| `repo` | path | From dashboard project configuration |
| `per_page` | query | `100` (max) |
| `page` | query | Paginate as needed |
| `name` | query | `quarantine-result` (filter by artifact name prefix) |

**Request headers:**

```
Authorization: Bearer {token}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
If-None-Match: "{etag_from_last_poll}"
```

**Success response (200):**

```json
{
  "total_count": 45,
  "artifacts": [
    {
      "id": 12345,
      "name": "quarantine-result-abc123",
      "size_in_bytes": 2048,
      "archive_download_url": "https://api.github.com/repos/org/repo/actions/artifacts/12345/zip",
      "created_at": "2026-03-17T10:00:00Z",
      "expires_at": "2026-06-15T10:00:00Z",
      "workflow_run": {
        "id": 67890,
        "head_branch": "main",
        "head_sha": "abc123"
      },
      ...
    }
  ]
}
```

**304 Not Modified:**

If the `If-None-Match` header matches the current ETag, GitHub returns 304 with
no body. This does NOT count against the rate limit. The dashboard skips
processing for this repo.

Handling:
- Filter artifacts by `created_at` > last successful poll timestamp for this repo.
- For each new artifact: queue for download (interaction 10).
- Store the response `ETag` header for the next poll.
- Update `last_synced` timestamp in the projects table.

**Pagination:**

Paginate through all results. In practice, filtering by `name` prefix and only
processing artifacts newer than `last_synced` limits the number of pages.

**Error handling:**

| Status | Meaning | Behavior |
|---|---|---|
| 404 | Repo not found or not accessible | Log error. Mark project as inaccessible. Skip until manually re-enabled. |
| 401/403 | Auth failure | Log error. Skip this repo for this poll cycle. Circuit breaker increments. |
| 429 | Rate limit | Read `Retry-After`. Skip this repo. Resume on next cycle. |
| 500/502/503/timeout | Transient | Retry once. If retry fails, skip this repo. Circuit breaker increments. |

**Circuit breaker (ADR-015):**

3 consecutive failures for a repo triggers a 30-minute pause. After the pause,
the circuit breaker resets and normal polling resumes.

**Debouncing:**

On-demand pulls (triggered by a user viewing a project page) are debounced to
max 1 pull per repo per 5 minutes. If data was fetched within the last 5 minutes,
return the cached result immediately.

---

### 10. Download artifact

Download a specific test result artifact for ingestion into SQLite.

| Field | Value |
|---|---|
| **Caller** | Dashboard |
| **When** | After discovering new artifacts (interaction 9) |
| **Endpoint** | `GET /repos/{owner}/{repo}/actions/artifacts/{artifact_id}/zip` |
| **URL pattern** | `GET /repos/{owner}/{repo}/actions/artifacts/{artifact_id}/zip` |
| **HTTP method** | GET |
| **Required scope** | `actions:read` (+ `repo` if private repository) |

**Request headers:**

```
Authorization: Bearer {token}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
```

**Success response (302 redirect):**

GitHub returns a 302 redirect to a short-lived (1 minute) pre-signed URL for the
artifact zip file. The HTTP client must follow the redirect.

The redirect URL is on `*.blob.core.windows.net` (Azure Blob Storage). The
redirect response includes a `Location` header with the pre-signed URL.

Handling:
1. Follow the 302 redirect (most HTTP clients do this automatically).
2. Download the zip file from the redirect URL.
3. Extract the zip archive in memory.
4. Parse the JSON file(s) inside.
5. Validate against the test-result schema.
6. Upsert into SQLite (keyed by `run_id` for idempotency).

**Error handling:**

| Status | Meaning | Behavior |
|---|---|---|
| 404 | Artifact expired (past 90-day retention) or deleted | Log warning. Skip this artifact. Remove from processing queue. |
| 410 Gone | Artifact explicitly deleted | Log warning. Skip this artifact. |
| 401/403 | Auth failure | Log error. Skip this artifact. Circuit breaker increments. |
| 429 | Rate limit | Read `Retry-After`. Re-queue artifact for next poll cycle. |
| 500/502/503/timeout | Transient (either GitHub API or blob storage) | Retry once. If retry fails, re-queue for next poll cycle. |

**Zip extraction errors:**

| Error | Behavior |
|---|---|
| Corrupt zip | Log warning. Skip artifact. |
| No JSON file in zip | Log warning. Skip artifact. |
| Malformed JSON | Log warning with parse error. Skip artifact. |
| JSON does not match schema | Log warning with validation errors. Skip artifact. |

**Degraded mode behavior:**

Failed downloads are re-queued for the next poll cycle. Successful downloads from
other artifacts proceed normally. The dashboard serves whatever data it has
successfully ingested.

---

## NOT in v1

The following GitHub API interactions are explicitly excluded from v1:

| Interaction | Reason | Version |
|---|---|---|
| `POST /repos/{owner}/{repo}/actions/artifacts` (artifact upload via REST) | CLI writes results to disk. Workflow uses `actions/upload-artifact` action. | v2+ (for non-GHA CI) |
| Actions cache read/write via REST API | Handled by `actions/cache` action within the workflow, not by the CLI directly. | N/A (action, not API) |
| `POST /app/installations/{id}/access_tokens` (GitHub App token exchange) | v1 uses PATs only. | v2+ |
| `GET /user` (OAuth user info) | Dashboard has no auth in v1. | v2+ |
| `GET /orgs/{org}/repos` (list org repositories) | Dashboard uses explicit project configuration in v1. | v2+ (auto-discovery) |
| `POST /repos/{owner}/{repo}/hooks` (webhook creation) | v1 uses polling for issue status. | v2+ (real-time unquarantine) |
| `GET /repos/{owner}/{repo}/pulls/{pr}/files` (PR file list) | No test-impact analysis in v1. | v3+ |

---

## Rate Limit Budget Analysis

### CLI: API calls per `quarantine run`

| Operation | Calls | Notes |
|---|---|---|
| Read quarantine.json | 1 | Always |
| Batch check closed issues | 1-2 | 1 call usually, paginate if >100 closed issues |
| Write quarantine.json | 1-4 | 1 call, up to 3 CAS retries (each retry = 1 read + 1 write) |
| Search for existing issue (dedup) | 0-N | 1 per newly detected flaky test. 0 if no new flaky tests. |
| Create GitHub issue | 0-N | 1 per newly detected flaky test without existing issue |
| List PR comments | 0-1 | 0 if no PR, 1 if PR |
| Post/update PR comment | 0-1 | 0 if no PR, 1 if PR |
| **Typical total** | **3-6** | No new flaky tests detected |
| **Worst case (10 new flaky tests)** | **~28** | 1 read + 2 closed check + 4 write (3 retries) + 10 dedup + 10 create + 1 list comments + 1 comment |

At 1,000 req/hr (`GITHUB_TOKEN`), a repo can support ~35 concurrent builds in
the worst case, or ~166 in the typical case, per hour. This is adequate for most
repositories.

### Dashboard: API calls per poll cycle

| Operation | Calls per repo | Notes |
|---|---|---|
| List artifacts | 1 | 304 (no change) = free. Paginate adds calls. |
| Download artifact | 0-N | 1 per new artifact since last poll |
| **Typical total** | **1-5** | 1 list + few new artifacts |

With 5,000 req/hr (PAT) and 5-minute poll intervals, the dashboard can monitor:
- ~400 repos (assuming 1 call/repo/cycle = 12 calls/repo/hr)
- ~80 repos (assuming 5 calls/repo/cycle = 60 calls/repo/hr)

This is well within budget for v1's internal deployment. For larger orgs, a
GitHub App (12,500 req/hr) provides more headroom.

---

## Open Questions

1. **Rate limit header logging:** Should the CLI always log `X-RateLimit-Remaining`
   at `--verbose` level, or only when approaching the threshold? Recommendation:
   always log at verbose, warn at <10%.

2. **Actions cache API vs action:** The current design uses the `actions/cache`
   action for cache fallback. If the CLI needs to read the cache outside of a
   GitHub Actions workflow (e.g., local development), the Cache API
   (`GET /repos/{owner}/{repo}/actions/caches`) would be needed. Defer to v2.

3. **Artifact name collision in matrix jobs:** When a workflow uses matrix
   strategy, multiple jobs may produce artifacts with the same name prefix. The
   artifact naming convention should include a matrix-safe suffix (e.g.,
   `quarantine-result-{run_id}-{matrix_key}`). Document in setup guide.
