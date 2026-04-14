/**
 * Interface tests for the project detail route (GET /projects/:owner/:repo).
 *
 * Tests exercise the full request → response path via router.fetch() — route
 * parameter extraction, controller invocation, filtering, and rendering —
 * with no external GitHub API calls (no sync on this route).
 */

import { describe } from "riteway"
import { bodyText } from "../app/test-helpers.js"
import { createTestApp, seedTestDb } from "./helpers.js"

describe("GET /projects/:owner/:repo — unknown project", async (assert) => {
  const { router, cleanup } = createTestApp({ repos: [{ owner: "acme", repo: "missing" }] })
  try {
    const response = await router.fetch(
      new Request("http://localhost/projects/acme/missing"),
    )
    const html = await bodyText(response)

    assert({
      given: "a GET /projects/:owner/:repo for a project not in the DB",
      should: "return HTTP 404",
      actual: response.status,
      expected: 404,
    })

    assert({
      given: "a GET /projects/:owner/:repo for a project not in the DB",
      should: "include 'Not Found' in the response",
      actual: html.includes("Not Found"),
      expected: true,
    })
  } finally {
    cleanup()
  }
})

describe("GET /projects/:owner/:repo — known project, no test results", async (assert) => {
  const repos = [{ owner: "acme", repo: "empty-repo" }]
  const { router, dbPath, cleanup } = createTestApp({ repos })

  seedTestDb(dbPath, [{ owner: "acme", repo: "empty-repo" }])

  try {
    const response = await router.fetch(
      new Request("http://localhost/projects/acme/empty-repo"),
    )
    const html = await bodyText(response)

    assert({
      given: "a GET /projects/:owner/:repo for a project with no quarantined tests",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a GET /projects/:owner/:repo for a project with no quarantined tests",
      should: "include the repo name in the response",
      actual: html.includes("acme/empty-repo"),
      expected: true,
    })

    assert({
      given: "a GET /projects/:owner/:repo for a project with no quarantined tests",
      should: "show 0 quarantined tests",
      actual: html.includes("Showing 0 of 0"),
      expected: true,
    })
  } finally {
    cleanup()
  }
})

describe("GET /projects/:owner/:repo — known project with quarantined tests", async (assert) => {
  const repos = [{ owner: "acme", repo: "payments" }]
  const { router, dbPath, cleanup } = createTestApp({ repos })

  seedTestDb(dbPath, [
    {
      owner: "acme",
      repo: "payments",
      tests: [
        {
          testId: "payments::checkout::charge card",
          name: "should charge card",
          quarantinedAt: "2026-03-01T00:00:00Z",
          issueUrl: "https://github.com/acme/payments/issues/42",
        },
        {
          testId: "payments::refund::process refund",
          name: "should process refund",
          quarantinedAt: "2026-03-05T00:00:00Z",
        },
      ],
    },
  ])

  try {
    const response = await router.fetch(
      new Request("http://localhost/projects/acme/payments"),
    )
    const html = await bodyText(response)

    assert({
      given: "a GET /projects/:owner/:repo for a project with 2 quarantined tests",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a GET /projects/:owner/:repo for a project with 2 quarantined tests",
      should: "include the first test name",
      actual: html.includes("should charge card"),
      expected: true,
    })

    assert({
      given: "a GET /projects/:owner/:repo for a project with 2 quarantined tests",
      should: "include the second test name",
      actual: html.includes("should process refund"),
      expected: true,
    })

    assert({
      given: "a GET /projects/:owner/:repo for a project with 2 quarantined tests",
      should: "show total count of 2",
      actual: html.includes("Showing 2 of 2"),
      expected: true,
    })
  } finally {
    cleanup()
  }
})

describe("GET /projects/:owner/:repo — route parameter extraction", async (assert) => {
  const repos = [{ owner: "my-org", repo: "my-service" }]
  const { router, dbPath, cleanup } = createTestApp({ repos })

  seedTestDb(dbPath, [{ owner: "my-org", repo: "my-service" }])

  try {
    const response = await router.fetch(
      new Request("http://localhost/projects/my-org/my-service"),
    )
    const html = await bodyText(response)

    assert({
      given: "a GET /projects/:owner/:repo with hyphenated owner and repo names",
      should: "return HTTP 200 (route params correctly extracted)",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a GET /projects/:owner/:repo with hyphenated owner and repo names",
      should: "include the correct owner/repo in the page title",
      actual: html.includes("my-org/my-service"),
      expected: true,
    })
  } finally {
    cleanup()
  }
})

describe("GET /projects/:owner/:repo?search= — query parameter filtering", async (assert) => {
  const repos = [{ owner: "acme", repo: "search-test" }]
  const { router, dbPath, cleanup } = createTestApp({ repos })

  seedTestDb(dbPath, [
    {
      owner: "acme",
      repo: "search-test",
      tests: [
        {
          testId: "test::checkout::charge",
          name: "checkout: should charge card",
          quarantinedAt: "2026-03-01T00:00:00Z",
        },
        {
          testId: "test::login::auth",
          name: "login: should authenticate user",
          quarantinedAt: "2026-03-02T00:00:00Z",
        },
      ],
    },
  ])

  try {
    const response = await router.fetch(
      new Request("http://localhost/projects/acme/search-test?search=checkout"),
    )
    const html = await bodyText(response)

    assert({
      given: "a GET /projects/:owner/:repo?search=checkout with 2 tests, 1 matching",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a GET /projects/:owner/:repo?search=checkout with 2 tests, 1 matching",
      should: "show 1 of 2 tests after filtering",
      actual: html.includes("Showing 1 of 2"),
      expected: true,
    })

    assert({
      given: "a GET /projects/:owner/:repo?search=checkout with 2 tests, 1 matching",
      should: "include the matching test name",
      actual: html.includes("checkout: should charge card"),
      expected: true,
    })

    assert({
      given: "a GET /projects/:owner/:repo?search=checkout with 2 tests, 1 matching",
      should: "exclude the non-matching test name",
      actual: html.includes("login: should authenticate user"),
      expected: false,
    })
  } finally {
    cleanup()
  }
})
