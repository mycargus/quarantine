import { describe } from "riteway"
import {
  shouldSyncInstallations,
  startDiscoveryLoop,
  startupSyncWithTimeout,
  validateAppCredentials,
} from "./installation-sync.server.js"

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

describe("startupSyncWithTimeout() — success path", async (assert) => {
  const logs: string[] = []
  const terminateCalls: number[] = []

  await startupSyncWithTimeout(
    () => Promise.resolve(),
    60_000,
    (msg) => logs.push(msg),
    (code) => terminateCalls.push(code),
  )

  assert({
    given: "syncFn resolves quickly and timeoutMs is very large",
    should: "not call terminate",
    actual: terminateCalls.length,
    expected: 0,
  })
})

describe("startupSyncWithTimeout() — timeout exit code", async (assert) => {
  const logs: string[] = []
  const terminateCalls: number[] = []

  let resolveTerminate!: () => void
  const terminateCalled = new Promise<void>((resolve) => {
    resolveTerminate = resolve
  })

  const neverResolves = new Promise<void>(() => {
    // intentionally never resolves
  })

  startupSyncWithTimeout(
    () => neverResolves,
    0,
    (msg) => logs.push(msg),
    (code) => {
      terminateCalls.push(code)
      resolveTerminate()
    },
  ).catch(() => {
    // timeout throws after terminate — ignore
  })

  await terminateCalled

  assert({
    given: "syncFn never resolves and timeoutMs is 0",
    should: "call terminate with exit code 1",
    actual: terminateCalls[0],
    expected: 1,
  })
})

describe("startupSyncWithTimeout() — timer cleared on success", async (assert) => {
  const terminateCalls: number[] = []

  await startupSyncWithTimeout(
    () => Promise.resolve(),
    1,
    () => {},
    (code) => terminateCalls.push(code),
  )

  // Wait longer than the timeout to confirm timer was cleared
  await new Promise((r) => setTimeout(r, 20))

  assert({
    given: "syncFn resolves before a 1ms timeout and we wait 20ms afterward",
    should: "not call terminate (timer was cleared)",
    actual: terminateCalls.length,
    expected: 0,
  })
})

describe("startDiscoveryLoop() — signal terminate exit code", async (assert) => {
  const terminateCalls: number[] = []

  startDiscoveryLoop({
    syncFn: () => Promise.resolve(),
    intervalMs: 60_000,
    shutdownSignals: ["SIGUSR2"],
    log: () => {},
    terminate: (code) => terminateCalls.push(code),
  })

  process.emit("SIGUSR2")

  assert({
    given: "a shutdown signal is received",
    should: "call terminate with exit code 0",
    actual: terminateCalls[0],
    expected: 0,
  })
})

describe("startDiscoveryLoop() — cleanup removes signal listener", async (assert) => {
  const terminateCalls: number[] = []

  const { cleanup } = startDiscoveryLoop({
    syncFn: () => Promise.resolve(),
    intervalMs: 60_000,
    shutdownSignals: ["SIGUSR2"],
    log: () => {},
    terminate: (code) => terminateCalls.push(code),
  })

  cleanup()
  process.emit("SIGUSR2")

  assert({
    given: "cleanup is called before the signal fires",
    should: "not call terminate",
    actual: terminateCalls.length,
    expected: 0,
  })
})
