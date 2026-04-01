# TODO

Deferred work noted during the contract testing and dogfooding session
(2026-03-31). Not prioritized — just captured so nothing is lost.

## Init improvements

### Auto-detect package manager and test framework

`quarantine init` currently prompts for the test framework. It should detect
it from the project:

- **Package manager:** `package-lock.json` → npm, `pnpm-lock.yaml` → pnpm,
  `bun.lockb` → bun, `yarn.lock` → yarn, `Gemfile.lock` → bundler
- **Test framework:** scan `package.json` devDependencies for jest/vitest,
  or `Gemfile` for rspec
- **JUnit XML path:** check existing config files (`jest.config.js` with
  `jest-junit`, `.rspec` with formatter flags, `vitest.config.ts` with
  reporter config)
- Present detected values as defaults, user confirms or overrides
- Open question: what to do when multiple frameworks are detected (e.g., both
  jest and vitest in `package.json`)

### Non-interactive init (`--yes` + args)

`quarantine init` assumes an interactive TTY. It should support:

- `quarantine init --yes` — accept all detected/default values without prompting
- `quarantine init --framework jest --yes` — override specific values
- Enables use in CI, scripts, Docker builds, non-TTY environments

## Rerun command

### Remove PATH workaround from fixture repo

The fixture repo (`quarantine-test-fixture`) workflow still has
`export PATH="${{ github.workspace }}/node_modules/.bin:$PATH"`. Now that the
CLI uses `npx jest` by default, this workaround should be removed to prove
`npx` actually works without it.

### Document rerun_command for non-standard setups

`rerun_command` examples for pnpm, bun, and custom configs should be added to
docs (README, cli-spec, config-schema). Examples:

```yaml
# pnpm
rerun_command: "pnpm exec jest --testNamePattern '{name}'"

# bun
rerun_command: "bunx jest --testNamePattern '{name}'"

# custom jest config
rerun_command: "npx jest --config jest.ci.config.js --testNamePattern '{name}'"
```

### Update config-schema.md auto-detected commands table

The table at `docs/specs/config-schema.md` line 393 still lists bare `jest`,
`rspec`, `vitest` as auto-detected commands. Update to reflect `npx jest`,
`bundle exec rspec`, `npx vitest`.

## Testify findings (runner package)

From testify review of `cli/internal/runner/runner_test.go`:

- **MEDIUM:** `TestEscapeJestPattern` missing backslash coverage — the
  `jestRegexSpecialChars` slice lists `\\` first to prevent double-escaping,
  but no test asserts `foo\bar` → `foo\\bar`.
- **LOW:** `SplitShellArgs` unclosed-quote behavior not asserted.
- **LOW:** `Run` non-`ExitError` Wait branch untested (line 61).
- **LOW:** Context cancellation not tested for `Run`.
- **LOW:** Custom template test at `runner_test.go:87` passes `runner.Jest`
  when framework is irrelevant (`customTemplate != ""` ignores it). Minor
  clarity issue — passing an empty string would make intent clearer.

## Release workflow

See `RELEASE-PLAN.md` for the full plan (GoReleaser, CHANGELOG, scripts,
environment gating, AI block hook).
