import { route } from "remix/fetch-router/routes"

export const routes = route({
  home: "/",
  health: "/health",
  authLogin: "/auth/login",
  projectDetail: "/projects/:owner/:repo",
})
