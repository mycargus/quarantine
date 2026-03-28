/**
 * E2E test: Scenario 20 — CI run detects a new flaky test (issue + PR comment)
 *
 * Exercises the full flow: flaky detection, quarantine.json update on GitHub,
 * GitHub Issue creation, and PR comment posting with <!-- quarantine-bot -->.
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
  writeFileSync,
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
      'test: reset quarantine state',
    )
  } catch (err) {
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

async function createBranchWithEmptyState() {
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

  const createRes = await ghRequest('POST', '/git/refs', {
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
  await writeFileOnBranch(STATE_FILE, emptyState, null, 'chore: initialize quarantine state')
}

// --- Label helpers ---

async function ensureQuarantineLabelExists() {
  const res = await ghRequest('GET', '/labels/quarantine')
  if (res.status === 200) return
  if (res.status === 404) {
    const createRes = await ghRequest('POST', '/labels', {
      name: 'quarantine',
      color: 'e11d48',
      description: 'Flaky test quarantine',
    })
    if (createRes.status !== 201) {
      const text = await createRes.text()
      throw new Error(`ensureQuarantineLabelExists: create label failed ${createRes.status}: ${text}`)
    }
  } else {
    const text = await res.text()
    throw new Error(`ensureQuarantineLabelExists: unexpected ${res.status}: ${text}`)
  }
}

// --- Issue helpers ---

async function createProxyIssue(title) {
  const res = await ghRequest('POST', '/issues', {
    title,
    body: 'Proxy issue used as PR stand-in for e2e testing.',
  })
  if (res.status !== 201) {
    const text = await res.text()
    throw new Error(`createProxyIssue: unexpected ${res.status}: ${text}`)
  }
  const data = await res.json()
  return data.number
}

async function closeIssue(issueNumber) {
  if (!issueNumber) return
  await ghRequest('PATCH', `/issues/${issueNumber}`, { state: 'closed' })
}

async function findOpenIssueByTitle(title) {
  // Retry up to 3 times with 2s delay for GitHub CDN propagation.
  for (let attempt = 0; attempt < 3; attempt++) {
    if (attempt > 0) {
      await new Promise(r => setTimeout(r, 2000))
    }
    const res = await ghRequest('GET', '/issues?state=open&per_page=50')
    if (res.status !== 200) {
      const text = await res.text()
      throw new Error(`findOpenIssueByTitle: unexpected ${res.status}: ${text}`)
    }
    const issues = await res.json()
    const found = issues.find(issue => issue.title === title)
    if (found) return found
  }
  return null
}

async function findQuarantineBotComment(issueNumber) {
  // Retry up to 3 times with 2s delay for GitHub CDN propagation.
  for (let attempt = 0; attempt < 3; attempt++) {
    if (attempt > 0) {
      await new Promise(r => setTimeout(r, 2000))
    }
    const res = await ghRequest('GET', `/issues/${issueNumber}/comments`)
    if (res.status !== 200) {
      const text = await res.text()
      throw new Error(`findQuarantineBotComment: unexpected ${res.status}: ${text}`)
    }
    const comments = await res.json()
    const found = comments.find(c => c.body.startsWith('<!-- quarantine-bot -->'))
    if (found) return found
  }
  return null
}

// --- Local setup helpers ---

function createWorkDir() {
  const dir = mkdtempSync(join(tmpdir(), 'quarantine-e2e-fullflow-'))
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
    console.error('args:', args.join(' '))
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

describe('quarantine run — Scenario 20: CI run detects a new flaky test (issue + PR comment)', () => {
  let dir
  let proxyIssueNumber = null
  let quarantineIssueNumber = null

  beforeAll(async () => {
    if (!(await branchExists())) {
      await createBranchWithEmptyState()
    }
    await ensureQuarantineLabelExists()
  })

  beforeEach(async () => {
    dir = createWorkDir()
    proxyIssueNumber = await createProxyIssue('[e2e test proxy] PR stand-in for Scenario 20')
  })

  afterEach(async () => {
    // Close the proxy issue used as PR stand-in.
    await closeIssue(proxyIssueNumber)
    proxyIssueNumber = null

    // Close the quarantine issue created by the CLI (if found).
    await closeIssue(quarantineIssueNumber)
    quarantineIssueNumber = null

    // Invalidate tracked SHA and reset quarantine state.
    lastKnownStateSHA = null
    await resetQuarantineState()

    if (dir) {
      rmSync(dir, { recursive: true, force: true })
      dir = null
    }
  })

  // -----------------------------------------------------------------------
  // Scenario 20: CI run detects a new flaky test
  //
  // The main script writes a failing JUnit XML and exits 1.
  // A fake `jest` binary on PATH exits 0 on retry — simulating the test
  // passing on the second attempt (flaky detection).
  // The CLI is invoked with --pr pointing to a real GitHub issue number
  // (proxy issue) so PR comment posting is exercised.
  // -----------------------------------------------------------------------

  describe('flaky test detected — full notification flow', () => {
    test('exits 0, creates a GitHub issue, and posts a PR comment with the quarantine-bot marker', async () => {
      const xmlPath = join(dir, 'junit.xml')

      // Main run: write failing JUnit XML and exit 1.
      makeScript(
        dir,
        'fake-jest-main',
        `cat > "${xmlPath}" << 'XMLEOF'
<?xml version="1.0"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/payment.test.js" tests="1" failures="1">
    <testcase classname="PaymentService" name="should handle charge timeout" file="src/payment.test.js" time="0.01">
      <failure message="Timeout exceeded">Timeout exceeded</failure>
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

      const mainScriptPath = join(dir, 'fake-jest-main')

      const result = runCLI(dir, [
        'run',
        '--retries', '3',
        '--junitxml', xmlPath,
        '--pr', String(proxyIssueNumber),
        '--', mainScriptPath,
      ], {
        PATH: `${binDir}:${process.env.PATH}`,
      })

      assert({
        given: 'a newly detected flaky test',
        should: 'exit 0',
        actual: result.status,
        expected: 0,
      })

      // Find the quarantine issue created by the CLI.
      const createdIssue = await findOpenIssueByTitle('[Quarantine] should handle charge timeout')

      // Track it for cleanup in afterEach.
      if (createdIssue) {
        quarantineIssueNumber = createdIssue.number
      }

      assert({
        given: 'a newly detected flaky test with no prior issue',
        should: 'create a GitHub issue titled "[Quarantine] should handle charge timeout"',
        actual: createdIssue !== null,
        expected: true,
      })

      // Find the quarantine-bot PR comment on the proxy issue.
      const quarantineBotComment = await findQuarantineBotComment(proxyIssueNumber)

      assert({
        given: 'a newly detected flaky test with --pr flag set',
        should: 'post a PR comment with the <!-- quarantine-bot --> marker',
        actual: quarantineBotComment !== null,
        expected: true,
      })

      assert({
        given: 'the quarantine-bot PR comment',
        should: 'mention the flaky test name',
        actual: quarantineBotComment?.body.includes('should handle charge timeout'),
        expected: true,
      })
    })
  })
})

// =========================================================================
// Scenario 27: Issue dedup — second run does NOT create a duplicate issue
//
// High-risk API interaction: GET /search/issues?q=label:quarantine:{hash}+is:open
// The Search API query format must match what GitHub actually indexes.
// A mock always returns what you tell it; the real API may not find the issue
// if the query string is wrong.
// =========================================================================

describe('quarantine run — Scenario 27: issue dedup via Search API', () => {
  let dir
  let proxyIssueNumber = null
  let quarantineIssueNumber = null

  beforeAll(async () => {
    if (!(await branchExists())) {
      await createBranchWithEmptyState()
    }
    await ensureQuarantineLabelExists()
  })

  beforeEach(async () => {
    dir = createWorkDir()
    proxyIssueNumber = await createProxyIssue('[e2e test proxy] PR stand-in for Scenario 27')
  })

  afterEach(async () => {
    await closeIssue(proxyIssueNumber)
    proxyIssueNumber = null
    await closeIssue(quarantineIssueNumber)
    quarantineIssueNumber = null
    lastKnownStateSHA = null
    await resetQuarantineState()
    if (dir) {
      rmSync(dir, { recursive: true, force: true })
      dir = null
    }
  })

  describe('second run with same flaky test', () => {
    test('finds the existing issue via Search API and does NOT create a duplicate', async () => {
      const xmlPath = join(dir, 'junit.xml')

      // Main run: write failing JUnit XML and exit 1.
      makeScript(
        dir,
        'fake-jest-main',
        `cat > "${xmlPath}" << 'XMLEOF'
<?xml version="1.0"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/dedup.test.js" tests="1" failures="1">
    <testcase classname="DedupService" name="should not duplicate" file="src/dedup.test.js" time="0.01">
      <failure message="intermittent">intermittent</failure>
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
      const mainScriptPath = join(dir, 'fake-jest-main')
      const pathEnv = { PATH: `${binDir}:${process.env.PATH}` }

      // --- First run: creates the issue ---
      const run1 = runCLI(dir, [
        'run',
        '--retries', '3',
        '--junitxml', xmlPath,
        '--pr', String(proxyIssueNumber),
        '--', mainScriptPath,
      ], pathEnv)

      assert({
        given: 'first run detecting a flaky test',
        should: 'exit 0',
        actual: run1.status,
        expected: 0,
      })

      // Find the issue created by the first run.
      const issueTitle = '[Quarantine] should not duplicate'
      const createdIssue = await findOpenIssueByTitle(issueTitle)
      if (createdIssue) {
        quarantineIssueNumber = createdIssue.number
      }

      assert({
        given: 'first run with a new flaky test',
        should: 'create a GitHub issue',
        actual: createdIssue !== null,
        expected: true,
      })

      // Wait for GitHub Search API to index the new issue.
      // The CLI uses label-based search (label:quarantine:{hash}) for dedup.
      // Search index lag is typically 1-5s.
      await new Promise(r => setTimeout(r, 5000))

      // --- Second run: same flaky test, same repo ---
      // Recreate workdir to avoid stale local state, but keep same test.
      const dir2 = createWorkDir()
      const xmlPath2 = join(dir2, 'junit.xml')

      makeScript(
        dir2,
        'fake-jest-main',
        `cat > "${xmlPath2}" << 'XMLEOF'
<?xml version="1.0"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/dedup.test.js" tests="1" failures="1">
    <testcase classname="DedupService" name="should not duplicate" file="src/dedup.test.js" time="0.01">
      <failure message="intermittent">intermittent</failure>
    </testcase>
  </testsuite>
</testsuites>
XMLEOF
exit 1`,
      )

      const binDir2 = join(dir2, 'bin')
      mkdirSync(binDir2)
      makeScript(binDir2, 'jest', 'exit 0')
      writeConfig(dir2, 'version: 1\nframework: jest\n')

      const run2 = runCLI(dir2, [
        'run',
        '--retries', '3',
        '--junitxml', xmlPath2,
        '--pr', String(proxyIssueNumber),
        '--', join(dir2, 'fake-jest-main'),
      ], { PATH: `${binDir2}:${process.env.PATH}` })

      assert({
        given: 'second run detecting the same flaky test',
        should: 'exit 0',
        actual: run2.status,
        expected: 0,
      })

      // Verify only ONE issue exists with this title.
      // List open issues and count matches.
      const res = await ghRequest('GET', '/issues?state=open&per_page=100')
      const allIssues = await res.json()
      const matchingIssues = allIssues.filter(i => i.title === issueTitle)

      assert({
        given: 'two consecutive runs with the same flaky test',
        should: 'have exactly 1 open issue (dedup prevented duplicate)',
        actual: matchingIssues.length,
        expected: 1,
      })

      // Clean up the second workdir.
      rmSync(dir2, { recursive: true, force: true })
    })
  })
})

// =========================================================================
// Scenario 49: PR comment update — second run PATCHes the existing comment
//
// High-risk API interaction: GET /repos/.../issues/{pr}/comments?per_page=100
// followed by PATCH /repos/.../issues/comments/{id}
// The list-then-patch flow depends on real pagination and comment ID semantics.
// =========================================================================

describe('quarantine run — Scenario 49: second run updates existing PR comment', () => {
  let dir
  let proxyIssueNumber = null
  let quarantineIssueNumber = null

  beforeAll(async () => {
    if (!(await branchExists())) {
      await createBranchWithEmptyState()
    }
    await ensureQuarantineLabelExists()
  })

  beforeEach(async () => {
    dir = createWorkDir()
    proxyIssueNumber = await createProxyIssue('[e2e test proxy] PR stand-in for Scenario 49')
  })

  afterEach(async () => {
    await closeIssue(proxyIssueNumber)
    proxyIssueNumber = null
    await closeIssue(quarantineIssueNumber)
    quarantineIssueNumber = null
    lastKnownStateSHA = null
    await resetQuarantineState()
    if (dir) {
      rmSync(dir, { recursive: true, force: true })
      dir = null
    }
  })

  describe('second run on same PR', () => {
    test('PATCHes the existing quarantine-bot comment instead of creating a second one', async () => {
      const xmlPath = join(dir, 'junit.xml')

      makeScript(
        dir,
        'fake-jest-main',
        `cat > "${xmlPath}" << 'XMLEOF'
<?xml version="1.0"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/comment.test.js" tests="1" failures="1">
    <testcase classname="CommentService" name="should update comment" file="src/comment.test.js" time="0.01">
      <failure message="flaky timeout">flaky timeout</failure>
    </testcase>
  </testsuite>
</testsuites>
XMLEOF
exit 1`,
      )

      const binDir = join(dir, 'bin')
      mkdirSync(binDir)
      makeScript(binDir, 'jest', 'exit 0')

      writeConfig(dir, 'version: 1\nframework: jest\n')
      const mainScriptPath = join(dir, 'fake-jest-main')
      const pathEnv = { PATH: `${binDir}:${process.env.PATH}` }

      // --- First run: creates the PR comment ---
      const run1 = runCLI(dir, [
        'run',
        '--retries', '3',
        '--junitxml', xmlPath,
        '--pr', String(proxyIssueNumber),
        '--', mainScriptPath,
      ], pathEnv)

      assert({
        given: 'first run detecting a flaky test',
        should: 'exit 0',
        actual: run1.status,
        expected: 0,
      })

      // Track the quarantine issue for cleanup.
      const createdIssue = await findOpenIssueByTitle('[Quarantine] should update comment')
      if (createdIssue) {
        quarantineIssueNumber = createdIssue.number
      }

      // Verify first comment was created.
      const firstComment = await findQuarantineBotComment(proxyIssueNumber)

      assert({
        given: 'first run with --pr flag',
        should: 'create a quarantine-bot PR comment',
        actual: firstComment !== null,
        expected: true,
      })

      const firstCommentId = firstComment?.id

      // --- Second run: same test, same PR ---
      // Recreate workdir for clean local state.
      const dir2 = createWorkDir()
      const xmlPath2 = join(dir2, 'junit.xml')

      makeScript(
        dir2,
        'fake-jest-main',
        `cat > "${xmlPath2}" << 'XMLEOF'
<?xml version="1.0"?>
<testsuites tests="1" failures="1">
  <testsuite name="src/comment.test.js" tests="1" failures="1">
    <testcase classname="CommentService" name="should update comment" file="src/comment.test.js" time="0.01">
      <failure message="flaky timeout">flaky timeout</failure>
    </testcase>
  </testsuite>
</testsuites>
XMLEOF
exit 1`,
      )

      const binDir2 = join(dir2, 'bin')
      mkdirSync(binDir2)
      makeScript(binDir2, 'jest', 'exit 0')
      writeConfig(dir2, 'version: 1\nframework: jest\n')

      const run2 = runCLI(dir2, [
        'run',
        '--retries', '3',
        '--junitxml', xmlPath2,
        '--pr', String(proxyIssueNumber),
        '--', join(dir2, 'fake-jest-main'),
      ], { PATH: `${binDir2}:${process.env.PATH}` })

      assert({
        given: 'second run detecting the same flaky test on the same PR',
        should: 'exit 0',
        actual: run2.status,
        expected: 0,
      })

      // Verify: still exactly one quarantine-bot comment (PATCH, not POST).
      // Retry to allow for CDN propagation.
      const updatedComment = await findQuarantineBotComment(proxyIssueNumber)

      assert({
        given: 'second run on the same PR',
        should: 'still have a quarantine-bot comment',
        actual: updatedComment !== null,
        expected: true,
      })

      assert({
        given: 'second run PATCHing the existing comment',
        should: 'keep the same comment ID (updated in place)',
        actual: updatedComment?.id,
        expected: firstCommentId,
      })

      // Count all quarantine-bot comments — should be exactly 1.
      const allCommentsRes = await ghRequest('GET', `/issues/${proxyIssueNumber}/comments`)
      const allComments = await allCommentsRes.json()
      const botComments = allComments.filter(c => c.body.startsWith('<!-- quarantine-bot -->'))

      assert({
        given: 'two consecutive runs on the same PR',
        should: 'have exactly 1 quarantine-bot comment (no duplicates)',
        actual: botComments.length,
        expected: 1,
      })

      rmSync(dir2, { recursive: true, force: true })
    })
  })
})
