---
name: create-e2e-test
description: Create an E2E test in test/e2e/ that observes real output from the fixture repo's CI pipeline. E2E tests read GitHub state, issues, artifacts, and comments — they never run the CLI binary or create fake test runners.
argument-hint: "<what fixture CI output to observe>"
model: sonnet
effort: medium
context: fork
disable-model-invocation: false
user-invocable: true
allowed-tools: Read, Grep, Glob, Write, Edit, Bash, Agent
---

Create an E2E test that observes real fixture CI output.

## Core principle

**E2E tests observe — they do not arrange.** The fixture repo
(`mycargus/quarantine-test-fixture`) runs `quarantine run jest-tests` daily
with deliberately flaky tests. It produces real artifacts, real GitHub Issues,
real quarantine state, and a real PR comment. E2E tests read that output and
verify it matches expectations.

## What E2E tests must NEVER do

- Run the quarantine CLI binary (`spawnSync`, `execSync`, `exec.Command`)
- Create fake test runners or shell scripts (`makeScript`, `writeFileSync` with `#!/bin/sh`)
- Write hand-crafted JUnit XML
- Pre-populate quarantine state on the state branch
- Create or close GitHub Issues as test setup
- Use `beforeAll` to skip — credentials must be checked at module top level

If a test needs controlled inputs, it belongs in the Interface layer (CLI
with mock HTTP servers), not in `test/e2e/`.

## Step 1 — Identify what to observe

What output does the fixture CI already produce that you want to verify?

| Output | Location | API |
|--------|----------|-----|
| Quarantine state | `.quarantine/jest-tests/state.json` on `quarantine/state` branch | Contents API |
| GitHub Issues | Open issues with `quarantine` label | Issues API |
| Closed issues | Closed by daily cleanup step | Search API |
| Artifact | `quarantine-results-jest-tests-<run_id>` | Artifacts API |
| PR comment | Comment on `e2e-pr-proxy` issue | Comments API |

If the fixture CI doesn't produce what you need, **update the fixture repo's
workflow first** — don't puppet from the E2E test.

## Step 2 — Check for existing coverage

```bash
grep -r "your-api-endpoint-or-feature" test/e2e/
```

Read `test/e2e/quarantine-observe.test.js` — it already covers state, issues,
Search API shape, artifact contents, and PR comments. Don't duplicate.

## Step 3 — Write the test

### Location and framework

- File: `test/e2e/<descriptive-name>.test.js`
- Framework: `vitest` + `riteway/vitest`

### Credential guard (top-level, not beforeAll)

```js
const token = process.env.QUARANTINE_GITHUB_TOKEN
const owner = process.env.QUARANTINE_TEST_OWNER
const repo = process.env.QUARANTINE_TEST_REPO

if (!token) throw new Error("QUARANTINE_GITHUB_TOKEN is required")
if (!owner) throw new Error("QUARANTINE_TEST_OWNER is required")
if (!repo) throw new Error("QUARANTINE_TEST_REPO is required")
```

### API request helper

```js
async function ghRequest(method, path) {
  return fetch(`https://api.github.com/repos/${owner}/${repo}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/vnd.github+json",
      "X-GitHub-Api-Version": "2022-11-28",
    },
  })
}
```

### Test timeout (Vitest 4 syntax)

```js
test("description", { timeout: 120_000 }, async () => { ... })
```

### Assertion style

```js
assert({
  given: "the quarantine state file from the fixture CI",
  should: "contain at least one quarantined test",
  actual: Object.keys(state.tests).length > 0,
  expected: true,
})
```

### Artifact download pattern

```js
import AdmZip from "adm-zip"

// List artifacts, find one matching the prefix
const { artifacts } = await (await ghRequest("GET", "/actions/artifacts?per_page=100")).json()
const artifact = artifacts.find(a => a.name.startsWith("quarantine-results-jest-tests-"))

// Download the ZIP (follows 302 redirect automatically)
const zipRes = await fetch(artifact.archive_download_url, {
  headers: { Authorization: `Bearer ${token}` },
  redirect: "follow",
})
const zip = new AdmZip(Buffer.from(await zipRes.arrayBuffer()))
const results = JSON.parse(zip.readAsText(zip.getEntry("results.json")))
```

## Step 4 — Run and verify

```bash
cd test && pnpm run test:e2e
```

Tests require real credentials and a fixture repo with recent CI output.
They run sequentially (`fileParallelism: false`).

## Step 5 — Lint

```bash
cd test && pnpm run lint:fix -- ./e2e/your-new-test.test.js
```

## Anti-patterns to reject

| Anti-pattern | Why it's wrong | What to do instead |
|---|---|---|
| `spawnSync(binPath, ["run", ...])` | Runs the CLI — that's an Interface test | Observe the fixture CI's output via GitHub API |
| `makeScript(dir, "fake-jest", "exit 0")` | Fake test runner — not the full system | The fixture repo runs real Jest with real flaky tests |
| `writeFileSync(xmlPath, "<testsuites>...")` | Hand-crafted XML — not real test output | The fixture CI produces real JUnit XML |
| `writeQuarantineState({...})` | Pre-arranging state — not observing | Read the state the fixture CI wrote |
| `createIssueWithLabel(...)` | Creating issues as setup — not observing | Read the issues the fixture CI created |
| `beforeAll(() => { if (!token) throw ... })` | Tests show as "skipped" not "failed" | Throw at module top level |
| `test.skip(...)` or `describe.skipIf(...)` | Silently hides missing coverage | Hard-fail with a descriptive error |
