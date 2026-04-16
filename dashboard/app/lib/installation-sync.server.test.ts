import { describe } from "riteway"
import { shouldSyncInstallations, validateAppCredentials } from "./installation-sync.server.js"

const throws = (fn: () => unknown): string | null => {
  try {
    fn()
    return null
  } catch (e) {
    return e instanceof Error ? e.message : String(e)
  }
}

const now = new Date("2026-04-16T12:00:00.000Z")
const FIFTEEN_MIN = 900_000

describe("shouldSyncInstallations()", async (assert) => {
  assert({
    given: "lastSyncedAt is null (no sync has ever occurred)",
    should: "return true",
    actual: shouldSyncInstallations(null, now, FIFTEEN_MIN),
    expected: true,
  })

  {
    const fiveMinAgo = new Date(now.getTime() - 5 * 60_000)
    assert({
      given: "lastSyncedAt is 5 minutes ago (within 15 min interval)",
      should: "return false",
      actual: shouldSyncInstallations(fiveMinAgo, now, FIFTEEN_MIN),
      expected: false,
    })
  }

  {
    const exactlyFifteenMinAgo = new Date(now.getTime() - FIFTEEN_MIN)
    assert({
      given: "now - lastSyncedAt equals intervalMs exactly (900000 ms)",
      should: "return false (stale means strictly greater than the interval)",
      actual: shouldSyncInstallations(exactlyFifteenMinAgo, now, FIFTEEN_MIN),
      expected: false,
    })
  }

  {
    const sixteenMinAgo = new Date(now.getTime() - 16 * 60_000)
    assert({
      given: "lastSyncedAt is 16 minutes ago (beyond 15 min interval)",
      should: "return true",
      actual: shouldSyncInstallations(sixteenMinAgo, now, FIFTEEN_MIN),
      expected: true,
    })
  }
})

describe("validateAppCredentials()", async (assert) => {
  assert({
    given: "both clientId and privateKey are present",
    should: "return validated credentials",
    actual: validateAppCredentials({
      clientId: "Iv1.abc123",
      privateKey: "-----BEGIN RSA PRIVATE KEY-----\nfake\n-----END RSA PRIVATE KEY-----",
    }),
    expected: {
      clientId: "Iv1.abc123",
      privateKey: "-----BEGIN RSA PRIVATE KEY-----\nfake\n-----END RSA PRIVATE KEY-----",
    },
  })

  assert({
    given: "clientId is missing",
    should: "throw an error naming QUARANTINE_APP_CLIENT_ID",
    actual: throws(() =>
      validateAppCredentials({
        clientId: undefined,
        privateKey: "-----BEGIN RSA PRIVATE KEY-----\nfake\n-----END RSA PRIVATE KEY-----",
      }),
    ),
    expected: "Missing required environment variable: QUARANTINE_APP_CLIENT_ID",
  })

  assert({
    given: "privateKey is missing",
    should: "throw an error naming QUARANTINE_APP_PRIVATE_KEY",
    actual: throws(() =>
      validateAppCredentials({
        clientId: "Iv1.abc123",
        privateKey: undefined,
      }),
    ),
    expected: "Missing required environment variable: QUARANTINE_APP_PRIVATE_KEY",
  })

  assert({
    given: "both clientId and privateKey are missing",
    should: "throw an error naming both missing variables",
    actual: throws(() =>
      validateAppCredentials({
        clientId: undefined,
        privateKey: undefined,
      }),
    ),
    expected:
      "Missing required environment variables: QUARANTINE_APP_CLIENT_ID, QUARANTINE_APP_PRIVATE_KEY",
  })

  assert({
    given: "clientId is an empty string",
    should: "throw an error identifying QUARANTINE_APP_CLIENT_ID as present but blank",
    actual: throws(() =>
      validateAppCredentials({
        clientId: "",
        privateKey: "-----BEGIN RSA PRIVATE KEY-----\nfake\n-----END RSA PRIVATE KEY-----",
      }),
    ),
    expected: "QUARANTINE_APP_CLIENT_ID is set but blank",
  })

  assert({
    given: "privateKey is an empty string",
    should: "throw an error identifying QUARANTINE_APP_PRIVATE_KEY as present but blank",
    actual: throws(() =>
      validateAppCredentials({
        clientId: "Iv1.abc123",
        privateKey: "",
      }),
    ),
    expected: "QUARANTINE_APP_PRIVATE_KEY is set but blank",
  })
})
