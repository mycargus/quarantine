/**
 * Unit tests for InstallationTokenProvider — token refresh logging.
 *
 * Uses a fake fetch to avoid real network calls. Tests the observable
 * side effect: a structured log entry written on successful token exchange.
 */

import { generateKeyPairSync } from "node:crypto"
import { describe } from "riteway"
import { InstallationTokenProvider } from "./installation-token.server.js"

const { privateKey } = generateKeyPairSync("rsa", {
  modulusLength: 2048,
  publicKeyEncoding: { type: "spki", format: "pem" },
  privateKeyEncoding: { type: "pkcs8", format: "pem" },
})

function makeFakeFetch(token: string, expiresAt: string): typeof globalThis.fetch {
  return (_url: string | URL | Request, _init?: RequestInit): Promise<Response> => {
    const body = JSON.stringify({ token, expires_at: expiresAt, permissions: {} })
    return Promise.resolve(
      new Response(body, {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    )
  }
}

describe("InstallationTokenProvider — token refresh logged on successful exchange", async (assert) => {
  const newExpiresAt = "2026-04-16T12:00:00Z"
  const newToken = "ghs_new_token"

  const logMessages: string[] = []
  const fakeFetch = makeFakeFetch(newToken, newExpiresAt)
  const originalFetch = globalThis.fetch
  globalThis.fetch = fakeFetch as typeof globalThis.fetch

  try {
    // Build a provider with a cached token that expires in 4 minutes (below 5-min threshold)
    // so the next getToken call will trigger a refresh
    const nearExpiry = new Date(Date.now() + 4 * 60 * 1000).toISOString()
    const expiredTokenFetch = makeFakeFetch("ghs_old_token", nearExpiry)
    globalThis.fetch = expiredTokenFetch as typeof globalThis.fetch

    const provider = new InstallationTokenProvider({
      clientID: "Iv1.abc123",
      privateKeyPEM: privateKey as string,
      log: (msg: string) => logMessages.push(msg),
    })

    // First call: populates cache with near-expiry token (no log yet since we haven't set up log interception yet)
    await provider.getToken(123)

    // Now set up fetch to return the new token and clear log messages captured so far
    logMessages.length = 0
    globalThis.fetch = fakeFetch as typeof globalThis.fetch

    // Second call: cached token is near-expiry, triggers refresh
    await provider.getToken(123)
  } finally {
    globalThis.fetch = originalFetch
  }

  assert({
    given: "a cached installation token expiring in 4 minutes",
    should: "write exactly one log entry when a refresh is triggered",
    actual: logMessages.length,
    expected: 1,
  })

  assert({
    given: "a cached installation token expiring in 4 minutes",
    should: 'include "installation_token_refreshed" in the log entry',
    actual: logMessages[0].includes("installation_token_refreshed"),
    expected: true,
  })

  assert({
    given: "a cached installation token expiring in 4 minutes",
    should: "include the installation ID (123) in the log entry",
    actual: logMessages[0].includes("123"),
    expected: true,
  })

  assert({
    given: "a cached installation token expiring in 4 minutes",
    should: "include the new token expires_at in ISO 8601 format in the log entry",
    actual: logMessages[0].includes("2026-04-16T12:00:00.000Z"),
    expected: true,
  })

  assert({
    given: "a cached installation token expiring in 4 minutes",
    should: "include an ISO 8601 refresh timestamp in the log entry",
    actual: /\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z/.test(logMessages[0]),
    expected: true,
  })

  assert({
    given: "a cached installation token expiring in 4 minutes",
    should: "NOT include the token bearer value in the log entry",
    actual: logMessages[0].includes("ghs_new_token"),
    expected: false,
  })
})

describe("InstallationTokenProvider — log is NOT written when cached token is fresh (no refresh)", async (assert) => {
  const logMessages: string[] = []
  const originalFetch = globalThis.fetch

  // Return a token that expires 30 minutes from now (well above 5-min threshold)
  const freshExpiresAt = new Date(Date.now() + 30 * 60 * 1000).toISOString()
  globalThis.fetch = makeFakeFetch("ghs_fresh_token", freshExpiresAt) as typeof globalThis.fetch

  try {
    const provider = new InstallationTokenProvider({
      clientID: "Iv1.abc123",
      privateKeyPEM: privateKey as string,
      log: (msg: string) => logMessages.push(msg),
    })

    // First call: populates cache with a 30-min token (triggers one exchange + log)
    await provider.getToken(456)
    const countAfterFirst = logMessages.length

    // Second call: cache is fresh — should NOT trigger another exchange or log
    await provider.getToken(456)

    assert({
      given: "a cached token with 30 minutes remaining (above 5-min threshold)",
      should: "NOT write any additional log entry on the second call (cache hit, no refresh)",
      actual: logMessages.length,
      expected: countAfterFirst, // same count as after first call
    })
  } finally {
    globalThis.fetch = originalFetch
  }
})
