/**
 * Interface tests for the OAuth flow and related auth endpoints.
 *
 * Tests exercise the full request -> response path via router.fetch() with
 * no external GitHub API calls (token: "" prevents sync).
 */

import { describe } from "riteway"
import { createTestApp } from "./helpers.js"

describe("GET /health — no authentication provided", async (assert) => {
  const { router, cleanup } = createTestApp({ repos: [] })
  try {
    const response = await router.fetch(new Request("http://localhost/health"))

    assert({
      given: "no authentication",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })
  } finally {
    cleanup()
  }
})

describe("GET / — no session cookie (unauthenticated)", async (assert) => {
  const { router, cleanup } = createTestApp({ repos: [] })
  try {
    const response = await router.fetch(new Request("http://localhost/"))

    assert({
      given: "no session cookie on GET /",
      should: "return HTTP 401",
      actual: response.status,
      expected: 401,
    })
  } finally {
    cleanup()
  }
})
