import { describe } from "riteway"
import { listStateSuites, readSuiteState } from "./state-sync.server.js"

type FetchFn = typeof fetch

// Fixture: 3 quarantined tests for backend suite
const backendState = {
  version: 1,
  updated_at: "2026-04-14T10:00:00Z",
  tests: {
    "auth/login.test.ts::LoginSuite::logs in successfully": {
      test_id: "auth/login.test.ts::LoginSuite::logs in successfully",
      file_path: "auth/login.test.ts",
      classname: "LoginSuite",
      name: "logs in successfully",
      suite: "backend",
      first_flaky_at: "2026-04-01T08:00:00Z",
      last_failure_at: "2026-04-13T08:00:00Z",
      flaky_count: 5,
      quarantined_at: "2026-04-01T09:00:00Z",
      quarantined_by: "cli-auto",
    },
    "auth/logout.test.ts::LogoutSuite::logs out correctly": {
      test_id: "auth/logout.test.ts::LogoutSuite::logs out correctly",
      file_path: "auth/logout.test.ts",
      classname: "LogoutSuite",
      name: "logs out correctly",
      suite: "backend",
      first_flaky_at: "2026-04-02T08:00:00Z",
      last_failure_at: "2026-04-12T08:00:00Z",
      flaky_count: 3,
      quarantined_at: "2026-04-02T09:00:00Z",
      quarantined_by: "cli-auto",
    },
    "db/connection.test.ts::DBSuite::connects on startup": {
      test_id: "db/connection.test.ts::DBSuite::connects on startup",
      file_path: "db/connection.test.ts",
      classname: "DBSuite",
      name: "connects on startup",
      suite: "backend",
      first_flaky_at: "2026-04-03T08:00:00Z",
      last_failure_at: "2026-04-11T08:00:00Z",
      flaky_count: 2,
      quarantined_at: "2026-04-03T09:00:00Z",
      quarantined_by: "cli-auto",
    },
  },
}

// Fixture: 1 quarantined test for frontend suite
const frontendState = {
  version: 1,
  updated_at: "2026-04-14T11:00:00Z",
  tests: {
    "components/Button.test.ts::ButtonSuite::renders correctly": {
      test_id: "components/Button.test.ts::ButtonSuite::renders correctly",
      file_path: "components/Button.test.ts",
      classname: "ButtonSuite",
      name: "renders correctly",
      suite: "frontend",
      first_flaky_at: "2026-04-05T08:00:00Z",
      last_failure_at: "2026-04-10T08:00:00Z",
      flaky_count: 1,
      quarantined_at: "2026-04-05T09:00:00Z",
      quarantined_by: "cli-auto",
    },
  },
}

// Directory listing response with backend/ and frontend/ subdirectories
const directoryListingResponse = [
  { name: "backend", path: ".quarantine/backend", type: "dir", sha: "abc111" },
  { name: "frontend", path: ".quarantine/frontend", type: "dir", sha: "abc222" },
  { name: "config.yml", path: ".quarantine/config.yml", type: "file", sha: "abc333" },
]

function makeDirectoryFetch200(): FetchFn {
  return (async () => ({
    ok: true,
    status: 200,
    json: async () => directoryListingResponse,
  })) as unknown as FetchFn
}

function makeFetch404(): FetchFn {
  return (async () => ({
    ok: false,
    status: 404,
    json: async () => ({ message: "Not Found" }),
  })) as unknown as FetchFn
}

function makeStateFileFetch200(state: object): FetchFn {
  const encoded = Buffer.from(JSON.stringify(state)).toString("base64")
  return (async () => ({
    ok: true,
    status: 200,
    json: async () => ({ content: encoded, sha: "def456" }),
  })) as unknown as FetchFn
}

function makeFetch500(): FetchFn {
  return (async () => ({
    ok: false,
    status: 500,
    json: async () => ({ message: "Internal Server Error" }),
  })) as unknown as FetchFn
}

const throws = async (fn: () => Promise<unknown>): Promise<boolean> => {
  try {
    await fn()
    return false
  } catch {
    return true
  }
}

describe("listStateSuites()", async (assert) => {
  assert({
    given: "a 200 response with 2 dir entries (backend/, frontend/) and 1 file entry (config.yml)",
    should: "return only the directory names ['backend', 'frontend'] — file entries excluded",
    actual: await listStateSuites(
      "mycargus",
      "my-app",
      "token",
      "quarantine/state",
      makeDirectoryFetch200(),
    ),
    expected: ["backend", "frontend"],
  })

  assert({
    given: "a 404 response (directory doesn't exist)",
    should: "return empty array without throwing",
    actual: await listStateSuites(
      "mycargus",
      "my-app",
      "token",
      "quarantine/state",
      makeFetch404(),
    ),
    expected: [],
  })

  assert({
    given: "a 500 response (GitHub error)",
    should: "throw — non-404 errors propagate to the caller (syncRepo handles via try/catch)",
    actual: await throws(() =>
      listStateSuites("mycargus", "my-app", "token", "quarantine/state", makeFetch500()),
    ),
    expected: true,
  })
})

describe("readSuiteState()", async (assert) => {
  assert({
    given: "a 200 response with base64-encoded backend state JSON containing 3 tests",
    should: "return parsed QuarantineState with 3 test entries",
    actual: Object.keys(
      (
        await readSuiteState(
          "mycargus",
          "my-app",
          "token",
          "quarantine/state",
          "backend",
          makeStateFileFetch200(backendState),
        )
      )?.tests ?? {},
    ).length,
    expected: 3,
  })

  assert({
    given: "a 200 response with base64-encoded backend state JSON",
    should: "correctly decode the flaky_count field of the first test entry",
    actual: (
      await readSuiteState(
        "mycargus",
        "my-app",
        "token",
        "quarantine/state",
        "backend",
        makeStateFileFetch200(backendState),
      )
    )?.tests["auth/login.test.ts::LoginSuite::logs in successfully"]?.flaky_count,
    expected: 5,
  })

  assert({
    given: "a 200 response with base64-encoded frontend state JSON containing 1 test",
    should: "return parsed QuarantineState with 1 test entry",
    actual: Object.keys(
      (
        await readSuiteState(
          "mycargus",
          "my-app",
          "token",
          "quarantine/state",
          "frontend",
          makeStateFileFetch200(frontendState),
        )
      )?.tests ?? {},
    ).length,
    expected: 1,
  })

  assert({
    given: "a 200 response with base64-encoded backend state JSON",
    should: "return the correct version field",
    actual: (
      await readSuiteState(
        "mycargus",
        "my-app",
        "token",
        "quarantine/state",
        "backend",
        makeStateFileFetch200(backendState),
      )
    )?.version,
    expected: 1,
  })

  assert({
    given: "a 404 response (state file doesn't exist)",
    should: "return null without throwing",
    actual: await readSuiteState(
      "mycargus",
      "my-app",
      "token",
      "quarantine/state",
      "frontend",
      makeFetch404(),
    ),
    expected: null,
  })
})
