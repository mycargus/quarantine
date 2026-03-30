import type { Handle } from "remix/component"
import { renderToStream } from "remix/component/server"
import { loadConfig } from "../lib/config.server.js"
import type { ProjectSummary } from "../lib/db.server.js"
import { getProjects, initDb } from "../lib/db.server.js"

function ProjectRow(_handle: Handle, project: ProjectSummary) {
  return () => (
    <tr>
      <td>{`${project.owner}/${project.repo}`}</td>
      <td>{String(project.testRunCount)}</td>
      <td>{project.lastSynced ?? "Never"}</td>
    </tr>
  )
}

function ProjectsPage(_handle: Handle, projects: ProjectSummary[]) {
  return () => (
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>Quarantine Dashboard</title>
      </head>
      <body>
        <main style="font-family: system-ui, sans-serif; padding: 2rem">
          <h1>Projects</h1>
          <table>
            <thead>
              <tr>
                <th>Repository</th>
                <th>Test Runs</th>
                <th>Last Synced</th>
              </tr>
            </thead>
            <tbody>
              {projects.map((p) => (
                <ProjectRow setup={p} key={`${p.owner}/${p.repo}`} />
              ))}
            </tbody>
          </table>
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
        <main style="font-family: system-ui, sans-serif; padding: 2rem">
          <h1>Configuration Error</h1>
          <p>{message}</p>
        </main>
      </body>
    </html>
  )
}

export async function home(): Promise<Response> {
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

  const { db } = initDb(dbPath)
  const projects = await getProjects(db, config.repos)

  const stream = renderToStream(<ProjectsPage setup={projects} />)

  return new Response(stream, {
    headers: { "Content-Type": "text/html; charset=utf-8" },
  })
}
