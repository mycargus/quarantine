/**
 * Interface tests for the discovery loop (startDiscoveryLoop) and
 * startup sync timeout (startupSyncWithTimeout).
 *
 * These test timer-based coordination and process signal handling with
 * injected dependencies — no real GitHub API calls or databases.
 */

import { describe } from "riteway"
import {
  startDiscoveryLoop,
  startupSyncWithTimeout,
} from "../app/lib/installation-sync.server.js"

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

describe("startDiscoveryLoop() — periodic re-sync", async (assert) => {
  let syncCount = 0
  const syncFn = async () => {
    syncCount++
  }

  const { cleanup } = startDiscoveryLoop({
    syncFn,
    intervalMs: 50,
    shutdownSignals: [], // no signals to avoid test runner issues
    log: () => {},
  })

  try {
    // Verify first tick does NOT fire immediately
    assert({
      given: "startDiscoveryLoop just called",
      should: "not call syncFn immediately (setInterval behavior)",
      actual: syncCount,
      expected: 0,
    })

    // Wait enough for 2 interval ticks (50ms each, so ~120ms)
    await wait(120)

    assert({
      given: "120ms elapsed with a 50ms interval",
      should: "have called syncFn at least 2 times",
      actual: syncCount >= 2,
      expected: true,
    })
  } finally {
    cleanup()
  }
})

describe("startDiscoveryLoop() — shutdown stops the loop", async (assert) => {
  let syncCount = 0
  const syncFn = async () => {
    syncCount++
  }

  let exitCode: number | null = null
  const terminate = (code: number) => {
    exitCode = code
  }

  const { cleanup } = startDiscoveryLoop({
    syncFn,
    intervalMs: 50,
    shutdownSignals: ["SIGTERM"],
    log: () => {},
    terminate,
  })

  try {
    // Wait for at least 1 tick to fire
    await wait(60)

    const countBeforeSignal = syncCount

    // Emit SIGTERM (safe: EventEmitter dispatch, no real signal)
    process.emit("SIGTERM")

    // Wait long enough for additional ticks that should NOT fire
    await wait(100)

    assert({
      given: "SIGTERM emitted after the loop started",
      should: "not call syncFn again after the signal",
      actual: syncCount,
      expected: countBeforeSignal,
    })

    assert({
      given: "SIGTERM emitted",
      should: "call terminate with exit code 0",
      actual: exitCode,
      expected: 0,
    })
  } finally {
    cleanup()
  }
})

describe("startupSyncWithTimeout() — sync exceeds timeout", async (assert) => {
  const logs: string[] = []
  const log = (msg: string) => logs.push(msg)

  let exitCode: number | null = null
  const terminate = (code: number) => {
    exitCode = code
  }

  // A syncFn that never resolves within the timeout
  const slowSyncFn = () => new Promise<void>((resolve) => setTimeout(resolve, 500))

  // Use a short timeout (100ms) so the test finishes quickly.
  // The function throws after timeout — catch it since we verify
  // behavior via the terminate spy and logs.
  let thrownMessage: string | null = null
  try {
    await startupSyncWithTimeout(slowSyncFn, 100, log, terminate)
  } catch (err) {
    thrownMessage = err instanceof Error ? err.message : String(err)
  }

  assert({
    given: "syncFn does not complete within 100ms timeout",
    should: "call terminate with a non-zero exit code",
    actual: exitCode !== null && exitCode !== 0,
    expected: true,
  })

  assert({
    given: "syncFn does not complete within 100ms timeout",
    should: "log a message containing 'timed out'",
    actual: logs.some((msg) => /timed out/i.test(msg)),
    expected: true,
  })

  assert({
    given: "syncFn does not complete within 100ms timeout",
    should: "throw an error indicating timeout",
    actual: thrownMessage,
    expected: "Startup sync timed out",
  })
})
