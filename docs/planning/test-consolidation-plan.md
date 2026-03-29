# Plan: Consolidate test infrastructure under `test/`

> Date: 2026-03-28

## Context

We're reorganizing the repo's external test infrastructure. Currently `e2e/` and `contract/` are separate top-level directories with duplicate configs (package.json, biome.jsonc, vitest.config.js, .tool-versions). The deliberation results confirmed `schemas/` and `testdata/` stay at root. The goal is:

```
test/
  contract/        ← Prism-based contract tests (offline)
  e2e/             ← real API E2E tests (credentials required)
  fixtures/        ← shared cross-component fixtures (future)
  package.json     ← merged deps
  vitest.config.js ← root config with inline project definitions (vitest v4)
  biome.jsonc
  .tool-versions
  README.md
```

## Steps

### 1. Create `test/` scaffold (new files)

- **`test/package.json`** — merge deps from both `e2e/package.json` and `contract/package.json`:
  - `@biomejs/biome`, `@stoplight/prism-cli`, `adm-zip`, `riteway`, `vitest`
  - Scripts: `test` (all), `test:contract`, `test:e2e`, `lint`, `lint:ci`, `lint:fix`
- **`test/vitest.workspace.js`** — define two projects referencing `contract/vitest.config.js` and `e2e/vitest.config.js`
- **`test/biome.jsonc`** — use e2e's version (broader glob pattern: `["tests/**/*", "**/*.test.*", "**/*.spec.*"]`). Note: e2e and contract configs differ in `overrides[0].include` — e2e is a superset, so use that.
- **`test/.tool-versions`** — `nodejs 22.22.1`

### 2. Create per-project vitest configs

- **`test/e2e/vitest.config.js`** — adapted from current `e2e/vitest.config.js`:
  - Add `name: 'e2e'` for workspace filtering
  - Keep .env loading logic (uses `import.meta.dirname`, still correct)
  - Keep `testTimeout: 120_000`, `fileParallelism: false`
- **`test/contract/vitest.config.js`** — minimal:
  - `name: 'contract'`, `testTimeout: 30_000`

### 3. Move test files

- **E2E tests**: `e2e/*.test.js` → `test/e2e/*.test.js`
  - Fix `binPath`: `../bin/quarantine` → `../../bin/quarantine` (3 files, line ~28-29 each)
- **E2E support files**: `e2e/.env.example` → `test/e2e/.env.example`
  - Update comment about default binary path
- **E2E Claude settings**: `e2e/.claude/settings.local.json` → `test/e2e/.claude/settings.local.json`
  - Update `../bin/quarantine` → `../../bin/quarantine` in the permission pattern
- **No contract test files to move** (none written yet, just the scaffold)

### 4. Write README files

- **`test/README.md`** — overview: two suites, make targets, when to use each
- **`test/e2e/README.md`** — adapted from existing `e2e/README.md` (credentials, env vars, running). Fix stale `../cli/bin/quarantine` reference to `../../bin/quarantine`
- **`test/contract/README.md`** — Prism-based contract tests, ADR-024 reference, running
- **`test/fixtures/README.md`** — placeholder explaining purpose (shared cross-component fixtures)

### 5. Update Makefile

Replace 6 current targets with:
```makefile
# --- Test Infrastructure (contract + e2e) ---
test-build:
    cd test && pnpm install

contract-test:
    cd test && pnpm run test:contract

e2e-test:
    cd test && pnpm run test:e2e

test-lint:
    cd test && pnpm run lint
```
Update aggregates:
- `lint-all`: replace `e2e-lint contract-lint` with `test-lint`
- `test-all`: replace individual targets, keep `contract-test` before `e2e-test`
- `.PHONY`: update target list

### 6. Update CI workflow

**`.github/workflows/ci.yml`**:
- Line 66: `package_json_file: e2e/package.json` → `package_json_file: test/package.json`
- Line 70: `working-directory: e2e` → `working-directory: test`
- Line 73: `make e2e-lint` → `make test-lint` (target renamed)
- Line 76: `make e2e-test` stays the same (Makefile still has this target)
- Add a `contract-test` step before the e2e step (runs without credentials, fast):
  ```yaml
  - name: Contract Tests
    run: make contract-test
  ```

### 7. Update CLAUDE.md

Commands section:
- `make test-build` (new — replaces e2e-build/contract-build)
- `make contract-test` and `make e2e-test` (already there, no change needed)
- `make test-lint` (replaces `make e2e-lint`)
- Remove `make e2e-lint` reference if present

### 8. Update ADR-024

- `contract/` → `test/contract/`
- Reference to `e2e/` → `test/e2e/`

### 9. Update skills (4 files)

**`.claude/skills/create-contract-test/SKILL.md`**:
- `contract/*.test.js` → `test/contract/*.test.js`
- `contract/<provider>-<feature>.test.js` → `test/contract/<provider>-<feature>.test.js`
- `cd contract && ./node_modules/.bin/prism` → `cd test && ./node_modules/.bin/prism`
- Spec path in template: `../schemas/` → `../../schemas/`  (from test/contract/)
- `make contract-test` (unchanged)
- `cd contract && pnpm run lint` → `cd test && pnpm run lint`
- **Prism binary path in template**: `./node_modules/.bin/prism` → `../node_modules/.bin/prism` (node_modules is at `test/`, not `test/contract/`)
- Prism spawn cwd: `import.meta.dirname` still works (file is in test/contract/)

**`.claude/skills/create-e2e-test/SKILL.md`**:
- `e2e/*.test.js` → `test/e2e/*.test.js`
- `e2e/<provider>-<feature>.test.js` → `test/e2e/<provider>-<feature>.test.js`
- `cd e2e && pnpm run lint` → `cd test && pnpm run lint`
- `make e2e-test` (unchanged)

**`.claude/skills/implement-milestone/SKILL.md`**:
- `contract/*.test.js` → `test/contract/*.test.js`
- `e2e/*.test.js` → `test/e2e/*.test.js`

**`.claude/skills/verify-milestone/SKILL.md`**:
- Same path updates as implement-milestone
- Report template: `contract/*.test.js` → `test/contract/*.test.js`, `e2e/*.test.js` → `test/e2e/*.test.js`

### 10. Update docs (non-skill)

- `docs/specs/test-strategy.md` lines 76, 78, 88: `e2e/` → `test/e2e/`
- `docs/milestones/m1.md` line 47, `m4.md` line 49, `m5.md` line 51, `m6.md` line 111: update e2e references
- `docs/research/contract-testing-prism-spike.md` line 15: `e2e/` directory reference

### 11. Delete old directories

```bash
rm -rf e2e/ contract/
```

### 12. Install and verify

```bash
cd test && pnpm install
make contract-test   # should pass (no tests yet, but vitest exits 0)
make test-lint       # should pass
```

## Execution strategy

- **Steps 1-4** (create new files + READMEs): all parallel via Write tool
- **Step 3** (move test files): parallel writes with path fixes
- **Steps 5-10** (update existing files): parallel edits where independent
- **Step 11** (delete old dirs): single bash command
- **Step 12** (install + verify): sequential

## Verification

1. `cd test && pnpm install` succeeds
2. `make test-lint` passes
3. `make contract-test` exits cleanly (no test files yet)
4. `git diff --stat` shows expected file moves
5. No remaining references to top-level `e2e/` or `contract/` (grep check)
