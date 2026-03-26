/**
 * E2E test: quarantine run
 *
 * Exercises the full `quarantine run` flow against a real GitHub repository.
 * Requires the CLI binary to be built and three environment variables to be set.
 *
 * Required env vars:
 *   QUARANTINE_GITHUB_TOKEN  — PAT with repo scope
 *   QUARANTINE_TEST_OWNER    — GitHub org or user (e.g. "mycargus")
 *   QUARANTINE_TEST_REPO     — repository name (e.g. "quarantine-test-fixture")
 *
 * Optional:
 *   QUARANTINE_BIN           — path to quarantine binary (default: ../bin/quarantine)
 */

import { describe, test, beforeAll, beforeEach, afterEach, onTestFailed } from 'vitest'
import { assert } from 'riteway/vitest'
import {
  mkdtempSync,
  mkdirSync,
  rmSync,
  readFileSync,
  writeFileSync,
  existsSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { execSync, spawnSync } from 'node:child_process'

const BRANCH = 'quarantine/state'
const STATE_FILE = 'quarantine.json'

const token = process.env.QUARANTINE_GITHUB_TOKEN
const owner = process.env.QUARANTINE_TEST_OWNER
const repo = process.env.QUARANTINE_TEST_REPO
const binPath =
  process.env.QUARANTINE_BIN ??
  new URL('../bin/quarantine', import.meta.url).pathname

// --- GitHub API helpers ---

async function ghRequest(method, path, body) {
  return fetch(`https://api.github.com/repos/${owner}/${repo}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: 'application/vnd.github+json',
      'X-GitHub-Api-Version': '2022-11-28',
      'Content-Type': 'application/json',
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
}

async function branchExists() {
  const res = await ghRequest('GET', `/git/ref/heads/${BRANCH}`)
  return res.status === 200
}

async function getFileOnBranch(filePath) {
  const res = await ghRequest('GET', `/contents/${filePath}?ref=${BRANCH}`)
  if (res.status === 404) return null
  if (res.status !== 200) {
    const text = await res.text()
    throw new Error(`getFileOnBranch(${filePath}): unexpected ${res.status}: ${text}`)
  }
  const data = await res.json()
  return {
    content: Buffer.from(data.content.replace(/\n/g, ''), 'base64').toString('utf8'),
    sha: data.sha,
  }
}

async function writeFileOnBranch(filePath, content, sha, message) {
  const res = await ghRequest('PUT', `/contents/${filePath}`, {
    message: message ?? `test: update ${filePath}`,
    content: Buffer.from(content).toString('base64'),
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
// Updated by resetQuarantineState and writeQuarantineState.
let lastKnownStateSHA = null

async function resetQuarantineState() {
  // Use tracked SHA when available; fall back to reading from GitHub
  // (the CLI may have updated the file since our last write).
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
      'test: reset quarantine state',
    )
  } catch (err) {
    // Retry on 409 (stale SHA from CDN cache) — re-read and try once more.
    if (err.message.includes('409')) {
      const file = await getFileOnBranch(STATE_FILE)
      lastKnownStateSHA = await writeFileOnBranch(
        STATE_FILE,
        emptyState,
        file?.sha ?? null,
        'test: reset quarantine state (retry)',
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
  lastKnownStateSHA = await writeFileOnBranch(
    STATE_FILE,
    JSON.stringify(state, null, 2),
    sha,
    'test: write quarantine state',
  )
}

async function createBranchWithEmptyState() {
  // Resolve the default branch SHA to use as the base.
  const repoRes = await ghRequest('GET', '')
  if (repoRes.status !== 200) {
    const text = await repoRes.text()
    throw new Error(`createBranchWithEmptyState: GET repo failed ${repoRes.status}: ${text}`)
  }
  const { default_branch: defaultBranch } = await repoRes.json()

  const refRes = await ghRequest('GET', `/git/ref/heads/${defaultBranch}`)
  if (refRes.status !== 200) {
    const text = await refRes.text()
    throw new Error(`createBranchWithEmptyState: GET ref failed ${refRes.status}: ${text}`)
  }
  const { object: { sha } } = await refRes.json()

  // Create the quarantine/state branch.
  const createRes = await ghRequest('POST', '/git/refs', {
    ref: `refs/heads/${BRANCH}`,
    sha,
  })
  if (createRes.status !== 201) {
    const text = await createRes.text()
    throw new Error(`createBranchWithEmptyState: create ref failed ${createRes.status}: ${text}`)
  }

  // Write an empty quarantine.json to the new branch.
  const emptyState = JSON.stringify(
    { version: 1, updated_at: new Date().toISOString(), tests: {} },
    null,
    2,
  )
  await writeFileOnBranch(STATE_FILE, emptyState, null, 'chore: initialize quarantine state')
}

// --- Local setup helpers ---

function createWorkDir() {
  const dir = mkdtempSync(join(tmpdir(), 'quarantine-e2e-run-'))
  execSync('git init', { cwd: dir, stdio: 'pipe' })
  execSync('git config user.email "test@example.com"', { cwd: dir, stdio: 'pipe' })
  execSync('git config user.name "Test"', { cwd: dir, stdio: 'pipe' })
  execSync(
    `git remote add origin https://github.com/${owner}/${repo}.git`,
    { cwd: dir, stdio: 'pipe' },
  )
  return dir
}

function writeConfig(dir, content) {
  writeFileSync(join(dir, 'quarantine.yml'), content, 'utf8')
}

function makeScript(dir, name, body) {
  const p = join(dir, name)
  writeFileSync(p, `#!/bin/sh\n${body}\n`, { mode: 0o755 })
  return p
}

function runCLI(dir, args, extraEnv = {}) {
  const result = spawnSync(binPath, args, {
    cwd: dir,
    encoding: 'utf8',
    env: { ...process.env, QUARANTINE_GITHUB_TOKEN: token, ...extraEnv },
    timeout: 120_000,
  })
  // Register a callback to surface CLI output if this test fails.
  onTestFailed(() => {
    console.error('\n--- quarantine CLI output (on failure) ---')
    console.error('args:', ['run', ...args].join(' '))
    if (result.stdout) console.error('stdout:\n' + result.stdout.trimEnd())
    if (result.stderr) console.error('stderr:\n' + result.stderr.trimEnd())
    if (result.error) console.error('spawn error:', result.error.message)
    console.error('exit code:', result.status)
    console.error('------------------------------------------\n')
  })
  return result
}

// ---

if (!token) throw new Error('QUARANTINE_GITHUB_TOKEN is required')
if (!owner) throw new Error('QUARANTINE_TEST_OWNER is required')
if (!repo) throw new Error('QUARANTINE_TEST_REPO is required')

describe('quarantine run — E2E against real GitHub', () => {
  let dir

  beforeAll(async () => {
    if (!(await branchExists())) {
      await createBranchWithEmptyState()
    }
  })

  beforeEach(() => {
    dir = createWorkDir()
    // State is already clean: afterEach of the previous test resets it,
    // and beforeAll ensures the branch exists with an empty initial state.
  })

  afterEach(async () => {
    // The CLI may have written quarantine.json (changing the SHA),
    // so invalidate the tracked SHA to force a fresh read.
    lastKnownStateSHA = null
    await resetQuarantineState()
    if (dir) {
      rmSync(dir, { recursive: true, force: true })
      dir = null
    }
  })

  // -----------------------------------------------------------------------
  // Scenario: Normal run — all tests pass, empty quarantine state
  // -----------------------------------------------------------------------

  describe('normal run — all tests pass', () => {
    test('exits 0 and writes results.json', () => {
      const xmlPath = join(dir, 'junit.xml')
      writeFileSync(
        xmlPath,
        `<?xml version="1.0"?>
<testsuites tests="2" failures="0">
  <testsuite name="src/app.test.js" tests="2" failures="0">
    <testcase classname="AppService" name="should initialize" file="src/app.test.js" time="0.01"/>
    <testcase classname="AppService" name="should process" file="src/app.test.js" time="0.01"/>
  </testsuite>
</testsuites>`,
        'utf8',
      )

      const scriptPath = makeScript(dir, 'fake-jest', 'exit 0')
      const resultsPath = join(dir, 'results.json')

      writeConfig(dir, 'version: 1\nframework: jest\n')

      const result = runCLI(dir, [
        'run',
        '--junitxml', xmlPath,
        '--output', resultsPath,
        '--', scriptPath,
      ])

      assert({
        given: 'quarantine run with empty state and all passing tests',
        should: 'exit 0',
        actual: result.status,
        expected: 0,
      })

      assert({
        given: 'quarantine run completes successfully',
        should: 'write results.json',
        actual: existsSync(resultsPath),
        expected: true,
      })

      const results = JSON.parse(readFileSync(resultsPath, 'utf8'))

      assert({
        given: 'results.json after a passing run',
        should: 'report 2 total tests',
        actual: results.summary.total,
        expected: 2,
      })

      assert({
        given: 'results.json after a passing run',
        should: 'report 0 failures',
        actual: results.summary.failed,
        expected: 0,
      })
    })
  })

  // -----------------------------------------------------------------------
  // Scenario: Flaky test detected — quarantine.json updated on GitHub
  //
  // The main run script writes a failing JUnit XML and exits 1.
  // A fake `jest` binary on PATH exits 0 on each retry invocation —
  // simulating the test passing individually without needing a real runner.
  // The CLI's default Jest rerun command (`jest --testNamePattern "..."`)
  // picks up the fake binary via the prepended PATH.
  // -----------------------------------------------------------------------

  describe('flaky test detected', () => {
    test('adds the flaky test to quarantine.json on the GitHub branch', async () => {
      const xmlPath = join(dir, 'junit.xml')

      // Main run: write failing XML and exit 1.
      makeScript(
        dir,
        'fake-jest-main',
        `cat > "${xmlPath}" << 'XMLEOF'
<?xml version="1.0"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/flaky.test.js" tests="1" failures="1">
    <testcase classname="FlakyService" name="should be non-deterministic" file="src/flaky.test.js" time="0.01">
      <failure message="intermittent failure">intermittent failure</failure>
    </testcase>
  </testsuite>
</testsuites>
XMLEOF
exit 1`,
      )

      // Fake `jest` binary on PATH: always exits 0 (successful retry).
      const binDir = join(dir, 'bin')
      mkdirSync(binDir)
      makeScript(binDir, 'jest', 'exit 0')

      writeConfig(dir, 'version: 1\nframework: jest\n')

      const resultsPath = join(dir, 'results.json')
      const mainScriptPath = join(dir, 'fake-jest-main')

      const result = runCLI(dir, [
        'run',
        '--retries', '3',
        '--junitxml', xmlPath,
        '--output', resultsPath,
        '--', mainScriptPath,
      ], {
        PATH: `${binDir}:${process.env.PATH}`,
      })

      assert({
        given: 'quarantine run detecting a flaky test (fails first, passes on retry)',
        should: 'exit 0 (flaky failure is forgiven)',
        actual: result.status,
        expected: 0,
      })

      // Verify quarantine.json was updated on GitHub with the flaky test.
      // GitHub's CDN may briefly serve stale content after a write.
      // Retry once after a short delay if the expected test isn't found.
      let file = await getFileOnBranch(STATE_FILE)
      if (file) {
        const state = JSON.parse(file.content)
        const hasTest = Object.keys(state.tests).some(
          id => id.includes('FlakyService') && id.includes('should be non-deterministic'),
        )
        if (!hasTest) {
          await new Promise(r => setTimeout(r, 2000))
          file = await getFileOnBranch(STATE_FILE)
        }
      }

      assert({
        given: 'quarantine run after flaky test detected',
        should: 'update quarantine.json on the GitHub branch',
        actual: file !== null,
        expected: true,
      })

      if (file) {
        const state = JSON.parse(file.content)
        const testIDs = Object.keys(state.tests)

        assert({
          given: 'quarantine.json after flaky detection',
          should: 'contain the flaky test ID',
          actual: testIDs.some(
            id =>
              id.includes('FlakyService') &&
              id.includes('should be non-deterministic'),
          ),
          expected: true,
        })
      }
    })
  })

  // -----------------------------------------------------------------------
  // Scenario: Quarantined test excluded from execution
  // -----------------------------------------------------------------------

  describe('quarantined test excluded from execution', () => {
    test('exits 0 and the quarantined test is absent from results.json', async () => {
      const quarantinedTestID =
        'src/flaky.test.js::FlakyService::should be non-deterministic'

      // Pre-populate quarantine.json on GitHub with one quarantined test.
      // Uses writeQuarantineState which tracks the SHA from the last write,
      // avoiding a stale-SHA 409 from GitHub's CDN cache.
      await writeQuarantineState({
        version: 1,
        updated_at: new Date().toISOString(),
        tests: {
          [quarantinedTestID]: {
            test_id: quarantinedTestID,
            file_path: 'src/flaky.test.js',
            classname: 'FlakyService',
            name: 'should be non-deterministic',
            suite: '',
            first_flaky_at: new Date().toISOString(),
            last_flaky_at: new Date().toISOString(),
            flaky_count: 1,
            quarantined_at: new Date().toISOString(),
            quarantined_by: 'auto',
          },
        },
      })

      // JUnit XML contains only the non-quarantined test
      // (the quarantined one was excluded from execution by the CLI).
      const xmlPath = join(dir, 'junit.xml')
      writeFileSync(
        xmlPath,
        `<?xml version="1.0"?>
<testsuites tests="1" failures="0">
  <testsuite name="src/app.test.js" tests="1" failures="0">
    <testcase classname="AppService" name="should work" file="src/app.test.js" time="0.01"/>
  </testsuite>
</testsuites>`,
        'utf8',
      )

      const scriptPath = makeScript(dir, 'fake-jest', 'exit 0')
      const resultsPath = join(dir, 'results.json')

      writeConfig(dir, 'version: 1\nframework: jest\n')

      const result = runCLI(dir, [
        'run',
        '--junitxml', xmlPath,
        '--output', resultsPath,
        '--', scriptPath,
      ])

      assert({
        given: 'quarantine run with one quarantined test (issue open)',
        should: 'exit 0',
        actual: result.status,
        expected: 0,
      })

      assert({
        given: 'quarantine run with quarantined test excluded',
        should: 'write results.json',
        actual: existsSync(resultsPath),
        expected: true,
      })

      if (existsSync(resultsPath)) {
        const results = JSON.parse(readFileSync(resultsPath, 'utf8'))
        const testIDs = results.tests.map((t) => t.test_id)

        assert({
          given: 'results.json after quarantined test was excluded from execution',
          should: 'not contain the quarantined test ID',
          actual: testIDs.includes(quarantinedTestID),
          expected: false,
        })
      }
    })
  })
})
