/**
 * E2E test: Scenario 25 — Quarantined test's GitHub issue is closed (unquarantine)
 *
 * Verifies that the Search API query format (GET /search/issues with is:closed
 * label filter) works as the production code assumes. If the query format drifts
 * or the response shape changes, closed issues stop being detected and tests stay
 * quarantined forever.
 *
 * High-risk API interactions under test:
 *   - GET /search/issues?q=repo:owner/repo+is:issue+is:closed+label:quarantine
 *   - Response shape: { total_count: number, items: [{ number: number }, ...] }
 *
 * Required env vars:
 *   QUARANTINE_GITHUB_TOKEN  — PAT with repo scope
 *   QUARANTINE_TEST_OWNER    — GitHub org or user (e.g. "mycargus")
 *   QUARANTINE_TEST_REPO     — repository name (e.g. "quarantine-test-fixture")
 *
 * Optional:
 *   QUARANTINE_BIN           — path to quarantine binary (default: ../../bin/quarantine)
 */

import { execSync, spawnSync } from "node:child_process"
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { assert } from "riteway/vitest"
import { afterEach, beforeAll, beforeEach, describe, onTestFailed, test } from "vitest"

const BRANCH = "quarantine/state"
const STATE_FILE = "quarantine.json"

const token = process.env.QUARANTINE_GITHUB_TOKEN
const owner = process.env.QUARANTINE_TEST_OWNER
const repo = process.env.QUARANTINE_TEST_REPO
const binPath =
  process.env.QUARANTINE_BIN ?? new URL("../../bin/quarantine", import.meta.url).pathname

// --- GitHub API helpers ---

async function ghRequest(method, path, body) {
  return fetch(`https://api.github.com/repos/${owner}/${repo}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/vnd.github+json",
      "X-GitHub-Api-Version": "2022-11-28",
      "Content-Type": "application/json",
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
}

async function branchExists() {
  const res = await ghRequest("GET", `/git/ref/heads/${BRANCH}`)
  return res.status === 200
}

async function getFileOnBranch(filePath) {
  const res = await ghRequest("GET", `/contents/${filePath}?ref=${BRANCH}`)
  if (res.status === 404) return null
  if (res.status !== 200) {
    const text = await res.text()
    throw new Error(`getFileOnBranch(${filePath}): unexpected ${res.status}: ${text}`)
  }
  const data = await res.json()
  return {
    content: Buffer.from(data.content.replace(/\n/g, ""), "base64").toString("utf8"),
    sha: data.sha,
  }
}

async function writeFileOnBranch(filePath, content, sha, message) {
  const res = await ghRequest("PUT", `/contents/${filePath}`, {
    message: message ?? `test: update ${filePath}`,
    content: Buffer.from(content).toString("base64"),
    branch: BRANCH,
    ...(sha ? { sha } : {}),
  })
  if (res.status !== 200 && res.status !== 201) {
    const text = await res.text()
    throw new Error(`writeFileOnBranch(${filePath}): unexpected ${res.status}: ${text}`)
  }
  return (await res.json()).content.sha
}

// Track the last-known SHA so we can skip a read when writing.
let lastKnownStateSHA = null

async function resetQuarantineState() {
  let sha = lastKnownStateSHA
  if (sha == null) {
    const file = await getFileOnBranch(STATE_FILE)
    sha = file?.sha ?? null
  }
  const emptyState = JSON.stringify(
    { version: 1, updated_at: new Date().toISOString(), tests: {} },
    null,
    2,
  )
  try {
    lastKnownStateSHA = await writeFileOnBranch(
      STATE_FILE,
      emptyState,
      sha,
      "test: reset quarantine state",
    )
  } catch (err) {
    if (err.message.includes("409")) {
      const file = await getFileOnBranch(STATE_FILE)
      lastKnownStateSHA = await writeFileOnBranch(
        STATE_FILE,
        emptyState,
        file?.sha ?? null,
        "test: reset quarantine state (retry)",
      )
    } else {
      throw err
    }
  }
}

async function writeQuarantineState(state) {
  let sha = lastKnownStateSHA
  if (sha == null) {
    const file = await getFileOnBranch(STATE_FILE)
    sha = file?.sha ?? null
  }
  try {
    lastKnownStateSHA = await writeFileOnBranch(
      STATE_FILE,
      JSON.stringify(state, null, 2),
      sha,
      "test: write quarantine state",
    )
  } catch (err) {
    if (err.message.includes("409")) {
      const file = await getFileOnBranch(STATE_FILE)
      lastKnownStateSHA = await writeFileOnBranch(
        STATE_FILE,
        JSON.stringify(state, null, 2),
        file?.sha ?? null,
        "test: write quarantine state (retry)",
      )
    } else {
      throw err
    }
  }
}

async function createBranchWithEmptyState() {
  const repoRes = await ghRequest("GET", "")
  if (repoRes.status !== 200) {
    const text = await repoRes.text()
    throw new Error(`createBranchWithEmptyState: GET repo failed ${repoRes.status}: ${text}`)
  }
  const { default_branch: defaultBranch } = await repoRes.json()

  const refRes = await ghRequest("GET", `/git/ref/heads/${defaultBranch}`)
  if (refRes.status !== 200) {
    const text = await refRes.text()
    throw new Error(`createBranchWithEmptyState: GET ref failed ${refRes.status}: ${text}`)
  }
  const {
    object: { sha },
  } = await refRes.json()

  const createRes = await ghRequest("POST", "/git/refs", {
    ref: `refs/heads/${BRANCH}`,
    sha,
  })
  if (createRes.status !== 201) {
    const text = await createRes.text()
    throw new Error(`createBranchWithEmptyState: create ref failed ${createRes.status}: ${text}`)
  }

  const emptyState = JSON.stringify(
    { version: 1, updated_at: new Date().toISOString(), tests: {} },
    null,
    2,
  )
  await writeFileOnBranch(STATE_FILE, emptyState, null, "chore: initialize quarantine state")
}

// --- Label helpers ---

async function ensureQuarantineLabelExists() {
  const res = await ghRequest("GET", "/labels/quarantine")
  if (res.status === 200) return
  if (res.status === 404) {
    const createRes = await ghRequest("POST", "/labels", {
      name: "quarantine",
      color: "e11d48",
      description: "Flaky test quarantine",
    })
    if (createRes.status !== 201) {
      const text = await createRes.text()
      throw new Error(
        `ensureQuarantineLabelExists: create label failed ${createRes.status}: ${text}`,
      )
    }
  } else {
    const text = await res.text()
    throw new Error(`ensureQuarantineLabelExists: unexpected ${res.status}: ${text}`)
  }
}

// --- Issue helpers ---

async function createIssueWithLabel(title, label) {
  const res = await ghRequest("POST", "/issues", {
    title,
    body: "Quarantine issue created by E2E test.",
    labels: [label],
  })
  if (res.status !== 201) {
    const text = await res.text()
    throw new Error(`createIssueWithLabel: unexpected ${res.status}: ${text}`)
  }
  const data = await res.json()
  return data.number
}

async function closeIssue(issueNumber) {
  if (!issueNumber) return
  const res = await ghRequest("PATCH", `/issues/${issueNumber}`, { state: "closed" })
  if (res.status !== 200) {
    const text = await res.text()
    throw new Error(`closeIssue(${issueNumber}): unexpected ${res.status}: ${text}`)
  }
}

// --- Local setup helpers ---

function createWorkDir() {
  const dir = mkdtempSync(join(tmpdir(), "quarantine-e2e-unquarantine-"))
  execSync("git init", { cwd: dir, stdio: "pipe" })
  execSync('git config user.email "test@example.com"', { cwd: dir, stdio: "pipe" })
  execSync('git config user.name "Test"', { cwd: dir, stdio: "pipe" })
  execSync(`git remote add origin https://github.com/${owner}/${repo}.git`, {
    cwd: dir,
    stdio: "pipe",
  })
  return dir
}

function writeConfig(dir, content) {
  writeFileSync(join(dir, "quarantine.yml"), content, "utf8")
}

function makeScript(dir, name, body) {
  const p = join(dir, name)
  writeFileSync(p, `#!/bin/sh\n${body}\n`, { mode: 0o755 })
  return p
}

function runCLI(dir, args, extraEnv = {}) {
  const result = spawnSync(binPath, args, {
    cwd: dir,
    encoding: "utf8",
    env: { ...process.env, QUARANTINE_GITHUB_TOKEN: token, ...extraEnv },
    timeout: 120_000,
  })
  // Register a callback to surface CLI output if this test fails.
  onTestFailed(() => {
    console.error("\n--- quarantine CLI output (on failure) ---")
    console.error("args:", args.join(" "))
    if (result.stdout) console.error(`stdout:\n${result.stdout.trimEnd()}`)
    if (result.stderr) console.error(`stderr:\n${result.stderr.trimEnd()}`)
    if (result.error) console.error("spawn error:", result.error.message)
    console.error("exit code:", result.status)
    console.error("------------------------------------------\n")
  })
  return result
}

// ---

if (!token) throw new Error("QUARANTINE_GITHUB_TOKEN is required")
if (!owner) throw new Error("QUARANTINE_TEST_OWNER is required")
if (!repo) throw new Error("QUARANTINE_TEST_REPO is required")

// Close any issues left open by previous test runs before starting.
// Issues with the 'quarantine' label belong to the fixture repo's own CI
// workflow (quarantine-test-fixture) and must never be closed here.
async function closeStaleE2EIssues() {
  const res = await ghRequest("GET", "/issues?state=open&per_page=100")
  if (res.status !== 200) return
  const issues = await res.json()
  const stale = issues.filter((issue) => !issue.labels.some((label) => label.name === "quarantine"))
  await Promise.all(
    stale.map((issue) =>
      ghRequest("PATCH", `/issues/${issue.number}`, { state: "closed" }).catch(() => {}),
    ),
  )
}

beforeAll(async () => {
  await closeStaleE2EIssues()
})

// =========================================================================
// Scenario 25: Quarantined test's GitHub issue is closed (unquarantine)
//
// High-risk API interaction: GET /search/issues?q=repo:owner/repo+is:issue+is:closed+label:quarantine
// Response shape: { total_count: number, items: [{ number: number }, ...] }
//
// The CLI calls SearchClosedIssues during quarantine run. If a quarantined
// test's issue is closed, the test is removed from quarantine state and runs
// normally again. If the Search API query format drifts or the response shape
// changes, closed issues stop being detected and tests stay quarantined forever.
// =========================================================================

describe("quarantine run — Scenario 25: unquarantine via closed issue", () => {
  let dir
  let quarantineIssueNumber = null

  beforeAll(async () => {
    if (!(await branchExists())) {
      await createBranchWithEmptyState()
    }
    await ensureQuarantineLabelExists()
  })

  beforeEach(() => {
    dir = createWorkDir()
  })

  afterEach(async () => {
    // Close the quarantine issue if it is still open (cleanup).
    if (quarantineIssueNumber) {
      await closeIssue(quarantineIssueNumber).catch(() => {
        // Ignore errors — the issue may already be closed by the test.
      })
      quarantineIssueNumber = null
    }

    // Invalidate tracked SHA and reset quarantine state.
    lastKnownStateSHA = null
    await resetQuarantineState()

    if (dir) {
      rmSync(dir, { recursive: true, force: true })
      dir = null
    }
  })

  // -----------------------------------------------------------------------
  // Test: developer closes a quarantine issue on GitHub → next CI run
  // detects the closed issue via SearchClosedIssues → test is removed from
  // quarantine state and runs normally again.
  // -----------------------------------------------------------------------

  describe("closed quarantine issue detected by CLI", () => {
    test("removes the test from quarantine.json when its issue is closed on GitHub", async () => {
      const quarantinedTestID =
        "src/unquarantine.test.js::UnquarantineService::should be unquarantined"

      // Step 1: Create a real GitHub issue with the quarantine label.
      quarantineIssueNumber = await createIssueWithLabel(
        "[Quarantine] should be unquarantined",
        "quarantine",
      )

      // Step 2: Pre-populate quarantine state with the test entry referencing the issue.
      await writeQuarantineState({
        version: 1,
        updated_at: new Date().toISOString(),
        tests: {
          [quarantinedTestID]: {
            test_id: quarantinedTestID,
            file_path: "src/unquarantine.test.js",
            classname: "UnquarantineService",
            name: "should be unquarantined",
            suite: "",
            first_flaky_at: new Date().toISOString(),
            last_flaky_at: new Date().toISOString(),
            flaky_count: 1,
            quarantined_at: new Date().toISOString(),
            quarantined_by: "auto",
            issue_number: quarantineIssueNumber,
            issue_url: `https://github.com/${owner}/${repo}/issues/${quarantineIssueNumber}`,
          },
        },
      })

      // Step 3: Close the issue on GitHub — this signals that the test is fixed.
      await closeIssue(quarantineIssueNumber)

      // Step 4: Wait for GitHub Search API to index the closed issue.
      // The CLI uses label-based search (is:closed label:quarantine) for unquarantine detection.
      // Search index lag is typically 1-5s.
      await new Promise((r) => setTimeout(r, 5000))

      // Step 5: Run the CLI. The test passes now that it is no longer quarantined.
      const xmlPath = join(dir, "junit.xml")
      writeFileSync(
        xmlPath,
        `<?xml version="1.0"?>
<testsuites tests="1" failures="0">
  <testsuite name="src/unquarantine.test.js" tests="1" failures="0">
    <testcase classname="UnquarantineService" name="should be unquarantined" file="src/unquarantine.test.js" time="0.01"/>
  </testsuite>
</testsuites>`,
        "utf8",
      )

      const binDir = join(dir, "bin")
      mkdirSync(binDir)
      makeScript(binDir, "jest", "exit 0")

      const scriptPath = makeScript(dir, "fake-jest", "exit 0")
      writeConfig(dir, "version: 1\nframework: jest\n")

      const result = runCLI(dir, ["run", "--junitxml", xmlPath, "--", scriptPath], {
        PATH: `${binDir}:${process.env.PATH}`,
      })

      assert({
        given: "a quarantine run after the quarantine issue is closed",
        should: "exit 0",
        actual: result.status,
        expected: 0,
      })

      // Step 6: Verify the test was removed from quarantine state on GitHub.
      // Retry once after a short delay to allow for CDN propagation.
      let file = await getFileOnBranch(STATE_FILE)
      if (file) {
        const state = JSON.parse(file.content)
        if (Object.hasOwn(state.tests, quarantinedTestID)) {
          await new Promise((r) => setTimeout(r, 2000))
          file = await getFileOnBranch(STATE_FILE)
        }
      }

      assert({
        given: "quarantine.json after the CLI run",
        should: "exist on the GitHub branch",
        actual: file !== null,
        expected: true,
      })

      const state = file ? JSON.parse(file.content) : { tests: {} }

      assert({
        given: "a quarantine run after the issue is closed",
        should: "remove the test from quarantine.json (test is unquarantined)",
        actual: Object.hasOwn(state.tests, quarantinedTestID),
        expected: false,
      })
    })
  })

  // -----------------------------------------------------------------------
  // Scenario: Search API response shape — verify that GET /search/issues
  // returns { total_count, items: [{ number }] } as the production code
  // assumes (mock-fidelity check).
  // -----------------------------------------------------------------------

  describe("Search API response shape", () => {
    test("GET /search/issues returns total_count and items with number fields", async () => {
      // Create and close an issue with the quarantine label so the search
      // has at least one result to return.
      const issueNumber = await createIssueWithLabel(
        "[Quarantine] search shape probe",
        "quarantine",
      )
      // Track for cleanup.
      quarantineIssueNumber = issueNumber
      await closeIssue(issueNumber)

      // Wait for search indexing.
      await new Promise((r) => setTimeout(r, 5000))

      // Call the Search API directly — the same query the production code uses.
      const q = `repo:${owner}/${repo} is:issue is:closed label:quarantine`
      const searchRes = await fetch(
        `https://api.github.com/search/issues?${new URLSearchParams({ q, per_page: "100", page: "1" })}`,
        {
          headers: {
            Authorization: `Bearer ${token}`,
            Accept: "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
          },
        },
      )

      assert({
        given: "GET /search/issues with is:closed label:quarantine",
        should: "return status 200",
        actual: searchRes.status,
        expected: 200,
      })

      const data = await searchRes.json()

      assert({
        given: "the search response",
        should: "have a numeric total_count field",
        actual: typeof data.total_count,
        expected: "number",
      })

      assert({
        given: "the search response",
        should: "have an items array",
        actual: Array.isArray(data.items),
        expected: true,
      })

      assert({
        given: "the search response",
        should: "return at least one closed issue with the quarantine label",
        actual: data.items.length >= 1,
        expected: true,
      })

      const firstItem = data.items[0]

      assert({
        given: "a search result item",
        should: "have a numeric number field",
        actual: typeof firstItem.number,
        expected: "number",
      })

      // Verify that the issue we created is present in the results.
      const found = data.items.some((item) => item.number === issueNumber)

      assert({
        given: "a closed issue with the quarantine label",
        should: "appear in the search results",
        actual: found,
        expected: true,
      })
    })
  })
})
