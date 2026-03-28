import { unlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { describe } from "riteway/esm"
import { loadConfig, parseConfig } from "./config.server.js"

const throws = (fn: () => unknown): string | null => {
  try {
    fn()
    return null
  } catch (e) {
    return e instanceof Error ? e.message : String(e)
  }
}

describe("parseConfig()", async (assert) => {
  const fullYaml = `
source: manual
repos:
  - owner: mycargus
    repo: my-app
  - owner: acme
    repo: payments-service
poll_interval: 300
`.trim()

  assert({
    given: "valid YAML with all fields",
    should: 'return source as "manual"',
    actual: parseConfig(fullYaml).source,
    expected: "manual",
  })

  assert({
    given: "valid YAML with all fields",
    should: "return two repos",
    actual: parseConfig(fullYaml).repos,
    expected: [
      { owner: "mycargus", repo: "my-app" },
      { owner: "acme", repo: "payments-service" },
    ],
  })

  assert({
    given: "valid YAML with all fields",
    should: "return poll_interval as 300",
    actual: parseConfig(fullYaml).poll_interval,
    expected: 300,
  })

  assert({
    given: "YAML missing poll_interval",
    should: "default poll_interval to 300",
    actual: parseConfig("source: manual\nrepos:\n  - owner: foo\n    repo: bar").poll_interval,
    expected: 300,
  })

  assert({
    given: "YAML with poll_interval set to 0",
    should: "return poll_interval as 0, not the default 300",
    actual: parseConfig("source: manual\nrepos:\n  - owner: foo\n    repo: bar\npoll_interval: 0")
      .poll_interval,
    expected: 0,
  })

  assert({
    given: "YAML with poll_interval as a string value",
    should: "default poll_interval to 300",
    actual: parseConfig(
      `source: manual\nrepos:\n  - owner: foo\n    repo: bar\npoll_interval: "fast"`,
    ).poll_interval,
    expected: 300,
  })

  assert({
    given: 'YAML missing required field "source"',
    should: "throw an error identifying the missing field",
    actual: throws(() => parseConfig("repos:\n  - owner: foo\n    repo: bar"))?.includes("source"),
    expected: true,
  })

  assert({
    given: 'YAML missing required field "repos"',
    should: "throw an error identifying the missing field",
    actual: throws(() => parseConfig("source: manual"))?.includes("repos"),
    expected: true,
  })

  assert({
    given: "YAML with source set to an empty string",
    should: "throw an error identifying the missing field",
    actual: throws(() =>
      parseConfig(`source: ''\nrepos:\n  - owner: foo\n    repo: bar`),
    )?.includes("source"),
    expected: true,
  })

  assert({
    given: "an empty string",
    should: "throw an error identifying a missing required field",
    actual: throws(() => parseConfig("")) !== null,
    expected: true,
  })

  assert({
    given: "malformed YAML (unbalanced brackets)",
    should: "throw a parse error",
    actual: throws(() => parseConfig("source: manual\nrepos: [unclosed")) !== null,
    expected: true,
  })
})

describe("parseConfig() — edge cases", async (assert) => {
  assert({
    given: "YAML with repos as an empty array",
    should: "return repos as empty array",
    actual: parseConfig("source: manual\nrepos: []").repos,
    expected: [],
  })

  assert({
    given: "YAML with source set to an unrecognized value",
    should: "return source as the provided string (no runtime validation)",
    actual: parseConfig("source: automatic\nrepos:\n  - owner: foo\n    repo: bar").source,
    expected: "automatic" as "manual",
  })

  assert({
    given: "YAML with a repo entry missing the repo field",
    should: "return the entry as-is (no field-level validation)",
    actual: parseConfig("source: manual\nrepos:\n  - owner: foo").repos,
    expected: [{ owner: "foo" }] as ReturnType<typeof parseConfig>["repos"],
  })
})

describe("loadConfig()", async (assert) => {
  const filePath = join(tmpdir(), `config-test-${Date.now()}.yml`)
  const yaml = "source: manual\nrepos:\n  - owner: testowner\n    repo: testrepo\npoll_interval: 60"
  writeFileSync(filePath, yaml, "utf8")

  try {
    assert({
      given: "a valid config file on disk",
      should: "return the parsed DashboardConfig",
      actual: loadConfig(filePath),
      expected: {
        source: "manual",
        repos: [{ owner: "testowner", repo: "testrepo" }],
        poll_interval: 60,
      },
    })
  } finally {
    unlinkSync(filePath)
  }

  assert({
    given: "a non-existent file path",
    should: "throw a file system error",
    actual: throws(() => loadConfig("/nonexistent/path/config.yml")) !== null,
    expected: true,
  })
})
