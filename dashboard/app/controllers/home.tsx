import type { Handle } from "remix/component"
import { renderToStream } from "remix/component/server"
import { loadConfig } from "../lib/config.server.js"
import type { OrgOverview, ProjectSummary } from "../lib/db.server.js"
import { getOrgOverview, getProjects, initDb } from "../lib/db.server.js"
import { syncRepo } from "../lib/sync.server.js"

function ProjectRow(_handle: Handle, project: ProjectSummary) {
  return () => (
    <tr>
      <td>{`${project.owner}/${project.repo}`}</td>
      <td>{String(project.testRunCount)}</td>
      <td>{project.lastSynced ?? "Never"}</td>
    </tr>
  )
}

interface PageData {
  projects: ProjectSummary[]
  overview: OrgOverview
}

function RecentTestRow(_handle: Handle, test: OrgOverview["mostRecentlyQuarantined"][number]) {
  return () => (
    <tr>
      <td>{`${test.owner}/${test.repo}`}</td>
      <td>{test.name}</td>
      <td>{test.quarantinedAt}</td>
      <td>{test.issueUrl ? <a href={test.issueUrl}>Issue</a> : "—"}</td>
    </tr>
  )
}

function RepoOverviewRow(_handle: Handle, row: OrgOverview["byRepo"][number]) {
  return () => (
    <tr>
      <td>
        <a href={`/projects/${row.owner}/${row.repo}`}>{`${row.owner}/${row.repo}`}</a>
      </td>
      <td>{String(row.quarantinedCount)}</td>
    </tr>
  )
}

function ProjectsPage(_handle: Handle, data: PageData) {
  return () => (
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>Quarantine Dashboard</title>
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
          <h1>Projects</h1>

          <section>
            <h2>Overview</h2>
            <p>
              Total quarantined tests: <strong>{String(data.overview.totalQuarantined)}</strong>
            </p>

            <h3>By Repository</h3>
            <table>
              <thead>
                <tr>
                  <th>Repository</th>
                  <th>Quarantined</th>
                </tr>
              </thead>
              <tbody>
                {data.overview.byRepo.map((r) => (
                  <RepoOverviewRow setup={r} key={`${r.owner}/${r.repo}`} />
                ))}
              </tbody>
            </table>

            <h3>Most Recently Quarantined</h3>
            <table>
              <thead>
                <tr>
                  <th>Repository</th>
                  <th>Test</th>
                  <th>Quarantined At</th>
                  <th>Issue</th>
                </tr>
              </thead>
              <tbody>
                {data.overview.mostRecentlyQuarantined.map((t) => (
                  <RecentTestRow setup={t} key={`${t.owner}/${t.repo}/${t.name}`} />
                ))}
              </tbody>
            </table>
          </section>

          <section>
            <h2>All Repositories</h2>
            <table>
              <thead>
                <tr>
                  <th>Repository</th>
                  <th>Test Runs</th>
                  <th>Last Synced</th>
                </tr>
              </thead>
              <tbody>
                {data.projects.map((p) => (
                  <ProjectRow setup={p} key={`${p.owner}/${p.repo}`} />
                ))}
              </tbody>
            </table>
          </section>
        </main>
      </body>
    </html>
  )
}

function ErrorPage(_handle: Handle, message: string) {
  return () => (
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>Quarantine Dashboard — Error</title>
      </head>
      <body>
        <main>
          <h1>Configuration Error</h1>
          <p>{message}</p>
        </main>
      </body>
    </html>
  )
}

interface HomeOptions {
  fetchFn?: typeof fetch
}

export async function home(options: HomeOptions = {}): Promise<Response> {
  const configPath = process.env.DASHBOARD_CONFIG ?? "./dashboard.yml"
  const dbPath = process.env.DATABASE_URL ?? "./quarantine.db"

  let config: ReturnType<typeof loadConfig>
  try {
    config = loadConfig(configPath)
  } catch (e) {
    const message =
      e instanceof Error && "code" in e && e.code === "ENOENT"
        ? `Config file not found: ${configPath}. Copy dashboard.yml.example to dashboard.yml to get started.`
        : `Failed to load config: ${e instanceof Error ? e.message : String(e)}`

    const stream = renderToStream(<ErrorPage setup={message} />)
    return new Response(stream, {
      status: 500,
      headers: { "Content-Type": "text/html; charset=utf-8" },
    })
  }

  try {
    const handle = initDb(dbPath)
    const token = process.env.QUARANTINE_GITHUB_TOKEN ?? process.env.GITHUB_TOKEN

    if (token) {
      const now = new Date()
      const fetchFn = options.fetchFn ?? fetch
      for (const { owner, repo } of config.repos) {
        await syncRepo(owner, repo, token, handle, now, fetchFn, console.warn)
      }
    }

    const [projects, overview] = await Promise.all([
      getProjects(handle.db, config.repos),
      getOrgOverview(handle, config.repos),
    ])

    const stream = renderToStream(<ProjectsPage setup={{ projects, overview }} />)

    return new Response(stream, {
      headers: { "Content-Type": "text/html; charset=utf-8" },
    })
  } catch (e) {
    const message = `Internal error: ${e instanceof Error ? e.message : String(e)}`
    const stream = renderToStream(<ErrorPage setup={message} />)
    return new Response(stream, {
      status: 500,
      headers: { "Content-Type": "text/html; charset=utf-8" },
    })
  }
}
