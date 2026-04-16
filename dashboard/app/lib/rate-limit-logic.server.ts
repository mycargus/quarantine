export interface RateLimitResult {
  allowed: boolean
  retryAfter: number // seconds until window resets
}

/**
 * Pure function: decides whether a request is allowed based on the current
 * counter and window state.
 *
 * Returns { allowed: true, retryAfter: 0 } when under the limit, or
 * { allowed: false, retryAfter: N } when the limit is exceeded.
 */
export function checkRateLimit(
  counter: number,
  windowStart: number,
  now: number,
  limit: number,
  windowMs: number,
): RateLimitResult {
  const elapsed = now - windowStart
  if (elapsed >= windowMs) {
    // Window has expired; this request starts a new window
    return { allowed: true, retryAfter: 0 }
  }
  if (counter < limit) {
    return { allowed: true, retryAfter: 0 }
  }
  const remainingMs = windowMs - elapsed
  return { allowed: false, retryAfter: Math.ceil(remainingMs / 1000) }
}
