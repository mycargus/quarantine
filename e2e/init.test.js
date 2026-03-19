/**
 * E2E test: quarantine init
 *
 * Exercises the full `quarantine init` flow against a real GitHub repository.
 * Requires the CLI binary to be built and three environment variables to be set.
 *
 * Required env vars:
 *   QUARANTINE_GITHUB_TOKEN  — PAT with repo scope
 *   QUARANTINE_TEST_OWNER    — GitHub org or user (e.g. "my-org")
 *   QUARANTINE_TEST_REPO     — repository name (e.g. "quarantine-test-fixture")
 *
 * Optional:
 *   QUARANTINE_BIN           — path to quarantine binary (default: ../bin/quarantine)
 */

import { describe, test, beforeAll, afterAll } from 'vitest'
import { assert } from 'riteway/vitest'
import { mkdtempSync, rmSync, readFileSync, existsSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { execSync, spawnSync } from 'node:child_process'

const BRANCH = 'quarantine/state'

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

async function deleteBranch() {
  const res = await ghRequest('DELETE', `/git/refs/heads/${BRANCH}`)
  if (res.status !== 204) {
    const text = await res.text()
    throw new Error(`deleteBranch: unexpected ${res.status}: ${text}`)
  }
}

async function getFileOnBranch(path) {
  const res = await ghRequest('GET', `/contents/${path}?ref=${BRANCH}`)
  if (res.status !== 200) {
    const text = await res.text()
    throw new Error(`getFileOnBranch(${path}): unexpected ${res.status}: ${text}`)
  }
  const data = await res.json()
  return Buffer.from(data.content.replace(/\n/g, ''), 'base64').toString('utf8')
}

// --- Test suite ---

if (!token) throw new Error('QUARANTINE_GITHUB_TOKEN is required')
if (!owner) throw new Error('QUARANTINE_TEST_OWNER is required')
if (!repo) throw new Error('QUARANTINE_TEST_REPO is required')

let dir
let result // spawnSync result

describe(
  'quarantine init — E2E against real GitHub',
  () => {
    beforeAll(async () => {
      // Clean up any leftover branch from a prior run.
      if (await branchExists()) {
        await deleteBranch()
      }

      // Create a temp directory with a git repo whose origin points to the
      // test repository. The init command reads the remote via `git remote get-url`.
      dir = mkdtempSync(join(tmpdir(), 'quarantine-e2e-'))
      execSync('git init', { cwd: dir, stdio: 'pipe' })
      execSync('git config user.email "test@example.com"', { cwd: dir, stdio: 'pipe' })
      execSync('git config user.name "Test"', { cwd: dir, stdio: 'pipe' })
      execSync(
        `git remote add origin https://github.com/${owner}/${repo}.git`,
        { cwd: dir, stdio: 'pipe' },
      )

      // Run `quarantine init` with framework=jest and defaults for everything else.
      result = spawnSync(binPath, ['init'], {
        cwd: dir,
        input: 'jest\n\n\n',
        encoding: 'utf8',
        env: { ...process.env, QUARANTINE_GITHUB_TOKEN: token },
        timeout: 60_000,
      })
    })

    afterAll(async () => {
      if (await branchExists()) {
        await deleteBranch()
      }
      if (dir) {
        rmSync(dir, { recursive: true, force: true })
      }
    })

    test('exits without error', () => {
      assert({
        given: 'quarantine init command',
        should: 'exit without error',
        actual: result.status,
        expected: 0
      })
    })

    test('prints success message', () => {
      assert({
        given: 'successful init',
        should: 'print success message',
        actual: result.stdout.includes('Quarantine initialized successfully'),
        expected: true
      })
    })

    test('creates quarantine.yml locally', () => {
      const path = join(dir, 'quarantine.yml')
      const content = existsSync(path) ? readFileSync(path, 'utf8') : ''
      assert({
        given: 'successful init',
        should: 'create quarantine.yml with version: 1',
        actual: content.includes('version: 1'),
        expected: true
      })
    })

    test('creates quarantine/state branch on GitHub', async () => {
      assert({
        given: 'successful init',
        should: 'create quarantine/state branch on GitHub',
        actual: await branchExists(),
        expected: true
      })
    })

    test('writes quarantine.json with version: 1 to the branch', async () => {
      const raw = await getFileOnBranch('quarantine.json')
      const state = JSON.parse(raw)
      assert({
        given: 'quarantine.json on quarantine/state branch',
        should: 'have version: 1',
        actual: state.version,
        expected: 1
      })
    })

    test('quarantine.json contains a tests object', async () => {
      const raw = await getFileOnBranch('quarantine.json')
      const state = JSON.parse(raw)
      assert({
        given: 'quarantine.json on quarantine/state branch',
        should: 'contain a tests object',
        actual: typeof state.tests,
        expected: 'object'
      })
    })
  },
)
