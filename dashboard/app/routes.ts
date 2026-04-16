import { route } from "remix/fetch-router/routes"

export const routes = route({
  home: "/",
  health: "/health",
  authLogin: "/auth/login",
  authLogout: "/auth/logout",
  projectDetail: "/projects/:owner/:repo",
})
