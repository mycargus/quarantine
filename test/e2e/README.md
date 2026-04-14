# E2E Tests

End-to-end tests that **observe real output** from the quarantine fixture
repo's CI pipeline. Written in JavaScript with Vitest as the runner and
[`riteway`](https://github.com/paralleldrive/riteway) assertions.

## Philosophy

E2E tests observe — they do not arrange. The fixture repo
(`mycargus/quarantine-test-fixture`) runs `quarantine run jest-tests` daily
with deliberately flaky Jest tests. It produces real artifacts, real GitHub
Issues, real quarantine state, and a real PR comment. The E2E tests in this
directory read that output and verify it matches expectations.

**E2E tests must never:**
- Run the quarantine CLI binary
- Create fake test runners or hand-crafted JUnit XML
- Pre-populate quarantine state on the state branch
- Create or close GitHub Issues as test setup

If a test needs to run the binary with controlled inputs, it belongs in the
Interface test layer (`cli/` with mock HTTP servers), not here.

## What the fixture repo produces

Each daily CI run produces:

| Output | Location | What E2E tests verify |
|--------|----------|-----------------------|
| Quarantine state | `.quarantine/jest-tests/state.json` on `quarantine/state` branch | Known flaky tests quarantined, stable tests excluded |
| GitHub Issues | Open issues with `quarantine` label | Title format, dedup labels (`quarantine:jest-tests:<hash>`), recent creation dates |
| Closed issues | Closed by the daily cleanup step | Search API shape (`total_count`, `items[].number`) |
| Artifact | `quarantine-results-jest-tests-<run_id>` | Valid JSON, `suite_name`, stable tests passed |
| PR comment | Comment on the `e2e-pr-proxy` issue | `<!-- quarantine:jest-tests -->` marker, mentions flaky tests |

## Requirements

- Node.js >= 20
- Three environment variables (see below)

The quarantine CLI binary is **not** required — E2E tests don't run it.

## Environment Variables

| Variable | Description |
|---|---|
| `QUARANTINE_GITHUB_TOKEN` | PAT with repo read/write access |
| `QUARANTINE_TEST_OWNER` | GitHub org or username that owns the fixture repo |
| `QUARANTINE_TEST_REPO` | Name of the fixture repo (e.g. `quarantine-test-fixture`) |

Tests fail immediately when required env vars are missing — they never skip.

For local development, copy `.env.example` to `.env` and fill in your values.
The vitest config loads `.env` automatically (CI env vars take precedence).

## Running

```bash
cd test
pnpm install
pnpm run test:e2e
```

Or from the repo root: `make e2e-test`

## Test files

```
test/e2e/
  quarantine-observe.test.js     # Observes fixture CI output (state, issues, artifacts, PR comment)
  github-artifacts.test.js       # Artifact list API shape (ETag, response fields)
  github-branch-not-found.test.js  # Contents API 404 error message fidelity
  dashboard-sync.test.js         # Dashboard HTTP server syncs real artifacts
```

## Adding new E2E tests

New E2E tests must follow the observe pattern:

1. Identify what the fixture CI already produces that you want to verify
2. If the fixture CI doesn't produce it, update the fixture repo's workflow
   to exercise the new behavior (e.g., add `--pr` flag to exercise PR comments)
3. Write a test that reads the output via GitHub API and asserts on it
4. Never import `child_process` or run the quarantine binary — that's an
   Interface test, not E2E
