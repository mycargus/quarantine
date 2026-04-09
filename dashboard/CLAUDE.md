# Dashboard (TypeScript)

## Commands

```bash
make dash-test       # Or: pnpm test
make dash-lint       # Or: pnpm run lint
make dash-typecheck  # Or: pnpm run typecheck
pnpm dev             # Start dev server (node --import tsx/esm app/server.ts)
pnpm run format
pnpm run seed        # Seed local SQLite database with fixture data
```

## Structure

| Path | Purpose |
|------|---------|
| `app/server.ts` | HTTP server entry point (remix/node-fetch-server + remix/fetch-router) |
| `app/routes.ts` | Route map — two routes: `home` (`/`) and `projectDetail` (`/projects/:owner/:repo`) |
| `app/controllers/home.tsx` | Index route handler + server-rendered component |
| `app/controllers/project.tsx` | Project detail route handler + component |
| `app/lib/db.server.ts` | SQLite operations (better-sqlite3); migrations run on startup in `initDb()` |
| `app/lib/config.server.ts` | YAML config parsing (js-yaml + AJV + JSON Schema) |
| `app/lib/github.server.ts` | GitHub Artifacts polling |
| `app/lib/ingest.server.ts` | Artifact ZIP download + JSON ingestion into SQLite |
| `app/lib/sync.server.ts` | Sync orchestration (polls GitHub, triggers ingest) |
| `app/lib/filter.server.ts` | Query filtering logic for dashboard views |
| `app/lib/circuit-breaker.server.ts` | Rate limit / repeated-failure circuit breaker for GitHub API calls |
| `app/lib/debounce.server.ts` | Request deduplication for concurrent sync triggers |

## Conventions

- No build step — Remix 3 is "Religiously Runtime". `tsx` handles TypeScript at runtime.
- Import extensions use `.js` (TypeScript ESM convention — tsx resolves these to `.ts`).
- Test files are co-located: `foo.server.test.ts` next to `foo.server.ts`.
- Test assertions: riteway (Given/Should/Actual/Expected pattern).
- Formatting and linting via Biome (not ESLint/Prettier).
- `remix/data-schema` for dashboard-internal validation (config, route params, forms).
- `ajv` for `schemas/test-result.schema.json` only — shared JSON Schema contract with the Go CLI.
- `remix/data-table` with `better-sqlite3` for all database queries.
- SQLite schema migrations run on server startup in `initDb()` via raw SQL.
- Read-only in v1 — no write-back features to GitHub.
