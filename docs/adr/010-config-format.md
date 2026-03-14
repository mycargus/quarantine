# ADR-010: YAML Configuration Format

**Status:** Accepted
**Date:** 2026-03-14

## Context

The CLI needs a configuration file in the repo root. Need to choose a format.

## Decision

YAML. File: `quarantine.yml` in repo root.

Example:

```yaml
version: 1
framework: jest     # auto-detected if omitted; v1 supports: rspec, jest, vitest
retries: 3          # number of retries before flagging as flaky

github:
  owner: my-org
  repo: my-project  # auto-detected from git remote if omitted

storage:
  backend: branch          # or "actions-cache"
  branch: quarantine/state # only if backend: branch
```

CLI also supports a `quarantine validate` command to verify config and print resolved values (including auto-detected fields).

## Alternatives Considered

- **JSON:** No comments (significant drawback for config files), familiar but more verbose (braces, quotes). `yq` validates YAML equivalently to `jq` for JSON, and `quarantine validate` provides built-in validation.
- **TOML:** Good format, supports comments, but less familiar in CI ecosystem (CI configs are predominantly YAML). Lower tooling availability.
- **No config file (all CLI flags):** Works for simple cases but does not scale for project-level defaults. CLI flags still override config file values.

## Consequences

- (+) Comments allow documenting why settings are configured a certain way.
- (+) Familiar to anyone who writes CI configs (GitHub Actions, GitLab CI, etc.).
- (+) Less verbose than JSON.
- (+) Well-supported in Go (gopkg.in/yaml.v3).
- (-) Indentation sensitivity can cause subtle errors. Mitigated by `quarantine validate` command.
- (-) YAML has well-known footguns (the Norway problem, implicit type coercion). Mitigated by strict parsing mode and limited schema.
