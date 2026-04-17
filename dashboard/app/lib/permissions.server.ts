import { parseLinkHeader } from "./github-client.server.js"

/**
 * Typed error thrown when the GitHub API returns 401 on user-permissions
 * endpoints, indicating the user's access token is expired or revoked.
 * Callers can use `instanceof UserPermissionsAuthError` to distinguish
 * auth failures from transient errors.
 */
export class UserPermissionsAuthError extends Error {
  constructor(status: number) {
    super(`GET /user/installations failed: ${status}`)
    this.name = "UserPermissionsAuthError"
  }
}

/**
 * Pure: returns the subset of `allProjects` whose `githubRepoId` appears
 * in `userRepoIds`. Projects with a null `githubRepoId` are excluded.
 */
export function filterProjectsByUserAccess(
  allProjects: Array<{ owner: string; repo: string; githubRepoId: number | null }>,
  userRepoIds: Set<number>,
): Array<{ owner: string; repo: string }> {
  return allProjects
    .filter((p) => p.githubRepoId !== null && userRepoIds.has(p.githubRepoId))
    .map(({ owner, repo }) => ({ owner, repo }))
}

interface UserInstallation {
  id: number
  account: { login: string }
}

interface UserReposResponse {
  total_count: number
  repositories: Array<{ id: number; name: string; owner: { login: string } }>
}

/**
 * I/O: fetches all repo IDs accessible to the user by paging through
 * GET /user/installations and GET /user/installations/{id}/repositories.
 * Returns a Set<number> of repository IDs.
 */
export async function fetchUserAccessibleRepoIds(
  userToken: string,
  fetchFn: typeof fetch,
  baseUrl: string,
): Promise<Set<number>> {
  const repoIds = new Set<number>()

  // 1. Fetch all user installations with pagination
  const allInstallations: UserInstallation[] = []
  let installationsUrl: string | null = `${baseUrl}/user/installations?per_page=100`

  while (installationsUrl) {
    const response = await fetchFn(installationsUrl, {
      headers: {
        Authorization: `Bearer ${userToken}`,
        Accept: "application/vnd.github+json",
      },
    })

    if (!response.ok) {
      if (response.status === 401) {
        throw new UserPermissionsAuthError(response.status)
      }
      throw new Error(`GET /user/installations failed: ${response.status}`)
    }

    const pageInstallations = (await response.json()) as UserInstallation[]
    allInstallations.push(...pageInstallations)

    const linkHeader = response.headers.get("link")
    installationsUrl = parseLinkHeader(linkHeader)
  }

  // 2. For each installation, fetch accessible repos with pagination
  for (const installation of allInstallations) {
    let reposUrl: string | null =
      `${baseUrl}/user/installations/${installation.id}/repositories?per_page=100`

    while (reposUrl) {
      const response = await fetchFn(reposUrl, {
        headers: {
          Authorization: `Bearer ${userToken}`,
          Accept: "application/vnd.github+json",
        },
      })

      if (!response.ok) {
        throw new Error(
          `GET /user/installations/${installation.id}/repositories failed: ${response.status}`,
        )
      }

      const repoData = (await response.json()) as UserReposResponse
      for (const repo of repoData.repositories) {
        repoIds.add(repo.id)
      }

      const linkHeader = response.headers.get("link")
      reposUrl = parseLinkHeader(linkHeader)
    }
  }

  return repoIds
}
