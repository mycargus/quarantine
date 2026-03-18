# CLI Flags & Configuration

### Scenario 43: User overrides framework in quarantine.yml [M2]

**Given** the project contains both `jest.config.js` and `vitest.config.ts`
files, and `quarantine.yml` has `framework: vitest`

**When** the developer runs
`quarantine run --retries 3 -- vitest run --reporter=junit`

**Then** the CLI uses `vitest` as the framework (per the config), parses
Vitest-formatted JUnit XML output, and uses Vitest-specific rerun commands for
retries. The `framework` field in config is the source of truth — there is no
auto-detection (ADR-010, amended).

---

### Scenario 44: User customizes retry count [M3]

**Given** `quarantine.yml` has `retries: 5`

**When** the developer runs `quarantine run -- jest --ci ...` (no `--retries`
flag)

**Then** the CLI reads `retries: 5` from config and retries each failing test up
to 5 times. Config resolution order: CLI flags > config values > auto-detected >
defaults (per docs/cli-spec.md).

**When** the developer runs `quarantine run --retries 2 -- jest --ci ...`

**Then** the CLI flag overrides the config: retries only 2 times.

---

### Scenario 45: --dry-run flag [M4]

**Given** the CLI is configured in CI and would normally detect a flaky test,
update `quarantine.json`, and create an issue

**When** the developer runs
`quarantine run --dry-run --retries 3 -- jest --ci ...`

**Then** the CLI runs tests, parses XML, detects the flaky test, but does NOT:
- Write to `quarantine.json`
- Create GitHub Issues
- Post PR comments

Instead, it prints a summary of what would have been done:
```
[quarantine] DRY RUN — no changes written.
  Would quarantine: PaymentService > should handle charge timeout
  Would create issue: [Quarantine] PaymentService > should handle charge timeout
  Would post PR comment on PR #42
```
Writes results to `.quarantine/results.json` (results are always written).
Exits with code 0.

---

### Scenario 46: --exclude patterns [M4]

**Given** `quarantine.yml` has:
```yaml
exclude:
  - "test/integration/**"
```
and the developer also passes `--exclude "**::SlowServiceTest::*"`

**When** a test matching `test/integration/api_test.js::ApiTest::should connect`
fails on the first run

**Then** the CLI merges exclude patterns from config and CLI flags. The test
matches the config pattern `test/integration/**` (matched against `test_id`
using glob syntax). Quarantine ignores it entirely — no retry, no quarantine,
no issue creation. It behaves as if quarantine is not installed for that test.
The test's failure still affects the exit code normally (exit 1 if it fails).

---

### Scenario 47: --pr flag override and auto-detection [M5]

**Given** the CLI is running in GitHub Actions on a PR build

**When** the developer runs `quarantine run --pr 99 -- jest --ci ...`

**Then** the CLI uses PR number 99 (from the `--pr` flag) instead of
auto-detecting from `GITHUB_EVENT_PATH`. Posts the quarantine comment on PR #99.

**When** the developer runs `quarantine run -- jest --ci ...` (no `--pr` flag)
and `GITHUB_EVENT_PATH` is set to a JSON file containing
`{"pull_request": {"number": 42}}`

**Then** the CLI auto-detects PR number 42 from the event JSON and posts the
comment there.

**When** the developer runs `quarantine run -- jest --ci ...` on a non-PR build
(e.g., push to main) with no `--pr` flag and `GITHUB_EVENT_PATH` does not
contain a `pull_request` field

**Then** the CLI skips posting a PR comment. No error — PR comments are
best-effort.

---

### Scenario 48: PR comment suppressed via config [M5]

**Given** `quarantine.yml` has:
```yaml
notifications:
  github_pr_comment: false
```

**When** the CLI detects a flaky test during a PR build

**Then** the CLI does NOT post or update a PR comment, even though a PR number
is available. All other behavior (quarantine state update, issue creation,
results file) proceeds normally.

---

### Scenario 49: PR comment updated on second run [M5]

**Given** the CLI previously posted a PR comment on PR #42 containing the
`<!-- quarantine-bot -->` HTML marker

**When** a second CI run on the same PR detects additional flaky tests

**Then** the CLI:
1. Lists comments on PR #42
   (`GET /repos/{owner}/{repo}/issues/42/comments?per_page=100`).
2. Scans for the `<!-- quarantine-bot -->` marker.
3. Finds the existing comment and updates it via
   `PATCH /repos/{owner}/{repo}/issues/comments/{comment_id}` with the new
   summary (replacing the old content entirely).
4. No duplicate comments are created.

---

### Scenario 50: Custom rerun_command template [M3]

**Given** `quarantine.yml` has:
```yaml
framework: jest
rerun_command: "npx jest --testNamePattern '{name}' --config jest.ci.config.js"
```

**When** the CLI detects a failing test `should handle timeout` and needs to
retry it

**Then** the CLI uses the custom `rerun_command` template instead of the
auto-detected Jest rerun command. It substitutes `{name}` with the test name:
`npx jest --testNamePattern 'should handle timeout' --config jest.ci.config.js`.
The `{classname}` and `{file}` placeholders are also available for templates
that need them.

---

### Scenario 51: --verbose and --quiet flags [M2]

**Given** the CLI is configured in CI

**When** the developer runs `quarantine run --verbose -- jest --ci ...`

**Then** the CLI outputs detailed information: API calls made, retry attempts
with outcomes, config resolution trace, quarantine state details, and timing
information. Test runner output is passed through unmodified.

**When** the developer runs `quarantine run --quiet -- jest --ci ...`

**Then** the CLI outputs only warnings and errors. Test runner output is passed
through unmodified. No `[quarantine] Reading quarantine state... OK` lines.

**When** the developer runs `quarantine run --verbose --quiet -- jest --ci ...`

**Then** the CLI prints an error:
`Error: --verbose and --quiet are mutually exclusive.`
Exits with code 2.

---
