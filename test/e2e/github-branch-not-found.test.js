/**
 * E2E test: GitHub branch-not-found error message fidelity
 *
 * Verifies that GitHub's 404 response body for a contents request against a
 * non-existent ref contains the exact string "No commit found for the ref".
 * The CLI's GetContents function (cli/internal/github/contents_ops.go) uses
 * this string match to distinguish branch-not-found from file-not-found.
 * If GitHub changes the message, the CLI will silently misclassify the error
 * and break the init-detection flow.
 *
 * Required env vars:
 *   QUARANTINE_GITHUB_TOKEN  — PAT with repo scope
 *   QUARANTINE_TEST_OWNER    — GitHub org or user (e.g. "my-org")
 *   QUARANTINE_TEST_REPO     — repository name (e.g. "quarantine-test-fixture")
 */

import { assert } from "riteway/vitest"
import { afterAll, beforeAll, describe, test } from "vitest"

const BRANCH = "quarantine/state"

const token = process.env.QUARANTINE_GITHUB_TOKEN
const owner = process.env.QUARANTINE_TEST_OWNER
const repo = process.env.QUARANTINE_TEST_REPO

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

async function deleteBranch() {
  const res = await ghRequest("DELETE", `/git/refs/heads/${BRANCH}`)
  if (res.status !== 204) {
    const text = await res.text()
    throw new Error(`deleteBranch: unexpected ${res.status}: ${text}`)
  }
}

// --- Credential guard ---

if (!token) throw new Error("QUARANTINE_GITHUB_TOKEN is required")
if (!owner) throw new Error("QUARANTINE_TEST_OWNER is required")
if (!repo) throw new Error("QUARANTINE_TEST_REPO is required")

// --- Test suite ---

describe("GitHub contents API — branch-not-found error message fidelity", () => {
  beforeAll(async () => {
    if (await branchExists()) {
      await deleteBranch()
    }
  })

  afterAll(async () => {
    if (await branchExists()) {
      await deleteBranch()
    }
  })

  test("returns 404 when the ref branch does not exist", async () => {
    const res = await ghRequest("GET", `/contents/quarantine.json?ref=${BRANCH}`)

    assert({
      given: `GET /contents/quarantine.json?ref=${BRANCH} when branch does not exist`,
      should: "return HTTP 404",
      actual: res.status,
      expected: 404,
    })
  })

  test('404 response body contains "No commit found for the ref"', async () => {
    const res = await ghRequest("GET", `/contents/quarantine.json?ref=${BRANCH}`)
    const body = await res.json()

    assert({
      given: `GET /contents/quarantine.json?ref=${BRANCH} when branch does not exist`,
      should: 'include "No commit found for the ref" in the error message field',
      actual:
        typeof body.message === "string" && body.message.includes("No commit found for the ref"),
      expected: true,
    })
  })
})
