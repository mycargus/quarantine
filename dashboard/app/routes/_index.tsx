/**
 * Index route: project listing page.
 *
 * Displays all configured repositories with their test run counts and last
 * sync timestamps. This is the dashboard entry point.
 */

export default function Index() {
  return (
    <main style={{ fontFamily: "system-ui, sans-serif", padding: "2rem" }}>
      <h1>Quarantine Dashboard</h1>
      <p>Flaky test analytics across your repositories.</p>

      <section>
        <h2>Projects</h2>
        <p style={{ color: "#666" }}>
          No projects configured yet. Add repositories to <code>dashboard.yml</code> to get started.
        </p>
        {/* TODO: M6 — Load projects from SQLite via loader, render table with
            repo name, test run count, last sync timestamp. */}
      </section>
    </main>
  )
}
