# Dashboard (TypeScript)

## Build & Test Commands

- Build: `pnpm run build`
- Dev: `pnpm dev`
- Test: `pnpm test`
- Lint: `pnpm run lint`
- Typecheck: `pnpm run typecheck`
- Format: `pnpm run format`

## Tech Stack

- **Framework:** React Router v7 (framework mode) with Vite
- **Language:** TypeScript (strict mode)
- **Database:** SQLite via better-sqlite3 (WAL mode)
- **Styling:** Tailwind CSS
- **Linting/Formatting:** Biome
- **Package Manager:** pnpm
- **Test assertions:** riteway (Given/Should/Actual/Expected pattern)

## Project Structure

| Path | Purpose |
|------|---------|
| `app/routes/` | React Router v7 route modules |
| `app/lib/db.server.ts` | SQLite operations (queries, migrations) |
| `app/lib/github.server.ts` | GitHub Artifacts polling |
| `app/lib/ingest.server.ts` | Artifact JSON ingestion into SQLite |
| `app/components/` | Shared UI components |

## Conventions

- Files ending in `.server.ts` run server-side only (React Router convention).
- Test files are co-located: `foo.server.test.ts` next to `foo.server.ts`.
- Use Biome for all formatting and linting (not ESLint/Prettier).
- Validate artifact JSON against `schemas/test-result.schema.json` using ajv.
- SQLite migrations run on server startup.

## Scope

- Do NOT modify CLI code (`cli/` directory).
- Do NOT add write-back features to the dashboard (read-only in v1).
- Do NOT add authentication beyond network-level access.
- Do NOT add frameworks beyond v1 scope.
