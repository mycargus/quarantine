import { describe } from "riteway"
import { formatAuthEvent, validateOAuthEnv } from "./auth.server.js"

const throws = (fn: () => unknown): string | null => {
  try {
    fn()
    return null
  } catch (e) {
    return e instanceof Error ? e.message : String(e)
  }
}

describe("validateOAuthEnv()", async (assert) => {
  assert({
    given: "all three env vars present",
    should: "return a validated config with clientId, clientSecret, and origin",
    actual: validateOAuthEnv({
      clientId: "my-client-id",
      clientSecret: "my-client-secret",
      origin: "https://example.com",
    }),
    expected: {
      clientId: "my-client-id",
      clientSecret: "my-client-secret",
      origin: "https://example.com",
    },
  })

  assert({
    given: "QUARANTINE_APP_CLIENT_ID is missing",
    should: "throw an error naming QUARANTINE_APP_CLIENT_ID",
    actual: throws(() =>
      validateOAuthEnv({
        clientId: undefined,
        clientSecret: "my-client-secret",
        origin: "https://example.com",
      }),
    ),
    expected: "Missing required environment variable: QUARANTINE_APP_CLIENT_ID",
  })

  assert({
    given: "QUARANTINE_APP_CLIENT_SECRET is missing",
    should: "throw an error naming QUARANTINE_APP_CLIENT_SECRET",
    actual: throws(() =>
      validateOAuthEnv({
        clientId: "my-client-id",
        clientSecret: undefined,
        origin: "https://example.com",
      }),
    ),
    expected: "Missing required environment variable: QUARANTINE_APP_CLIENT_SECRET",
  })

  assert({
    given: "QUARANTINE_APP_ORIGIN is missing",
    should: "throw an error naming QUARANTINE_APP_ORIGIN",
    actual: throws(() =>
      validateOAuthEnv({
        clientId: "my-client-id",
        clientSecret: "my-client-secret",
        origin: undefined,
      }),
    ),
    expected: "Missing required environment variable: QUARANTINE_APP_ORIGIN",
  })

  assert({
    given: "multiple env vars are missing",
    should: "throw an error naming all missing variables",
    actual: throws(() =>
      validateOAuthEnv({
        clientId: undefined,
        clientSecret: undefined,
        origin: undefined,
      }),
    ),
    expected:
      "Missing required environment variables: QUARANTINE_APP_CLIENT_ID, QUARANTINE_APP_CLIENT_SECRET, QUARANTINE_APP_ORIGIN",
  })
})

describe("formatAuthEvent()", async (assert) => {
  assert({
    given: "a login event with userId and timestamp",
    should: "return a formatted log string containing [auth], event, userId, and timestamp",
    actual: formatAuthEvent("login", "octocat", "2026-04-15T12:00:00.000Z"),
    expected: "[auth] login: userId=octocat at=2026-04-15T12:00:00.000Z",
  })
})
