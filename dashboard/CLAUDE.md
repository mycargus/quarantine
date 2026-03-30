# Dashboard (TypeScript)

## Commands

```bash
make dash-build      # Or: pnpm run build (no-op — no build step with Remix 3)
make dash-test       # Or: pnpm test
make dash-lint       # Or: pnpm run lint
make dash-typecheck  # Or: pnpm run typecheck
pnpm dev             # Start dev server (node --import tsx/esm app/server.ts)
pnpm run format
```

## Structure

| Path | Purpose |
|------|---------|
| `app/server.ts` | HTTP server entry point (remix/node-fetch-server + remix/fetch-router) |
| `app/routes.ts` | Route map (remix/fetch-router/routes) |
| `app/controllers/home.tsx` | Index route handler + server-rendered component |
| `app/lib/db.server.ts` | SQLite operations (remix/data-table + better-sqlite3) |
| `app/lib/config.server.ts` | YAML config parsing (remix/data-schema) |
| `app/lib/github.server.ts` | GitHub Artifacts polling |
| `app/lib/ingest.server.ts` | Artifact JSON ingestion into SQLite |

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
