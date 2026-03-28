import { describe } from "riteway/esm"
import {
  PAUSE_DURATION_MS,
  isPaused,
  recordFailure,
  recordSuccess,
} from "./circuit-breaker.server.js"
import type { PollState } from "./github.server.js"

const clean: PollState = { lastEtag: null, consecutiveFailures: 0, pausedUntil: null }

const now = new Date("2026-01-01T12:00:00.000Z")

describe("isPaused()", async (assert) => {
  assert({
    given: "a state with null pausedUntil",
    should: "return false",
    actual: isPaused(clean, now),
    expected: false,
  })

  assert({
    given: "a pausedUntil 10 minutes in the future",
    should: "return true",
    actual: isPaused(
      { ...clean, pausedUntil: new Date(now.getTime() + 10 * 60 * 1000).toISOString() },
      now,
    ),
    expected: true,
  })

  assert({
    given: "a pausedUntil 10 minutes in the past",
    should: "return false (pause expired)",
    actual: isPaused(
      { ...clean, pausedUntil: new Date(now.getTime() - 10 * 60 * 1000).toISOString() },
      now,
    ),
    expected: false,
  })

  assert({
    given: "a pausedUntil equal to now (boundary)",
    should: "return false (strict greater-than, exact boundary is not paused)",
    actual: isPaused({ ...clean, pausedUntil: now.toISOString() }, now),
    expected: false,
  })
})

describe("recordFailure()", async (assert) => {
  assert({
    given: "0 consecutive failures (1st failure)",
    should: "increment consecutiveFailures to 1 and keep pausedUntil null",
    actual: recordFailure(clean, now),
    expected: { lastEtag: null, consecutiveFailures: 1, pausedUntil: null },
  })

  assert({
    given: "1 consecutive failure (2nd failure)",
    should: "increment consecutiveFailures to 2 and keep pausedUntil null",
    actual: recordFailure({ ...clean, consecutiveFailures: 1 }, now),
    expected: { lastEtag: null, consecutiveFailures: 2, pausedUntil: null },
  })

  assert({
    given: "2 consecutive failures (3rd failure, threshold reached)",
    should: "increment consecutiveFailures to 3 and set pausedUntil to now + 30 min",
    actual: recordFailure({ ...clean, consecutiveFailures: 2 }, now),
    expected: {
      lastEtag: null,
      consecutiveFailures: 3,
      pausedUntil: new Date(now.getTime() + PAUSE_DURATION_MS).toISOString(),
    },
  })

  assert({
    given: "3 consecutive failures already paused (4th failure)",
    should: "increment consecutiveFailures to 4 and leave pausedUntil unchanged",
    actual: recordFailure(
      {
        ...clean,
        consecutiveFailures: 3,
        pausedUntil: new Date(now.getTime() + PAUSE_DURATION_MS).toISOString(),
      },
      now,
    ),
    expected: {
      lastEtag: null,
      consecutiveFailures: 4,
      pausedUntil: new Date(now.getTime() + PAUSE_DURATION_MS).toISOString(),
    },
  })

  assert({
    given: "a state with a non-null lastEtag",
    should: "preserve lastEtag in the returned state",
    actual: recordFailure({ ...clean, lastEtag: '"etag-abc"' }, now).lastEtag,
    expected: '"etag-abc"',
  })
})

describe("recordSuccess()", async (assert) => {
  assert({
    given: "0 failures (already clean state)",
    should: "return state with consecutiveFailures 0 and null pausedUntil",
    actual: recordSuccess(clean),
    expected: { lastEtag: null, consecutiveFailures: 0, pausedUntil: null },
  })

  assert({
    given: "2 consecutive failures (below threshold)",
    should: "reset consecutiveFailures to 0 and keep pausedUntil null",
    actual: recordSuccess({ ...clean, consecutiveFailures: 2 }),
    expected: { lastEtag: null, consecutiveFailures: 0, pausedUntil: null },
  })

  assert({
    given: "paused state with 3 consecutive failures",
    should: "reset consecutiveFailures to 0 and clear pausedUntil",
    actual: recordSuccess({
      ...clean,
      consecutiveFailures: 3,
      pausedUntil: new Date(now.getTime() + PAUSE_DURATION_MS).toISOString(),
    }),
    expected: { lastEtag: null, consecutiveFailures: 0, pausedUntil: null },
  })

  assert({
    given: "a state with a non-null lastEtag",
    should: "preserve lastEtag in the returned state",
    actual: recordSuccess({ ...clean, lastEtag: '"etag-abc"' }).lastEtag,
    expected: '"etag-abc"',
  })
})
