import { unlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway"
import { initDb } from "../lib/db.server.js"
import { bodyText } from "../test-helpers.js"
import { home } from "./home.js"

describe("home() — missing config", async (assert) => {
  const response = await home({ configPath: "/nonexistent/dashboard.yml" })
  const html = await bodyText(response)

  assert({
    given: "a config path that does not exist",
    should: "return HTTP 500",
    actual: response.status,
    expected: 500,
  })

  assert({
    given: "a config path that does not exist",
    should: "include 'Configuration Error' in the body",
    actual: html.includes("Configuration Error"),
    expected: true,
  })

  assert({
    given: "a config path that does not exist",
    should: "include the missing file path in the body",
    actual: html.includes("/nonexistent/dashboard.yml"),
    expected: true,
  })
})

describe("home() — invalid config", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-test-bad-${Date.now()}.yml`)
  writeFileSync(configPath, "not: valid\nconfig: true", "utf8")

  try {
    const response = await home({ configPath })
    const html = await bodyText(response)

    assert({
      given: "a config file with invalid content (missing source/repos)",
      should: "return HTTP 500",
      actual: response.status,
      expected: 500,
    })

    assert({
      given: "a config file with invalid content",
      should: "include 'Configuration Error' in the body",
      actual: html.includes("Configuration Error"),
      expected: true,
    })
  } finally {
    unlinkSync(configPath)
  }
})

describe("home() — valid config, empty repos", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-test-ok-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-test-${Date.now()}.db`)
  writeFileSync(configPath, "source: manual\nrepos: []", "utf8")

  try {
    const response = await home({ configPath, dbPath })
    const html = await bodyText(response)

    assert({
      given: "a valid config with empty repos list",
      should: "return HTTP 200",
      actual: response.status,
      expected: 200,
    })

    assert({
      given: "a valid config with empty repos list",
      should: "return HTML with content-type header",
      actual: response.headers.get("Content-Type"),
      expected: "text/html; charset=utf-8",
    })

    assert({
      given: "a valid config with empty repos list",
      should: "include the page title",
      actual: html.includes("Quarantine Dashboard"),
      expected: true,
    })

    assert({
      given: "a valid config with empty repos list",
      should: "include the Projects heading",
      actual: html.includes("Projects"),
      expected: true,
    })
  } finally {
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})

describe("home() — org overview: total quarantined count", async (assert) => {
  const configPath = join(tmpdir(), `dashboard-overview-${Date.now()}.yml`)
  const dbPath = join(tmpdir(), `dashboard-overview-db-${Date.now()}.db`)
  writeFileSync(
    configPath,
    "source: manual\nrepos:\n  - owner: acme\n    repo: payments-service\n  - owner: acme\n    repo: frontend",
    "utf8",
  )

  // Seed the DB before the handler runs (WAL mode supports concurrent connections).
  const { raw } = initDb(dbPath)
  const stmt = raw.prepare("INSERT INTO projects (owner, repo) VALUES (?, ?)")
  stmt.run("acme", "payments-service")
  stmt.run("acme", "frontend")
  const ids = raw.prepare("SELECT id, owner, repo FROM projects").all() as {
    id: number
    owner: string
    repo: string
  }[]
  const pid1 = ids.find((r) => r.repo === "payments-service")!.id
  const pid2 = ids.find((r) => r.repo === "frontend")!.id
  raw
    .prepare(
      "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at, issue_url) VALUES (?, ?, ?, ?, ?)",
    )
    .run(
      pid1,
      "t1",
      "payment test",
      "2026-03-01T00:00:00Z",
      "https://github.com/acme/payments-service/issues/1",
    )
  raw
    .prepare(
      "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at, issue_url) VALUES (?, ?, ?, ?, ?)",
    )
    .run(
      pid2,
      "f1",
      "nav test",
      "2026-03-02T00:00:00Z",
      "https://github.com/acme/frontend/issues/1",
    )
  raw.close()

  try {
    const response = await home({ configPath, dbPath })
    const html = await bodyText(response)

    assert({
      given: "2 repos with 1 quarantined test each",
      should: "include the total quarantined count wrapped in <strong>",
      actual: html.includes("<strong>2</strong>"),
      expected: true,
    })

    assert({
      given: "2 repos with quarantined tests",
      should: "include the payments-service repo name",
      actual: html.includes("payments-service"),
      expected: true,
    })

    assert({
      given: "2 repos with quarantined tests",
      should: "include the frontend repo name",
      actual: html.includes("frontend"),
      expected: true,
    })

    assert({
      given: "2 repos with quarantined tests",
      should: "include a link to each project details page",
      actual:
        html.includes("/projects/acme/payments-service") &&
        html.includes("/projects/acme/frontend"),
      expected: true,
    })

    assert({
      given: "a repo with a recently quarantined test",
      should: "include the most recently quarantined test name",
      actual: html.includes("nav test"),
      expected: true,
    })
  } finally {
    unlinkSync(configPath)
    try {
      unlinkSync(dbPath)
    } catch {
      // db may not have been created
    }
  }
})
