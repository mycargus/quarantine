import { describe } from "riteway"
import { getOrgOverview, initDb, upsertProject } from "./db.server.js"

describe("getOrgOverview()", async (assert) => {
  // --- empty database: no repos configured ---
  {
    const { db, raw } = initDb(":memory:")

    const result = await getOrgOverview({ db, raw }, [])

    assert({
      given: "an empty repos config",
      should: "return totalQuarantined of 0",
      actual: result.totalQuarantined,
      expected: 0,
    })

    assert({
      given: "an empty repos config",
      should: "return an empty byRepo array",
      actual: result.byRepo,
      expected: [],
    })

    assert({
      given: "an empty repos config",
      should: "return an empty mostRecentlyQuarantined array",
      actual: result.mostRecentlyQuarantined,
      expected: [],
    })
  }

  // --- repos configured but none have quarantined tests ---
  {
    const { db, raw } = initDb(":memory:")
    await upsertProject(db, "acme", "alpha")
    await upsertProject(db, "acme", "beta")

    const result = await getOrgOverview({ db, raw }, [
      { owner: "acme", repo: "alpha" },
      { owner: "acme", repo: "beta" },
    ])

    assert({
      given: "2 repos with no quarantined tests",
      should: "return totalQuarantined of 0",
      actual: result.totalQuarantined,
      expected: 0,
    })

    assert({
      given: "2 repos with no quarantined tests",
      should: "return byRepo with 0 count for each",
      actual: result.byRepo,
      expected: [
        { owner: "acme", repo: "alpha", quarantinedCount: 0 },
        { owner: "acme", repo: "beta", quarantinedCount: 0 },
      ],
    })
  }

  // --- 4 repos with a combined 12 quarantined tests (the scenario spec) ---
  {
    const { db, raw } = initDb(":memory:")
    const id1 = await upsertProject(db, "acme", "payments-service")
    const id2 = await upsertProject(db, "acme", "user-service")
    const id3 = await upsertProject(db, "acme", "frontend")
    const id4 = await upsertProject(db, "acme", "api-gateway")

    // payments-service: 4 quarantined tests
    const tests1 = [
      {
        testId: "t1",
        name: "should process payment",
        quarantinedAt: "2026-03-01T10:00:00Z",
        issueUrl: "https://github.com/acme/payments-service/issues/1",
      },
      {
        testId: "t2",
        name: "should refund payment",
        quarantinedAt: "2026-03-05T10:00:00Z",
        issueUrl: "https://github.com/acme/payments-service/issues/2",
      },
      {
        testId: "t3",
        name: "should validate card",
        quarantinedAt: "2026-03-10T10:00:00Z",
        issueUrl: "https://github.com/acme/payments-service/issues/3",
      },
      {
        testId: "t4",
        name: "should handle timeout",
        quarantinedAt: "2026-03-15T10:00:00Z",
        issueUrl: "https://github.com/acme/payments-service/issues/4",
      },
    ]
    for (const t of tests1) {
      raw
        .prepare(
          "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at, issue_url) VALUES (?, ?, ?, ?, ?)",
        )
        .run(id1, t.testId, t.name, t.quarantinedAt, t.issueUrl)
    }

    // user-service: 3 quarantined tests
    const tests2 = [
      {
        testId: "u1",
        name: "should register user",
        quarantinedAt: "2026-02-20T10:00:00Z",
        issueUrl: "https://github.com/acme/user-service/issues/1",
      },
      {
        testId: "u2",
        name: "should login user",
        quarantinedAt: "2026-02-25T10:00:00Z",
        issueUrl: "https://github.com/acme/user-service/issues/2",
      },
      {
        testId: "u3",
        name: "should logout user",
        quarantinedAt: "2026-03-20T10:00:00Z",
        issueUrl: "https://github.com/acme/user-service/issues/3",
      },
    ]
    for (const t of tests2) {
      raw
        .prepare(
          "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at, issue_url) VALUES (?, ?, ?, ?, ?)",
        )
        .run(id2, t.testId, t.name, t.quarantinedAt, t.issueUrl)
    }

    // frontend: 5 quarantined tests
    const tests3 = [
      {
        testId: "f1",
        name: "should render header",
        quarantinedAt: "2026-01-10T10:00:00Z",
        issueUrl: "https://github.com/acme/frontend/issues/1",
      },
      {
        testId: "f2",
        name: "should render footer",
        quarantinedAt: "2026-01-15T10:00:00Z",
        issueUrl: "https://github.com/acme/frontend/issues/2",
      },
      {
        testId: "f3",
        name: "should render login form",
        quarantinedAt: "2026-02-01T10:00:00Z",
        issueUrl: "https://github.com/acme/frontend/issues/3",
      },
      {
        testId: "f4",
        name: "should navigate to home",
        quarantinedAt: "2026-03-25T10:00:00Z",
        issueUrl: "https://github.com/acme/frontend/issues/4",
      },
      {
        testId: "f5",
        name: "should navigate to settings",
        quarantinedAt: "2026-03-28T10:00:00Z",
        issueUrl: "https://github.com/acme/frontend/issues/5",
      },
    ]
    for (const t of tests3) {
      raw
        .prepare(
          "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at, issue_url) VALUES (?, ?, ?, ?, ?)",
        )
        .run(id3, t.testId, t.name, t.quarantinedAt, t.issueUrl)
    }

    // api-gateway: 0 quarantined tests (id4 unused intentionally)
    void id4

    const repos = [
      { owner: "acme", repo: "payments-service" },
      { owner: "acme", repo: "user-service" },
      { owner: "acme", repo: "frontend" },
      { owner: "acme", repo: "api-gateway" },
    ]

    const result = await getOrgOverview({ db, raw }, repos)

    assert({
      given: "4 repos with a combined 12 quarantined tests",
      should: "return totalQuarantined of 12",
      actual: result.totalQuarantined,
      expected: 12,
    })

    assert({
      given: "4 repos with a combined 12 quarantined tests",
      should: "return byRepo with counts in config order",
      actual: result.byRepo,
      expected: [
        { owner: "acme", repo: "payments-service", quarantinedCount: 4 },
        { owner: "acme", repo: "user-service", quarantinedCount: 3 },
        { owner: "acme", repo: "frontend", quarantinedCount: 5 },
        { owner: "acme", repo: "api-gateway", quarantinedCount: 0 },
      ],
    })

    assert({
      given: "4 repos with 12 quarantined tests",
      should: "return top 5 mostRecentlyQuarantined ordered by quarantined_at desc",
      actual: result.mostRecentlyQuarantined,
      expected: [
        {
          owner: "acme",
          repo: "frontend",
          name: "should navigate to settings",
          quarantinedAt: "2026-03-28T10:00:00Z",
          issueUrl: "https://github.com/acme/frontend/issues/5",
        },
        {
          owner: "acme",
          repo: "frontend",
          name: "should navigate to home",
          quarantinedAt: "2026-03-25T10:00:00Z",
          issueUrl: "https://github.com/acme/frontend/issues/4",
        },
        {
          owner: "acme",
          repo: "user-service",
          name: "should logout user",
          quarantinedAt: "2026-03-20T10:00:00Z",
          issueUrl: "https://github.com/acme/user-service/issues/3",
        },
        {
          owner: "acme",
          repo: "payments-service",
          name: "should handle timeout",
          quarantinedAt: "2026-03-15T10:00:00Z",
          issueUrl: "https://github.com/acme/payments-service/issues/4",
        },
        {
          owner: "acme",
          repo: "payments-service",
          name: "should validate card",
          quarantinedAt: "2026-03-10T10:00:00Z",
          issueUrl: "https://github.com/acme/payments-service/issues/3",
        },
      ],
    })
  }

  // --- repos not in DB yet (never ingested) ---
  {
    const { db, raw } = initDb(":memory:")

    const result = await getOrgOverview({ db, raw }, [{ owner: "acme", repo: "never-ingested" }])

    assert({
      given: "a configured repo that has never been ingested (not in projects table)",
      should: "return quarantinedCount 0 for that repo",
      actual: result.byRepo,
      expected: [{ owner: "acme", repo: "never-ingested", quarantinedCount: 0 }],
    })
  }

  // Mutation guard: WHERE qt.project_id IN (...) must not be removed
  // If the WHERE clause is dropped, quarantined tests from unconfigured projects
  // leak into the results.
  {
    const { db, raw } = initDb(":memory:")
    const configuredId = await upsertProject(db, "acme", "configured-repo")
    const unconfiguredId = await upsertProject(db, "acme", "unconfigured-repo")

    // configured-repo has 2 quarantined tests
    raw
      .prepare(
        "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at, issue_url) VALUES (?, ?, ?, ?, ?)",
      )
      .run(
        configuredId,
        "c1",
        "should work A",
        "2026-03-01T10:00:00Z",
        "https://github.com/acme/configured-repo/issues/1",
      )
    raw
      .prepare(
        "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at, issue_url) VALUES (?, ?, ?, ?, ?)",
      )
      .run(
        configuredId,
        "c2",
        "should work B",
        "2026-03-02T10:00:00Z",
        "https://github.com/acme/configured-repo/issues/2",
      )

    // unconfigured-repo has 5 quarantined tests — these must NOT appear
    for (let i = 1; i <= 5; i++) {
      raw
        .prepare(
          "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at, issue_url) VALUES (?, ?, ?, ?, ?)",
        )
        .run(
          unconfiguredId,
          `u${i}`,
          `unconfigured test ${i}`,
          `2026-04-0${i}T00:00:00Z`,
          `https://github.com/acme/unconfigured-repo/issues/${i}`,
        )
    }

    const result = await getOrgOverview({ db, raw }, [{ owner: "acme", repo: "configured-repo" }])

    assert({
      given: "2 projects in DB but only 1 in the config repos list",
      should: "return totalQuarantined of 2 (only configured-repo's tests)",
      actual: result.totalQuarantined,
      expected: 2,
    })

    assert({
      given: "2 projects in DB but only 1 in the config repos list",
      should: "return byRepo with quarantinedCount 2 for the configured repo",
      actual: result.byRepo,
      expected: [{ owner: "acme", repo: "configured-repo", quarantinedCount: 2 }],
    })

    assert({
      given: "2 projects in DB but only 1 in the config repos list",
      should: "return only configured-repo tests in mostRecentlyQuarantined",
      actual: result.mostRecentlyQuarantined.every((t) => t.repo === "configured-repo"),
      expected: true,
    })

    assert({
      given: "2 projects in DB but only 1 in the config repos list",
      should: "return 2 entries in mostRecentlyQuarantined (not 7 from both projects)",
      actual: result.mostRecentlyQuarantined.length,
      expected: 2,
    })
  }

  // --- mostRecentlyQuarantined limited to 5 even with more entries ---
  {
    const { db, raw } = initDb(":memory:")
    const pid = await upsertProject(db, "org", "big-repo")
    for (let i = 1; i <= 8; i++) {
      const month = String(i).padStart(2, "0")
      raw
        .prepare(
          "INSERT INTO quarantined_tests (project_id, test_id, name, quarantined_at, issue_url) VALUES (?, ?, ?, ?, ?)",
        )
        .run(
          pid,
          `test-${i}`,
          `Test ${i}`,
          `2026-${month}-01T00:00:00Z`,
          `https://github.com/org/big-repo/issues/${i}`,
        )
    }

    const result = await getOrgOverview({ db, raw }, [{ owner: "org", repo: "big-repo" }])

    assert({
      given: "a repo with 8 quarantined tests",
      should: "return at most 5 in mostRecentlyQuarantined",
      actual: result.mostRecentlyQuarantined.length,
      expected: 5,
    })

    assert({
      given: "a repo with 8 quarantined tests",
      should: "return the 5 most recently quarantined (highest quarantined_at first)",
      actual: result.mostRecentlyQuarantined[0].quarantinedAt,
      expected: "2026-08-01T00:00:00Z",
    })
  }
})
