# ADR-005: Remix 3 + SQLite for Dashboard

**Status:** Accepted (updated)
**Date:** 2026-03-30

## Context

Need a tech stack for the dashboard web UI. The dashboard serves analytics, trends, and cross-repo flaky test visibility. It must support GitHub OAuth in v2.

Hard capability requirements:
- Dynamic charts (trends, flakiness over time)
- Interactive filters and tables (cross-repo visibility)
- Mature auth with OAuth (GitHub SSO)

These capabilities are framework-agnostic — Chart.js, D3, and ECharts work in any framework; Tailwind works everywhere; OAuth is a protocol. The question is which framework delivers them with the lowest risk for a v1 timeline.

## Decision

Remix 3 (alpha) + SQLite + Tailwind CSS.

Why Remix 3:
- **First-party auth with GitHub OAuth built in.** `remix/auth` ships providers for GitHub, Google, Microsoft, Okta, Auth0, and others. `remix/auth-middleware` handles route protection. No third-party auth library needed.
- **Working component model.** `remix/component` provides JSX-based server rendering with `renderToStream`, hydration via `clientEntry`, and partial server UI via `<Frame>`. APIs are stable enough for production use.
- **Cohesive first-party ecosystem.** `remix/fetch-router`, `remix/data-table`, `remix/data-schema`, `remix/node-fetch-server`, and related packages are actively released and designed to work together.
- **No bundler required ("Religiously Runtime").** `tsx` + `node --import tsx` is the full dev and production story. No Vite, no webpack.
- **Web standards throughout.** Fetch API, Request/Response, Web Crypto. Runs on Node, Bun, Deno, Workers without adapters.
- **Low-stakes migration window.** The dashboard is read-only and non-critical. Almost no code exists yet. Migrating now is cheaper than migrating later once more routes and components are built.

SQLite provides zero-config persistence. Tailwind provides utility-first CSS.

### Validation strategy

- **ajv** for `schemas/test-result.schema.json`: This is a shared JSON Schema contract with the Go CLI. `remix/data-schema` uses its own DSL and cannot consume JSON Schema files. Maintaining two representations would create drift risk. ajv stays.
- **remix/data-schema** for all other validation: dashboard config, route params, form data.

### Alpha risk acceptance

Remix 3 is at `3.0.0-alpha.4` (released 2026-03-25). This is accepted for the dashboard because:
1. The dashboard is read-only and non-critical — it is not in the CI path.
2. The packages we depend on (`fetch-router`, `component`, `data-table`, `node-fetch-server`) have active releases and stable-enough APIs.
3. Migrating later (once more is built) would cost more than the risk of occasional alpha breakage.

## Alternatives Considered

- **React Router v7 (previously adopted):** Rejected on revisit. The dashboard has minimal existing code, making now the cheapest migration window. React Router v7 requires React, Vite, and a build step — overhead that Remix 3 eliminates. React Router v7 remains the correct choice for React-based apps.
- **Go + HTMX:** Simpler architecture, single-binary deployment, same language as the CLI. Rejected because: (1) the dashboard is the user-facing product where UI richness matters; (2) HTMX is limited for interactive visualizations (dynamic charts, client-side filtering); (3) the Remix 3 ecosystem offers a richer component ecosystem. Go + HTMX remains a valid alternative if priorities change.
- **Python + FastAPI:** Excellent HTMX pairing but no single-binary deployment story. Adds a second language without the frontend ecosystem benefits.

## Consequences

**Positive:**
- No build step — dev and production both run with `node --import tsx app/server.ts`.
- First-party auth covers the v2 GitHub OAuth requirement without additional libraries.
- Web standards throughout — no runtime lock-in.
- Cohesive Remix ecosystem: data-table, data-schema, fetch-router all work together.
- Lighter than React Router v7 + React (no virtual DOM, no bundler).

**Negative:**
- Alpha software — APIs may change before stable release. Mitigated by low stakes (read-only, non-critical).
- Non-React component model — `remix/component` uses manual state updates (`handle.update()`) rather than reactive rendering. Steeper learning curve for developers familiar with React.
- Two languages in the project (Go CLI + TypeScript dashboard). Mitigated by a clean boundary — the shared data contract is a JSON schema, no shared types needed.
- Requires Node.js runtime (no single binary).
