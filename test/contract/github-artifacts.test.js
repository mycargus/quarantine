/**
 * JS contract tests for the GitHub Artifacts API (Dashboard consumer).
 *
 * Validates that the dashboard's expected response shapes are served by the
 * Prism mock. These tests read PRISM_URL from the environment — they do NOT
 * spawn their own Prism instance (the shared Prism is started by
 * scripts/run-contract-tests.sh).
 *
 * Covers: list artifacts (200), download artifact (302), expired artifact (410).
 */

import { assert } from "riteway/vitest"
import { describe, test } from "vitest"

const PRISM_URL = process.env.PRISM_URL

if (!PRISM_URL) {
  throw new Error(
    "PRISM_URL is not set — run 'make contract-test' or scripts/run-contract-tests.sh",
  )
}

const BASE = `${PRISM_URL}/repos/octo-org/octo-docs`

describe("GitHub Artifacts API — list artifacts", () => {
  test("GET /actions/artifacts returns 200 with total_count and artifacts array", async () => {
    const res = await fetch(`${BASE}/actions/artifacts`, {
      headers: {
        Authorization: "Bearer contract-test-token",
        Accept: "application/vnd.github+json",
      },
    })

    assert({
      given: "GET /repos/{owner}/{repo}/actions/artifacts",
      should: "return HTTP 200",
      actual: res.status,
      expected: 200,
    })

    const body = await res.json()

    assert({
      given: "list artifacts 200 response",
      should: "have a total_count field",
      actual: typeof body.total_count === "number",
      expected: true,
    })
    assert({
      given: "list artifacts 200 response",
      should: "have an artifacts array",
      actual: Array.isArray(body.artifacts),
      expected: true,
    })
  })

  test("artifacts array items have required fields: id, name, archive_download_url, created_at, expires_at", async () => {
    const res = await fetch(`${BASE}/actions/artifacts`, {
      headers: {
        Authorization: "Bearer contract-test-token",
        Accept: "application/vnd.github+json",
      },
    })
    const body = await res.json()

    // If the example returns artifacts, each must have the required fields.
    // The spec example always returns 2 artifacts.
    assert({
      given: "list artifacts 200 response",
      should: "return at least one artifact in the example",
      actual: body.artifacts.length > 0,
      expected: true,
    })

    const first = body.artifacts[0]
    assert({
      given: "first artifact in list response",
      should: "have a numeric id",
      actual: typeof first.id === "number",
      expected: true,
    })
    assert({
      given: "first artifact in list response",
      should: "have a string name",
      actual: typeof first.name === "string",
      expected: true,
    })
    assert({
      given: "first artifact in list response",
      should: "have a string archive_download_url",
      actual: typeof first.archive_download_url === "string",
      expected: true,
    })
    assert({
      given: "first artifact in list response",
      should: "have a created_at field (string or null)",
      actual: first.created_at === null || typeof first.created_at === "string",
      expected: true,
    })
    assert({
      given: "first artifact in list response",
      should: "have an expires_at field (string or null)",
      actual: first.expires_at === null || typeof first.expires_at === "string",
      expected: true,
    })
  })
})

describe("GitHub Artifacts API — download artifact", () => {
  test("GET /actions/artifacts/{id}/zip returns 302 with Location header", async () => {
    // Prism returns 302 by default for the download endpoint.
    const res = await fetch(`${BASE}/actions/artifacts/11/zip`, {
      headers: {
        Authorization: "Bearer contract-test-token",
        Accept: "application/vnd.github+json",
      },
      redirect: "manual", // don't follow the redirect
    })

    assert({
      given: "GET /repos/{owner}/{repo}/actions/artifacts/{id}/zip",
      should: "return HTTP 302",
      actual: res.status,
      expected: 302,
    })

    assert({
      given: "download artifact 302 response",
      should: "include a Location header",
      actual: res.headers.has("location"),
      expected: true,
    })
  })

  test("GET /actions/artifacts/{id}/zip returns 410 when artifact has expired (Prefer: code=410)", async () => {
    const res = await fetch(`${BASE}/actions/artifacts/11/zip`, {
      headers: {
        Authorization: "Bearer contract-test-token",
        Accept: "application/vnd.github+json",
        Prefer: "code=410",
      },
      redirect: "manual",
    })

    assert({
      given: "GET /actions/artifacts/{id}/zip with Prefer: code=410 (expired artifact)",
      should: "return HTTP 410",
      actual: res.status,
      expected: 410,
    })

    const body = await res.json()
    assert({
      given: "expired artifact 410 response",
      should: "have a message field",
      actual: typeof body.message === "string",
      expected: true,
    })
  })
})
