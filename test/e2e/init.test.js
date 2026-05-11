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

import { execSync, spawnSync } from "node:child_process"
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { assert } from "riteway/vitest"
import { afterAll, beforeAll, describe, test } from "vitest"

const BRANCH = "quarantine/state"

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

async function getFileOnBranch(path) {
  const res = await ghRequest("GET", `/contents/${path}?ref=${BRANCH}`)
  if (res.status !== 200) {
    const text = await res.text()
    throw new Error(`getFileOnBranch(${path}): unexpected ${res.status}: ${text}`)
  }
  const data = await res.json()
  return Buffer.from(data.content.replace(/\n/g, ""), "base64").toString("utf8")
}

// --- Test suite ---

if (!token) throw new Error("QUARANTINE_GITHUB_TOKEN is required")
if (!owner) throw new Error("QUARANTINE_TEST_OWNER is required")
if (!repo) throw new Error("QUARANTINE_TEST_REPO is required")

let dir
let result // spawnSync result

describe("quarantine init — E2E against real GitHub", () => {
  beforeAll(async () => {
    // Do NOT delete the quarantine/state branch — quarantine-observe.test.js
    // depends on it to verify real CI output. The init command is idempotent:
    // it skips branch creation if the branch already exists.

    // Create a temp directory with a git repo whose origin points to the test
    // repository. Phase 1 scans git remotes to emit advisory hint comments.
    dir = mkdtempSync(join(tmpdir(), "quarantine-e2e-"))
    execSync("git init", { cwd: dir, stdio: "pipe" })
    execSync('git config user.email "test@example.com"', { cwd: dir, stdio: "pipe" })
    execSync('git config user.name "Test"', { cwd: dir, stdio: "pipe" })
    execSync(`git remote add origin https://github.com/${owner}/${repo}.git`, {
      cwd: dir,
      stdio: "pipe",
    })

    // Phase 1: no config yet — init writes a partial .quarantine/config.yml with
    // an empty github block (plus hint comments for the detected origin) and exits 2.
    spawnSync(binPath, ["init"], {
      cwd: dir,
      encoding: "utf8",
      env: { ...process.env, QUARANTINE_GITHUB_TOKEN: token },
      timeout: 60_000,
    })

    // Simulate the hand-edit step (Scenario 174 → 175): fill in owner/repo.
    const configPath = join(dir, ".quarantine", "config.yml")
    const configContent = readFileSync(configPath, "utf8")
    writeFileSync(
      configPath,
      configContent
        .replace("owner: # set to your GitHub organization or user", `owner: ${owner}`)
        .replace("repo:  # set to your GitHub repository name", `repo:  ${repo}`),
    )

    // Phase 2: config now has owner/repo — init validates the token, creates the
    // state branch (idempotent), and exits 0.
    result = spawnSync(binPath, ["init"], {
      cwd: dir,
      encoding: "utf8",
      env: { ...process.env, QUARANTINE_GITHUB_TOKEN: token },
      timeout: 60_000,
    })
  })

  afterAll(async () => {
    // Do NOT delete the quarantine/state branch — quarantine-observe.test.js
    // depends on it to verify real CI output. The branch is shared state
    // between the fixture CI and the E2E observation tests.
    if (dir) {
      rmSync(dir, { recursive: true, force: true })
    }
  })

  test("exits without error", () => {
    assert({
      given: "quarantine init command",
      should: "exit without error",
      actual: result.status,
      expected: 0,
    })
  })

  test("prints success message", () => {
    assert({
      given: "successful init",
      should: "print success message",
      actual: result.stdout.includes("Quarantine initialized."),
      expected: true,
    })
  })

  test("creates .quarantine/config.yml locally", () => {
    const path = join(dir, ".quarantine", "config.yml")
    const content = existsSync(path) ? readFileSync(path, "utf8") : ""
    assert({
      given: "successful init",
      should: "create .quarantine/config.yml with version: 1",
      actual: content.includes("version: 1"),
      expected: true,
    })
    assert({
      given: "successful init",
      should: "create .quarantine/config.yml with test_suites:",
      actual: content.includes("test_suites:"),
      expected: true,
    })
  })

  test("creates quarantine/state branch on GitHub", async () => {
    assert({
      given: "successful init",
      should: "create quarantine/state branch on GitHub",
      actual: await branchExists(),
      expected: true,
    })
  })

  test("writes .quarantine/README.md to the state branch", async () => {
    const content = await getFileOnBranch(".quarantine/README.md")
    assert({
      given: "quarantine/state branch after init",
      should: "have a .quarantine/README.md file",
      actual: content.length > 0,
      expected: true,
    })
  })

  // Drift detection: the init command calls GET /repos/{owner}/{repo} and reads
  // default_branch to determine which SHA to branch from. If this field is
  // renamed or removed, init would silently fail or branch from the wrong ref.
  test("GET /repos/{owner}/{repo} includes default_branch as a string", async () => {
    const res = await ghRequest("GET", "")
    const body = await res.json()
    assert({
      given: "GET /repos/{owner}/{repo} against the real GitHub API",
      should: "return default_branch as a string",
      actual: typeof body.default_branch,
      expected: "string",
    })
  })
})
