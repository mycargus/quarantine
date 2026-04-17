/**
 * Unit tests for filterProjectsByUserAccess (pure function).
 * Colocated per project convention for pure functions.
 */

import { describe } from "riteway"
import { filterProjectsByUserAccess } from "./permissions.server.js"

const p = (owner: string, repo: string, githubRepoId: number | null) => ({
  owner,
  repo,
  githubRepoId,
})

describe("filterProjectsByUserAccess — empty allProjects", async (assert) => {
  assert({
    given: "allProjects is empty and userRepoIds has entries",
    should: "return empty array",
    actual: filterProjectsByUserAccess([], new Set([1, 2, 3])),
    expected: [],
  })
})

describe("filterProjectsByUserAccess — empty userRepoIds", async (assert) => {
  const projects = [p("acme", "payments", 101), p("acme", "api", 102)]
  assert({
    given: "userRepoIds is empty",
    should: "return empty array",
    actual: filterProjectsByUserAccess(projects, new Set()),
    expected: [],
  })
})

describe("filterProjectsByUserAccess — no overlap", async (assert) => {
  const projects = [p("acme", "payments", 101), p("acme", "api", 102)]
  assert({
    given: "userRepoIds has no IDs matching any project",
    should: "return empty array",
    actual: filterProjectsByUserAccess(projects, new Set([201, 202])),
    expected: [],
  })
})

describe("filterProjectsByUserAccess — partial overlap", async (assert) => {
  const projects = [
    p("acme", "payments", 101),
    p("acme", "api", 102),
    p("acme", "frontend", 103),
    p("acme", "admin", 104),
  ]
  assert({
    given: "userRepoIds matches 2 of 4 projects",
    should: "return only the 2 matching projects",
    actual: filterProjectsByUserAccess(projects, new Set([102, 104])),
    expected: [
      { owner: "acme", repo: "api" },
      { owner: "acme", repo: "admin" },
    ],
  })
})

describe("filterProjectsByUserAccess — null githubRepoId excluded", async (assert) => {
  const projects = [
    p("acme", "payments", 101),
    p("acme", "manual-repo", null), // manual project, no github_repo_id
  ]
  assert({
    given: "one project has null githubRepoId and userRepoIds contains that position",
    should: "exclude the project with null githubRepoId",
    actual: filterProjectsByUserAccess(projects, new Set([101, 999])),
    expected: [{ owner: "acme", repo: "payments" }],
  })
})
