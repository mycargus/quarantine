# Dashboard (TypeScript)

## Commands

```bash
make dash-build      # Or: pnpm run build
make dash-test       # Or: pnpm test
make dash-lint       # Or: pnpm run lint
make dash-typecheck  # Or: pnpm run typecheck
pnpm dev             # Start dev server (no Makefile target)
pnpm run format
```

## Structure

| Path | Purpose |
|------|---------|
| `app/routes/` | React Router v7 route modules |
| `app/lib/db.server.ts` | SQLite operations (queries, migrations) |
| `app/lib/github.server.ts` | GitHub Artifacts polling |
| `app/lib/ingest.server.ts` | Artifact JSON ingestion into SQLite |
| `app/components/` | Shared UI components |

## Conventions

- `.server.ts` files run server-side only (React Router convention).
- Test files are co-located: `foo.server.test.ts` next to `foo.server.ts`.
- Test assertions: riteway (Given/Should/Actual/Expected pattern).
- Formatting and linting via Biome (not ESLint/Prettier).
- Validate artifact JSON against `schemas/test-result.schema.json` using ajv.
- SQLite migrations run on server startup.
- Read-only in v1 — no write-back features to GitHub.
