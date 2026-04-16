import type { Middleware } from "remix/fetch-router"
import { checkRateLimit } from "./rate-limit-logic.server.js"

const IP_LIMIT = 20
const WINDOW_MS = 60_000

interface WindowState {
  count: number
  windowStart: number
}

/**
 * Extracts the client IP from the X-Forwarded-For header (first entry)
 * or falls back to "unknown".
 */
function extractIp(request: Request): string {
  const xff = request.headers.get("X-Forwarded-For")
  if (xff) {
    return xff.split(",")[0].trim()
  }
  return "unknown"
}

/**
 * Creates an IP-based rate limit middleware.
 * Applies a fixed-window counter: IP_LIMIT requests per WINDOW_MS.
 */
export function createIpRateLimiter(clock: () => number = Date.now): Middleware {
  const counters = new Map<string, WindowState>()

  return (context, next) => {
    const now = clock()
    const ip = extractIp(context.request)
    let state = counters.get(ip)

    if (!state || now - state.windowStart >= WINDOW_MS) {
      // New window
      state = { count: 0, windowStart: now }
      counters.set(ip, state)
    }

    const result = checkRateLimit(state.count, state.windowStart, now, IP_LIMIT, WINDOW_MS)

    if (!result.allowed) {
      return new Response("Too Many Requests", {
        status: 429,
        headers: { "Retry-After": String(result.retryAfter) },
      })
    }

    state.count++
    return next()
  }
}
