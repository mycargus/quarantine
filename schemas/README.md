# Schemas

Formal definitions for shared data formats and external API shapes.

| File | Format | Defines |
|------|--------|---------|
| `test-result.schema.json` | JSON Schema (draft 2020-12) | `.quarantine/results.json` — test run output from CLI, consumed by dashboard |
| `quarantine-state.schema.json` | JSON Schema (draft 2020-12) | `quarantine.json` — quarantine state on the `quarantine/state` branch |
| `quarantine-config.schema.json` | JSON Schema (draft 2020-12) | `quarantine.yml` — CLI configuration |
| `github-api.json` | OpenAPI 3.x | All GitHub API endpoints used by CLI and dashboard — vendored spec for Prism contract tests |

## github-api.json provenance

`github-api.json` is a curated extract. The `info.x-source-commit` field in the
spec records which commit from `github/rest-api-description` the endpoint
definitions were sourced from. Update this field whenever you refresh endpoints
from the official spec.

For full context on each schema — what it validates, who produces and consumes the data, and how it's tested — see [docs/specs/contracts.md](../docs/specs/contracts.md).
