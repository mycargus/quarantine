/**
 * E2E test: Dashboard artifact API interactions
 *
 * Verifies that the GitHub Artifacts API behaves as the dashboard's
 * integration tests assume. These tests catch mock-fidelity drift:
 * response shapes, ETag round-trips, zip structure, and redirect behavior.
 *
 * Required env vars:
 *   QUARANTINE_GITHUB_TOKEN  — PAT with repo + actions:read scope
 *   QUARANTINE_TEST_OWNER    — GitHub org or user (e.g. "mycargus")
 *   QUARANTINE_TEST_REPO     — repository name (e.g. "quarantine-test-fixture")
 */

import { assert } from "riteway/vitest"
import { beforeAll, describe, test } from "vitest"

const token = process.env.QUARANTINE_GITHUB_TOKEN
const owner = process.env.QUARANTINE_TEST_OWNER
const repo = process.env.QUARANTINE_TEST_REPO

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

beforeAll(() => {
  if (!token || !owner || !repo) {
    throw new Error(
      "E2E tests require QUARANTINE_GITHUB_TOKEN, QUARANTINE_TEST_OWNER, and QUARANTINE_TEST_REPO",
    )
  }
})

describe("Dashboard: GitHub Artifacts API", () => {
  let firstEtag = null
  let artifacts = []

  test("GET /actions/artifacts returns expected response shape", async (_t) => {
    const res = await ghRequest("GET", "/actions/artifacts?per_page=100")

    assert({
      given: "a GET request to the artifacts API",
      should: "return status 200",
      actual: res.status,
      expected: 200,
    })

    const data = await res.json()

    assert({
      given: "the artifacts API response",
      should: "have a total_count number field",
      actual: typeof data.total_count,
      expected: "number",
    })

    assert({
      given: "the artifacts API response",
      should: "have an artifacts array field",
      actual: Array.isArray(data.artifacts),
      expected: true,
    })

    // Verify each artifact has the fields the dashboard relies on
    if (data.artifacts.length > 0) {
      const a = data.artifacts[0]
      assert({
        given: "the first artifact in the list",
        should: "have an id (number)",
        actual: typeof a.id,
        expected: "number",
      })

      assert({
        given: "the first artifact in the list",
        should: "have a name (string)",
        actual: typeof a.name,
        expected: "string",
      })

      assert({
        given: "the first artifact in the list",
        should: "have an archive_download_url (string)",
        actual: typeof a.archive_download_url,
        expected: "string",
      })

      assert({
        given: "the first artifact in the list",
        should: "have a created_at ISO timestamp (string)",
        actual: typeof a.created_at,
        expected: "string",
      })
    }

    // Store for subsequent tests
    firstEtag = res.headers.get("etag")
    artifacts = data.artifacts

    assert({
      given: "the artifacts API response headers",
      should: "include an etag header (non-null string)",
      actual: typeof firstEtag === "string" && firstEtag.length > 0,
      expected: true,
    })
  })

  test("GET /actions/artifacts with If-None-Match returns 304 when unchanged", async (t) => {
    if (!firstEtag) {
      t.skip("No ETag from previous request — cannot test conditional request")
      return
    }

    const res = await ghRequest("GET", "/actions/artifacts?per_page=100", {
      "If-None-Match": firstEtag,
    })

    assert({
      given: "a GET with If-None-Match matching the current ETag",
      should: "return 304 Not Modified",
      actual: res.status,
      expected: 304,
    })
  })

  test("Artifact zip download returns a valid zip that can be extracted", async (t) => {
    // Find a quarantine-results artifact, or any artifact if none match
    const target = artifacts.find((a) => a.name.startsWith("quarantine-results")) ?? artifacts[0]

    if (!target) {
      t.skip("No artifacts available in the test repo — cannot test download")
      return
    }

    // The archive_download_url returns a 302 redirect to blob storage.
    // Node fetch follows redirects by default — the final response should be
    // the zip content.
    const res = await fetch(target.archive_download_url, {
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "application/vnd.github+json",
      },
    })

    assert({
      given: `downloading artifact "${target.name}" (id: ${target.id})`,
      should: "return status 200 (after following redirect)",
      actual: res.status,
      expected: 200,
    })

    const buffer = Buffer.from(await res.arrayBuffer())

    // Verify the response is a valid zip by checking the magic bytes (PK\x03\x04)
    assert({
      given: "the downloaded artifact content",
      should: "start with ZIP magic bytes (PK header)",
      actual: buffer.length >= 4 && buffer[0] === 0x50 && buffer[1] === 0x4b,
      expected: true,
    })

    // Verify adm-zip (the dashboard's zip library) can parse it
    // Import dynamically since adm-zip is a dashboard dependency
    const { default: AdmZip } = await import("adm-zip")
    const zip = new AdmZip(buffer)
    const entries = zip.getEntries()

    assert({
      given: "the downloaded artifact parsed by adm-zip",
      should: "contain at least one entry",
      actual: entries.length >= 1,
      expected: true,
    })

    // If it's a quarantine-results artifact, verify the JSON is parseable
    if (target.name.startsWith("quarantine-results")) {
      const jsonString = entries[0].getData().toString("utf8")
      let parsed = null
      try {
        parsed = JSON.parse(jsonString)
      } catch {
        // Will be caught by the assertion below
      }

      assert({
        given: "the first entry of a quarantine-results artifact",
        should: "be valid JSON",
        actual: parsed !== null,
        expected: true,
      })

      // Verify it has the run_id field the dashboard keys on
      if (parsed) {
        assert({
          given: "the parsed quarantine-results JSON",
          should: "have a run_id string field (upsert key)",
          actual: typeof parsed.run_id,
          expected: "string",
        })
      }
    }
  })
})
