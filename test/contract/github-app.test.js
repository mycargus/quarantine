/**
 * JS contract tests for the GitHub Apps API (Dashboard consumer).
 *
 * Validates that the dashboard's expected response shapes are served by the
 * Prism mock. These tests read PRISM_URL from the environment — they do NOT
 * spawn their own Prism instance (the shared Prism is started by
 * scripts/run-contract-tests.sh).
 *
 * Covers: create installation access token (201, 401, 403, 404, 422),
 *         list installations (200), list installation repos (200).
 */

import { assert } from "riteway/vitest"
import { describe, test } from "vitest"

const PRISM_URL = process.env.PRISM_URL

if (!PRISM_URL) {
  throw new Error(
    "PRISM_URL is not set — run 'make contract-test' or scripts/run-contract-tests.sh",
  )
}

describe("GitHub Apps API — create installation access token", () => {
  test("POST /app/installations/{id}/access_tokens returns 201 with token and expires_at", async () => {
    const res = await fetch(
      `${PRISM_URL}/app/installations/1/access_tokens`,
      {
        method: "POST",
        headers: {
          Authorization: "Bearer contract-test-token",
          Accept: "application/vnd.github+json",
        },
      },
    )

    assert({
      given: "POST /app/installations/{id}/access_tokens",
      should: "return HTTP 201",
      actual: res.status,
      expected: 201,
    })

    const body = await res.json()

    assert({
      given: "create installation access token 201 response",
      should: "have a token field",
      actual: typeof body.token === "string",
      expected: true,
    })
    assert({
      given: "create installation access token 201 response",
      should: "have an expires_at field",
      actual: typeof body.expires_at === "string",
      expected: true,
    })
  })

  test("POST /app/installations/{id}/access_tokens returns 401 when unauthenticated (Prefer: code=401)", async () => {
    const res = await fetch(
      `${PRISM_URL}/app/installations/1/access_tokens`,
      {
        method: "POST",
        headers: {
          Authorization: "Bearer contract-test-token",
          Accept: "application/vnd.github+json",
          Prefer: "code=401",
        },
      },
    )

    assert({
      given: "POST /app/installations/{id}/access_tokens with Prefer: code=401",
      should: "return HTTP 401",
      actual: res.status,
      expected: 401,
    })

    const body = await res.json()
    assert({
      given: "unauthenticated 401 response",
      should: "have a message field",
      actual: typeof body.message === "string",
      expected: true,
    })
  })

  test("POST /app/installations/{id}/access_tokens returns 403 when integration lacks access (Prefer: code=403)", async () => {
    const res = await fetch(
      `${PRISM_URL}/app/installations/1/access_tokens`,
      {
        method: "POST",
        headers: {
          Authorization: "Bearer contract-test-token",
          Accept: "application/vnd.github+json",
          Prefer: "code=403",
        },
      },
    )

    assert({
      given: "POST /app/installations/{id}/access_tokens with Prefer: code=403",
      should: "return HTTP 403",
      actual: res.status,
      expected: 403,
    })

    const body = await res.json()
    assert({
      given: "forbidden 403 response",
      should: "have a message about integration access",
      actual: typeof body.message === "string",
      expected: true,
    })
  })

  test("POST /app/installations/{id}/access_tokens returns 404 when installation not found (Prefer: code=404)", async () => {
    const res = await fetch(
      `${PRISM_URL}/app/installations/1/access_tokens`,
      {
        method: "POST",
        headers: {
          Authorization: "Bearer contract-test-token",
          Accept: "application/vnd.github+json",
          Prefer: "code=404",
        },
      },
    )

    assert({
      given: "POST /app/installations/{id}/access_tokens with Prefer: code=404",
      should: "return HTTP 404",
      actual: res.status,
      expected: 404,
    })

    const body = await res.json()
    assert({
      given: "not found 404 response",
      should: "have a message field",
      actual: typeof body.message === "string",
      expected: true,
    })
  })

  test("POST /app/installations/{id}/access_tokens returns 422 on validation error (Prefer: code=422)", async () => {
    const res = await fetch(
      `${PRISM_URL}/app/installations/1/access_tokens`,
      {
        method: "POST",
        headers: {
          Authorization: "Bearer contract-test-token",
          Accept: "application/vnd.github+json",
          Prefer: "code=422",
        },
      },
    )

    assert({
      given: "POST /app/installations/{id}/access_tokens with Prefer: code=422",
      should: "return HTTP 422",
      actual: res.status,
      expected: 422,
    })

    const body = await res.json()
    assert({
      given: "validation error 422 response",
      should: "have a message field",
      actual: typeof body.message === "string",
      expected: true,
    })
  })
})

describe("GitHub Apps API — list installations", () => {
  test("GET /app/installations returns 200 with an array of installations", async () => {
    const res = await fetch(`${PRISM_URL}/app/installations`, {
      headers: {
        Authorization: "Bearer contract-test-token",
        Accept: "application/vnd.github+json",
      },
    })

    assert({
      given: "GET /app/installations",
      should: "return HTTP 200",
      actual: res.status,
      expected: 200,
    })

    const body = await res.json()

    assert({
      given: "list installations 200 response",
      should: "return an array",
      actual: Array.isArray(body),
      expected: true,
    })
    assert({
      given: "list installations 200 response",
      should: "return at least one installation",
      actual: body.length > 0,
      expected: true,
    })

    const first = body[0]
    assert({
      given: "first installation in list response",
      should: "have a numeric id",
      actual: typeof first.id === "number",
      expected: true,
    })
    assert({
      given: "first installation in list response",
      should: "have an account object",
      actual: typeof first.account === "object" && first.account !== null,
      expected: true,
    })
    assert({
      given: "first installation in list response",
      should: "have suspended_at field (string or null)",
      actual: first.suspended_at === null || typeof first.suspended_at === "string",
      expected: true,
    })
  })
})

describe("GitHub Apps API — list installation repositories", () => {
  test("GET /installation/repositories returns 200 with total_count and repositories array", async () => {
    const res = await fetch(`${PRISM_URL}/installation/repositories`, {
      headers: {
        Authorization: "Bearer contract-test-token",
        Accept: "application/vnd.github+json",
      },
    })

    assert({
      given: "GET /installation/repositories",
      should: "return HTTP 200",
      actual: res.status,
      expected: 200,
    })

    const body = await res.json()

    assert({
      given: "list installation repos 200 response",
      should: "have a total_count field",
      actual: typeof body.total_count === "number",
      expected: true,
    })
    assert({
      given: "list installation repos 200 response",
      should: "have a repositories array",
      actual: Array.isArray(body.repositories),
      expected: true,
    })
    assert({
      given: "list installation repos 200 response",
      should: "return at least one repository",
      actual: body.repositories.length > 0,
      expected: true,
    })

    const first = body.repositories[0]
    assert({
      given: "first repository in installation repos response",
      should: "have a numeric id",
      actual: typeof first.id === "number",
      expected: true,
    })
    assert({
      given: "first repository in installation repos response",
      should: "have a string full_name",
      actual: typeof first.full_name === "string",
      expected: true,
    })
  })
})
