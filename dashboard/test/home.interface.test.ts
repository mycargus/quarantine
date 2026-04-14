/**
 * Interface tests for the home route (GET /).
 *
 * Tests exercise the full request → response path via router.fetch() — routing,
 * controller invocation, and rendering — with no external GitHub API calls
 * (token: "" prevents sync).
 */

import { unlinkSync, writeFileSync } from "node:fs"
import { randomUUID } from "node:crypto"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway"
import { bodyText } from "../app/test-helpers.js"
import { createApp } from "../app/app.js"
import { createTestApp, seedTestDb } from "./helpers.js"

describe("GET / — valid config, empty repos", async (assert) => {
  const { router, cleanup } = createTestApp({ repos: [] })
  try {
    const response = await router.fetch(new Request("http://localhost/"))
    const html = await bodyText(response)

    assert({
      given: "a GET / request with valid config and no repos",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a GET / request with valid config and no repos",
      should: "include 'Quarantine Dashboard' in the response",
      actual: html.includes("Quarantine Dashboard"),
      expected: true,
    })
  } finally {
    cleanup()
  }
})

describe("GET / — valid config, seeded DB with projects", async (assert) => {
  const repos = [
    { owner: "acme", repo: "payments" },
    { owner: "acme", repo: "frontend" },
  ]
  const { router, dbPath, cleanup } = createTestApp({ repos })

  seedTestDb(dbPath, [
    {
      owner: "acme",
      repo: "payments",
      tests: [
        {
          testId: "payments::checkout::should charge card",
          name: "should charge card",
          quarantinedAt: "2026-03-01T00:00:00Z",
          issueUrl: "https://github.com/acme/payments/issues/1",
        },
      ],
    },
    {
      owner: "acme",
      repo: "frontend",
      tests: [
        {
          testId: "frontend::nav::renders",
          name: "renders nav correctly",
          quarantinedAt: "2026-03-02T00:00:00Z",
        },
      ],
    },
  ])

  try {
    const response = await router.fetch(new Request("http://localhost/"))
    const html = await bodyText(response)

    assert({
      given: "a GET / with 2 repos, each with 1 quarantined test",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a GET / with 2 repos, each with 1 quarantined test",
      should: "include 'acme/payments' in the response",
      actual: html.includes("acme/payments"),
      expected: true,
    })

    assert({
      given: "a GET / with 2 repos, each with 1 quarantined test",
      should: "include 'acme/frontend' in the response",
      actual: html.includes("acme/frontend"),
      expected: true,
    })

    assert({
      given: "a GET / with 2 repos, each with 1 quarantined test",
      should: "show total quarantined count of 2",
      actual: html.includes("<strong>2</strong>"),
      expected: true,
    })
  } finally {
    cleanup()
  }
})

describe("GET / — missing config file", async (assert) => {
  const router = createApp({
    configPath: "/nonexistent/path/dashboard.yml",
    dbPath: "/tmp/unused.db",
    token: "",
  })
  const response = await router.fetch(new Request("http://localhost/"))
  const html = await bodyText(response)

  assert({
    given: "a GET / request when the config file does not exist",
    should: "return HTTP 500",
    actual: response.status,
    expected: 500,
  })

  assert({
    given: "a GET / request when the config file does not exist",
    should: "include 'Configuration Error' in the response",
    actual: html.includes("Configuration Error"),
    expected: true,
  })
})

describe("GET / — invalid config content", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-iface-bad-${randomUUID()}.yml`)
  writeFileSync(configPath, "not_a_valid_config: true", "utf8")

  const router = createApp({ configPath, dbPath: "/tmp/unused.db", token: "" })
  try {
    const response = await router.fetch(new Request("http://localhost/"))
    const html = await bodyText(response)

    assert({
      given: "a GET / request with a config file that has invalid content",
      should: "return HTTP 500",
      actual: response.status,
      expected: 500,
    })

    assert({
      given: "a GET / request with a config file that has invalid content",
      should: "include 'Configuration Error' in the response",
      actual: html.includes("Configuration Error"),
      expected: true,
    })
  } finally {
    try {
      unlinkSync(configPath)
    } catch {
      // ignore
    }
  }
})

describe("GET /nonexistent — unknown route", async (assert) => {
  const { router, cleanup } = createTestApp()
  try {
    const response = await router.fetch(new Request("http://localhost/nonexistent"))

    assert({
      given: "a GET request to an unknown route",
      should: "return HTTP 404",
      actual: response.status,
      expected: 404,
    })
  } finally {
    cleanup()
  }
})
