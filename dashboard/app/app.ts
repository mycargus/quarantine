import {
  completeAuth,
  createGitHubAuthProvider,
  finishExternalAuth,
  startExternalAuth,
} from "@remix-run/auth"
import { requireAuth } from "@remix-run/auth-middleware"
import { Session } from "@remix-run/session"
import { createRouter } from "remix/fetch-router"
import { home } from "./controllers/home.js"
import { project } from "./controllers/project.js"
import { createIpRateLimiter, createUserRateLimiter } from "./lib/rate-limit-middleware.server.js"
import { createSessionMiddleware } from "./lib/session.server.js"
import { routes } from "./routes.js"

export interface AppOptions {
  configPath?: string
  dbPath?: string
  token?: string
  fetchFn?: typeof fetch
  sessionSecret?: string
  oauthClientId?: string
  oauthClientSecret?: string
  oauthOrigin?: string
  clock?: () => number
}

const DEFAULT_SECRET = "dev-secret-change-in-production"

export function createApp(opts: AppOptions = {}) {
  const { session, auth } = createSessionMiddleware(opts.sessionSecret ?? DEFAULT_SECRET)
  const ipRateLimiter = createIpRateLimiter(opts.clock)
  const userRateLimiter = createUserRateLimiter(opts.clock)

  const githubProvider =
    opts.oauthClientId && opts.oauthClientSecret && opts.oauthOrigin
      ? createGitHubAuthProvider({
          clientId: opts.oauthClientId,
          clientSecret: opts.oauthClientSecret,
          redirectUri: `${opts.oauthOrigin}/auth/github/callback`,
        })
      : null

  const router = createRouter({ middleware: [ipRateLimiter, session, auth, userRateLimiter] })
  router.map(routes, {
    actions: {
      home: {
        middleware: [requireAuth()],
        handler: () => home(opts),
      },
      health: () => new Response("ok", { status: 200 }),
      authLogin: (ctx) => startExternalAuth(githubProvider!, ctx),
      authCallback: async (ctx) => {
        const { result } = await finishExternalAuth(githubProvider!, ctx)
        const session = completeAuth(ctx)
        session.set("userId" as never, result.profile.login as never)
        return new Response(null, {
          status: 302,
          headers: { Location: "/" },
        })
      },
      authLogout: (ctx) => {
        const s = ctx.get(Session)
        s.destroy()
        return new Response(null, {
          status: 302,
          headers: { Location: "/auth/login" },
        })
      },
      projectDetail: {
        middleware: [requireAuth()],
        handler: (ctx) => project(ctx.params.owner, ctx.params.repo, ctx.request.url, opts.dbPath),
      },
    },
  })
  return router
}
