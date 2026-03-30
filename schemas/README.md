# Schemas

Formal definitions for shared data formats and external API shapes.

| File | Format | Defines |
|------|--------|---------|
| `test-result.schema.json` | JSON Schema (draft 2020-12) | `.quarantine/results.json` — test run output from CLI, consumed by dashboard |
| `quarantine-state.schema.json` | JSON Schema (draft 2020-12) | `quarantine.json` — quarantine state on the `quarantine/state` branch |
| `quarantine-config.schema.json` | JSON Schema (draft 2020-12) | `quarantine.yml` — CLI configuration |
| `github-api-artifacts.json` | OpenAPI 3.x | GitHub Artifacts API subset — vendored spec for Prism contract tests |

For full context on each schema — what it validates, who produces and consumes the data, and how it's tested — see [docs/specs/contracts.md](../docs/specs/contracts.md).
