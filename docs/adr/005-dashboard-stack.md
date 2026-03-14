# ADR-005: React Router v7 (Framework Mode) + SQLite for Dashboard

**Status:** Accepted (updated)
**Date:** 2026-03-14

## Context

Need a tech stack for the dashboard web UI. The dashboard serves analytics, trends, and cross-repo flaky test visibility. It must support GitHub OAuth in v2.

Hard capability requirements:
- Dynamic charts (trends, flakiness over time)
- Interactive filters and tables (cross-repo visibility)
- Mature auth with OAuth (GitHub SSO)

These capabilities are framework-agnostic — Chart.js, D3, and ECharts work in any framework; Tailwind works everywhere; OAuth is a protocol. The question is which framework delivers them with the lowest risk for a v1 timeline.

## Decision

React Router v7 in framework mode (TypeScript) + SQLite + Tailwind CSS.

Why React Router v7:
- **Production-ready and stable** (v7.13.x) — proven SSR, data loading, and server actions.
- **Mature auth ecosystem** — remix-auth works with React Router v7 and is battle-tested for GitHub OAuth.
- **Rich component ecosystem accelerates development** — shadcn/ui, Recharts, and similar libraries exist and work, but they are conveniences, not requirements. The hard capability requirements (charts, filters, auth) could be met other ways.
- **Backed by Shopify**, large community, steady release cadence.
- **Lowest risk for v1 timeline** — the framework, tooling, and ecosystem are all stable today.

SQLite provides zero-config persistence. Tailwind provides utility-first CSS.

## Alternatives Considered

- **Remix 3:** Evaluated as the natural upgrade from Remix 2. Rejected because: (1) it is experimental with no stable release as of March 2026 — building on it now means building on a moving target; (2) the auth ecosystem is unproven — no battle-tested auth library exists for Remix 3 yet, and while OAuth can be implemented manually, "mature auth" implies battle-tested libraries; (3) the component model is still being designed, so APIs may change under us. The capability requirements (charts, filters, auth) are framework-agnostic and could be met by Remix 3 in theory, but the framework isn't ready yet. **Revisit when Remix 3 reaches stable release.** The capability requirements are achievable once its ecosystem matures.
- **Go + HTMX:** Simpler architecture, single-binary deployment, same language as the CLI. Rejected because: (1) the dashboard is the user-facing product where UI richness matters; (2) HTMX is limited for interactive visualizations (dynamic charts, client-side filtering); (3) the React Router v7 ecosystem offers a more mature auth story and richer component ecosystem that accelerates v1 delivery; (4) React Router v7 is a learning goal for the developer. Go + HTMX remains a valid alternative if priorities change.
- **Python + FastAPI:** Excellent HTMX pairing but no single-binary deployment story. Adds a second language without the frontend ecosystem benefits.

## Consequences

**Positive:**
- Rich UI capabilities via the React ecosystem.
- Server-rendered by default, well-suited for a dashboard use case.
- GitHub OAuth is near-turnkey with remix-auth (compatible with React Router v7).
- Strong charting options (Recharts).
- Direct successor to Remix 2 — large community, active maintenance, Shopify backing.
- Good learning and resume value.

**Negative:**
- Two languages in the project (Go CLI + TypeScript dashboard). Mitigated by a clean boundary -- the shared data contract is a JSON schema, no shared types needed.
- Requires Node.js runtime (no single binary). Can be deployed directly with Node.js or via Docker.
- Heavier than HTMX (approximately 150-200KB JS vs 14KB). Acceptable for an internal dashboard.
- npm/build toolchain adds complexity compared to Go's `go build`.
