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
import { formatAuthEvent } from "./lib/auth.server.js"
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
  getInstallationToken?: (installationId: number) => Promise<string>
  userAccessToken?: string
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
        handler: (ctx) => {
          const s = ctx.get(Session)
          const userAccessToken = s.get("accessToken" as never) as string | undefined
          return home({ ...opts, userAccessToken })
        },
      },
      health: () => new Response("ok", { status: 200 }),
      authLogin: (ctx) => startExternalAuth(githubProvider!, ctx),
      authCallback: async (ctx) => {
        try {
          const { result } = await finishExternalAuth(githubProvider!, ctx)
          const session = completeAuth(ctx)
          session.set("userId" as never, result.profile.login as never)
          session.set("accessToken" as never, result.tokens.accessToken as never)
          const timestamp = new Date().toISOString()
          console.log(formatAuthEvent("login", result.profile.login, timestamp))
          return new Response(null, {
            status: 302,
            headers: { Location: "/" },
          })
        } catch {
          return new Response("OAuth authentication failed", { status: 400 })
        }
      },
      authLogout: (ctx) => {
        const s = ctx.get(Session)
        const userId = s.get("userId" as never) as string
        const timestamp = new Date().toISOString()
        console.log(formatAuthEvent("logout", userId, timestamp))
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
