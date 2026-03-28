/**
 * Index route: project listing page.
 *
 * Displays all configured repositories with their test run counts and last
 * sync timestamps. This is the dashboard entry point.
 */

import type { LoaderFunctionArgs } from "react-router"
import { useLoaderData } from "react-router"
import { loadConfig } from "../lib/config.server.js"
import { getProjects, initDb } from "../lib/db.server.js"
import type { ProjectSummary } from "../lib/db.server.js"

export async function loader(_: LoaderFunctionArgs) {
  const configPath = process.env.DASHBOARD_CONFIG ?? "./dashboard.yml"
  const dbPath = process.env.DATABASE_URL ?? "./quarantine.db"
  const config = loadConfig(configPath)
  const db = initDb(dbPath)
  const projects = getProjects(db, config.repos)
  return { projects }
}

export default function Index() {
  const { projects } = useLoaderData<typeof loader>()

  return (
    <main style={{ fontFamily: "system-ui, sans-serif", padding: "2rem" }}>
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
          {projects.map((p: ProjectSummary) => (
            <tr key={`${p.owner}/${p.repo}`}>
              <td>{`${p.owner}/${p.repo}`}</td>
              <td>{p.testRunCount}</td>
              <td>{p.lastSynced ?? "Never"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </main>
  )
}
