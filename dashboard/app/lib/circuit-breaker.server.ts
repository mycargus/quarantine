import type { PollState } from "./github.server.js"

export const FAILURE_THRESHOLD = 3
export const PAUSE_DURATION_MS = 30 * 60 * 1000 // 30 minutes

/** Returns true if the poll state is currently paused (pausedUntil is in the future). */
export function isPaused(state: PollState, now: Date): boolean {
  if (state.pausedUntil === null) return false
  return new Date(state.pausedUntil) > now
}

/** Records a failure. If consecutiveFailures reaches FAILURE_THRESHOLD, sets pausedUntil to now + 30 min. */
export function recordFailure(state: PollState, now: Date): PollState {
  const consecutiveFailures = state.consecutiveFailures + 1
  const shouldPause = consecutiveFailures >= FAILURE_THRESHOLD && state.pausedUntil === null
  return {
    ...state,
    consecutiveFailures,
    pausedUntil: shouldPause
      ? new Date(now.getTime() + PAUSE_DURATION_MS).toISOString()
      : state.pausedUntil,
  }
}

/** Records a success. Resets consecutiveFailures to 0 and clears pausedUntil. */
export function recordSuccess(state: PollState): PollState {
  return { ...state, consecutiveFailures: 0, pausedUntil: null }
}
