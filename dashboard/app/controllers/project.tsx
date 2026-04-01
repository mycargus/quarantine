import type { Handle } from "remix/component"
import { renderToStream } from "remix/component/server"
import type { QuarantinedTestDetail, TrendPoint } from "../lib/db.server.js"
import {
  getProjectByOwnerRepo,
  getProjectQuarantinedTests,
  getProjectTrend,
  initDb,
} from "../lib/db.server.js"

function NotFoundPage(_handle: Handle, repoHandle: string) {
  return () => (
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>Not Found — Quarantine Dashboard</title>
      </head>
      <body>
        <main style="font-family: system-ui, sans-serif; padding: 2rem">
          <h1>Not Found</h1>
          <p>Project {repoHandle} was not found.</p>
        </main>
      </body>
    </html>
  )
}

function QuarantinedTestRow(_handle: Handle, test: QuarantinedTestDetail) {
  return () => (
    <tr>
      <td>{test.name}</td>
      <td>{test.quarantinedAt}</td>
      <td>{test.lastFlakyAt ?? "Never"}</td>
      <td>
        {test.issueUrl !== null && test.issueNumber !== null ? (
          <a href={test.issueUrl}>{String(test.issueNumber)}</a>
        ) : (
          "—"
        )}
      </td>
    </tr>
  )
}

function TrendRow(_handle: Handle, point: TrendPoint) {
  return () => (
    <tr>
      <td>{point.date}</td>
      <td>{String(point.flakyCount)}</td>
    </tr>
  )
}

interface ProjectPageData {
  owner: string
  repo: string
  tests: QuarantinedTestDetail[]
  trend: TrendPoint[]
}

function ProjectPage(_handle: Handle, data: ProjectPageData) {
  return () => (
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>{`${data.owner}/${data.repo} — Quarantine Dashboard`}</title>
      </head>
      <body>
        <main style="font-family: system-ui, sans-serif; padding: 2rem">
          <h1>{`${data.owner}/${data.repo}`}</h1>

          <section>
            <h2>Quarantined Tests</h2>
            <table>
              <thead>
                <tr>
                  <th>Test Name</th>
                  <th>First Quarantined</th>
                  <th>Last Flaky Occurrence</th>
                  <th>Issue</th>
                </tr>
              </thead>
              <tbody>
                {data.tests.map((t) => (
                  <QuarantinedTestRow setup={t} key={t.name} />
                ))}
              </tbody>
            </table>
          </section>

          <section>
            <h2>Flaky Test Trend</h2>
            <table>
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Flaky Test Count</th>
                </tr>
              </thead>
              <tbody>
                {data.trend.map((p) => (
                  <TrendRow setup={p} key={p.date} />
                ))}
              </tbody>
            </table>
          </section>
        </main>
      </body>
    </html>
  )
}

export async function project(owner: string, repo: string): Promise<Response> {
  const dbPath = process.env.DATABASE_URL ?? "./quarantine.db"
  const handle = initDb(dbPath)

  const projectRow = await getProjectByOwnerRepo(handle, owner, repo)
  if (projectRow === null) {
    const stream = renderToStream(<NotFoundPage setup={`${owner}/${repo}`} />)
    return new Response(stream, {
      status: 404,
      headers: { "Content-Type": "text/html; charset=utf-8" },
    })
  }

  const [tests, trend] = await Promise.all([
    getProjectQuarantinedTests(handle, owner, repo),
    getProjectTrend(handle, owner, repo),
  ])

  const stream = renderToStream(<ProjectPage setup={{ owner, repo, tests, trend }} />)
  return new Response(stream, {
    headers: { "Content-Type": "text/html; charset=utf-8" },
  })
}
