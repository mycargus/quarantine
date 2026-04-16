import { Auth } from "@remix-run/auth-middleware"
import type { Middleware } from "remix/fetch-router"
import { checkRateLimit } from "./rate-limit-logic.server.js"

const IP_LIMIT = 20
const USER_LIMIT = 300
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

/**
 * Creates a user-based rate limit middleware.
 * Applies a fixed-window counter: USER_LIMIT requests per WINDOW_MS per authenticated user.
 * Must run AFTER session + auth middleware so that Auth state is available.
 * Unauthenticated requests are skipped (already covered by the IP limiter).
 */
export function createUserRateLimiter(clock: () => number = Date.now): Middleware {
  const counters = new Map<string, WindowState>()

  return (context, next) => {
    const authState = context.get(Auth)
    if (!authState.ok) {
      return next()
    }

    const now = clock()
    const userId = String(authState.identity)
    let state = counters.get(userId)

    if (!state || now - state.windowStart >= WINDOW_MS) {
      state = { count: 0, windowStart: now }
      counters.set(userId, state)
    }

    const result = checkRateLimit(state.count, state.windowStart, now, USER_LIMIT, WINDOW_MS)

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
