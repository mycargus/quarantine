/**
 * E2E test: dashboard artifact polling — GitHub Artifacts API shape
 *
 * Verifies that the real GitHub Artifacts API response matches the shape
 * assumed by the dashboard's listArtifacts function in
 * dashboard/app/lib/github.server.ts.
 *
 * High-risk interactions verified here:
 *   - GET /repos/{owner}/{repo}/actions/artifacts?per_page=100
 *     Response shape: total_count (number), artifacts[] with id, name,
 *     archive_download_url, created_at, expires_at
 *   - ETag-based conditional requests (If-None-Match → 304 Not Modified)
 *   - ETag header present on 200 response
 *
 * Required env vars:
 *   QUARANTINE_GITHUB_TOKEN  — PAT with repo scope
 *   QUARANTINE_TEST_OWNER    — GitHub org or user (e.g. "mycargus")
 *   QUARANTINE_TEST_REPO     — repository name (e.g. "quarantine-test-fixture")
 */

import { assert } from "riteway/vitest"
import { beforeAll, describe, test } from "vitest"

const token = process.env.QUARANTINE_GITHUB_TOKEN
const owner = process.env.QUARANTINE_TEST_OWNER
const repo = process.env.QUARANTINE_TEST_REPO

// --- GitHub API helper ---

async function ghRequest(method, path, headers = {}) {
  return fetch(`https://api.github.com/repos/${owner}/${repo}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/vnd.github+json",
      "X-GitHub-Api-Version": "2022-11-28",
      ...headers,
    },
  })
}

// ---

beforeAll(() => {
  if (!token || !owner || !repo) {
    throw new Error(
      "E2E tests require QUARANTINE_GITHUB_TOKEN, QUARANTINE_TEST_OWNER, and QUARANTINE_TEST_REPO. See test/e2e/.env.example.",
    )
  }
})

// =========================================================================
// Artifacts list — response shape
//
// The dashboard destructures: total_count, artifacts[].id, artifacts[].name,
// artifacts[].archive_download_url, artifacts[].created_at, artifacts[].expires_at
// =========================================================================

describe("dashboard artifact polling — GET /actions/artifacts?per_page=100", () => {
  test("returns status 200", async () => {
    const res = await ghRequest("GET", "/actions/artifacts?per_page=100")

    assert({
      given: "a GET request to the artifacts API",
      should: "return status 200",
      actual: res.status,
      expected: 200,
    })
  })

  test("response body has total_count as a number", async () => {
    const res = await ghRequest("GET", "/actions/artifacts?per_page=100")
    const data = await res.json()

    assert({
      given: "a successful artifacts API response",
      should: "include total_count as a number",
      actual: typeof data.total_count,
      expected: "number",
    })
  })

  test("response body has artifacts as an array", async () => {
    const res = await ghRequest("GET", "/actions/artifacts?per_page=100")
    const data = await res.json()

    assert({
      given: "a successful artifacts API response",
      should: "include artifacts as an array",
      actual: Array.isArray(data.artifacts),
      expected: true,
    })
  })

  test("response includes an ETag header", async () => {
    const res = await ghRequest("GET", "/actions/artifacts?per_page=100")

    assert({
      given: "a successful artifacts API response",
      should: "include an ETag header",
      actual: res.headers.get("etag") !== null,
      expected: true,
    })
  })

  test("each artifact has the required fields with the expected types", async () => {
    const res = await ghRequest("GET", "/actions/artifacts?per_page=100")
    const data = await res.json()

    // The test fixture repo must have at least one artifact for this
    // assertion to be meaningful. Fail clearly if that precondition is unmet.
    assert({
      given: "the test fixture repository",
      should: "have at least one artifact to verify field shapes",
      actual: data.artifacts.length >= 1,
      expected: true,
    })

    const artifact = data.artifacts[0]

    assert({
      given: "the first artifact",
      should: "have id as a number",
      actual: typeof artifact.id,
      expected: "number",
    })

    assert({
      given: "the first artifact",
      should: "have name as a string",
      actual: typeof artifact.name,
      expected: "string",
    })

    assert({
      given: "the first artifact",
      should: "have archive_download_url as a string",
      actual: typeof artifact.archive_download_url,
      expected: "string",
    })

    assert({
      given: "the first artifact",
      should: "have created_at as a string",
      actual: typeof artifact.created_at,
      expected: "string",
    })

    assert({
      given: "the first artifact",
      should: "have expires_at as a string",
      actual: typeof artifact.expires_at,
      expected: "string",
    })
  })

  test("archive_download_url is a non-empty HTTPS URL", async () => {
    const res = await ghRequest("GET", "/actions/artifacts?per_page=100")
    const data = await res.json()

    assert({
      given: "the test fixture repository",
      should: "have at least one artifact to verify archive_download_url",
      actual: data.artifacts.length >= 1,
      expected: true,
    })

    const { archive_download_url } = data.artifacts[0]

    assert({
      given: "the first artifact's archive_download_url",
      should: "start with https://",
      actual: archive_download_url.startsWith("https://"),
      expected: true,
    })
  })
})

// =========================================================================
// ETag conditional request — 304 Not Modified
//
// The dashboard sends If-None-Match on subsequent polls to avoid
// re-downloading unchanged data. The real API must honor this.
// =========================================================================

describe("dashboard artifact polling — ETag conditional request (304 Not Modified)", () => {
  test("sending the ETag from a prior response as If-None-Match returns 304", async () => {
    // First request: get the current ETag.
    const firstRes = await ghRequest("GET", "/actions/artifacts?per_page=100")

    assert({
      given: "the initial artifacts request",
      should: "return status 200",
      actual: firstRes.status,
      expected: 200,
    })

    const etag = firstRes.headers.get("etag")

    assert({
      given: "the initial artifacts response",
      should: "include an ETag header to use for conditional requests",
      actual: etag !== null,
      expected: true,
    })

    // Second request: send If-None-Match with the ETag from the first response.
    const secondRes = await ghRequest("GET", "/actions/artifacts?per_page=100", {
      "If-None-Match": etag,
    })

    assert({
      given: "a second artifacts request with the prior ETag in If-None-Match",
      should: "return 304 Not Modified",
      actual: secondRes.status,
      expected: 304,
    })
  })
})
