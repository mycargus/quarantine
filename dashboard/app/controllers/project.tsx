import type { Handle } from "remix/component"
import { renderToStream } from "remix/component/server"
import type { QuarantinedTestDetail, TrendPoint } from "../lib/db.server.js"
import {
  getProjectByOwnerRepo,
  getProjectQuarantinedTests,
  getProjectTrend,
  initDb,
} from "../lib/db.server.js"
import { applyFilters } from "../lib/filter.server.js"

function NotFoundPage(_handle: Handle, repoHandle: string) {
  return () => (
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>Not Found — Quarantine Dashboard</title>
      </head>
      <body>
        <main>
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
  filteredCount: number
  totalCount: number
}

function ProjectPage(_handle: Handle, data: ProjectPageData) {
  return () => (
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>{`${data.owner}/${data.repo} — Quarantine Dashboard`}</title>
        <style>{`
          body { font-family: system-ui, sans-serif; margin: 0; padding: 0; }
          main { padding: 1rem 2rem; max-width: 1200px; margin: 0 auto; }
          table { width: 100%; border-collapse: collapse; }
          th, td { padding: 0.5rem; text-align: left; border-bottom: 1px solid #e5e7eb; }
          th { font-weight: 600; background: #f9fafb; }
          @media (max-width: 640px) {
            main { padding: 1rem; }
            table, thead, tbody, tr { display: block; }
            thead { display: none; }
            td { display: flex; gap: 0.5rem; padding: 0.25rem 0; }
            td::before { content: attr(data-label); font-weight: 600; min-width: 8rem; }
          }
        `}</style>
      </head>
      <body>
        <main>
          <h1>{`${data.owner}/${data.repo}`}</h1>

          <section>
            <h2>Quarantined Tests</h2>
            <p>
              {"Showing "}
              {String(data.filteredCount)}
              {" of "}
              {String(data.totalCount)}
              {" quarantined tests"}
            </p>
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

export async function project(owner: string, repo: string, url: string): Promise<Response> {
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

  const parsedUrl = new URL(url)
  const search = parsedUrl.searchParams.get("search") ?? ""
  const statusParam = parsedUrl.searchParams.get("status")
  const status = statusParam === "failing" || statusParam === "passing" ? statusParam : null
  const from = parsedUrl.searchParams.get("from")
  const until = parsedUrl.searchParams.get("until")

  const [allTests, trend] = await Promise.all([
    getProjectQuarantinedTests(handle, owner, repo),
    getProjectTrend(handle, owner, repo),
  ])

  const tests = applyFilters(allTests, search, status, from, until)

  const stream = renderToStream(
    <ProjectPage
      setup={{
        owner,
        repo,
        tests,
        trend,
        filteredCount: tests.length,
        totalCount: allTests.length,
      }}
    />,
  )
  return new Response(stream, {
    headers: { "Content-Type": "text/html; charset=utf-8" },
  })
}
