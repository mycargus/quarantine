# Release Plan

> Implements GitHub Releases for the quarantine CLI binary.
> Adapted from the riteway-golang release model (human-triggered tag push,
> CHANGELOG extraction, environment gating, AI block hook) with GoReleaser
> for cross-compiled binary distribution.

## Files to create

| File | Purpose |
|------|---------|
| `CHANGELOG.md` | Release notes source (Keep a Changelog format) |
| `RELEASING.md` | Human-facing release process documentation |
| `scripts/release.sh` | Local preflight checks, annotated tag creation, push |
| `.github/workflows/release.yml` | Tag-triggered workflow: validate, CI gate, GoReleaser |
| `.goreleaser.yml` | GoReleaser cross-compilation and archive config |
| `.claude/hooks/block-publish.sh` | PreToolUse hook blocking AI from triggering releases |

## Files to modify

| File | Change |
|------|--------|
| `Makefile` | Add `release` target delegating to `scripts/release.sh` |

## Key decisions

### GoReleaser over hand-rolled cross-compilation

GoReleaser handles `GOOS`/`GOARCH` matrix, archive creation (`.tar.gz` for
linux/darwin), SHA256 checksums, and GitHub Release upload in one step. The
alternative is ~50 lines of shell scripting. GoReleaser is the industry
standard for Go CLI releases and less maintenance.

### `cli/` subdirectory handling

The Go module lives in `cli/`, not the repo root. GoReleaser config needs:

```yaml
builds:
  - dir: cli
    main: ./cmd/quarantine
```

### Targets

`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.

Windows deferred — no user demand. M8 mentions it as optional.

### Release workflow runs full CI first

The workflow runs CLI lint+test and contract tests before GoReleaser creates the
release. This is broader than riteway-golang (which only runs Go tests) because
quarantine has multiple components.

### `go install` works for free

Once the module is tagged, users can install without release artifacts:

```
go install github.com/mycargus/quarantine/cli/cmd/quarantine@v0.1.0
```

Document this in RELEASING.md as an alternative install method.

### Dashboard is out of scope

Dashboard has its own deployment story (Docker image, M8). The release workflow
produces CLI binaries only.

## `.goreleaser.yml`

```yaml
project_name: quarantine
builds:
  - dir: cli
    main: ./cmd/quarantine
    binary: quarantine
    goos: [linux, darwin]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w -X main.version={{.Version}}
archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
checksum:
  name_template: checksums.txt
release:
  draft: false
  prerelease: auto
changelog:
  disable: true  # We extract from CHANGELOG.md instead
```

## Release workflow sequence

```
Human                          GitHub Actions
──────                         ──────────────
make release VERSION=v0.1.0
  │
  ├─ scripts/release.sh
  │   ├─ validate version format (vX.Y.Z)
  │   ├─ verify CHANGELOG.md has entry
  │   ├─ verify clean working tree
  │   ├─ verify tag doesn't exist
  │   ├─ run make check (lint + typecheck)
  │   ├─ run make cli-test
  │   ├─ prompt for confirmation
  │   ├─ create annotated tag
  │   └─ git push origin vX.Y.Z ───────────► on: push: tags: v*
                                                │
                                                ├─ checkout
                                                ├─ verify CHANGELOG entry
                                                ├─ CLI: lint + test
                                                ├─ contract tests
                                                ├─ GoReleaser:
                                                │   ├─ cross-compile (4 targets)
                                                │   ├─ create archives
                                                │   ├─ generate checksums.txt
                                                │   └─ upload to GitHub Release
                                                └─ done
```

## `scripts/release.sh` outline

Adapted from riteway-golang's `scripts/release.sh`:

1. Validate `$1` matches `vX.Y.Z` pattern
2. Extract version without `v` prefix
3. Verify `CHANGELOG.md` has `## [X.Y.Z]` section
4. Extract release notes from CHANGELOG
5. Verify working tree is clean (`git status --porcelain`)
6. Verify tag doesn't already exist
7. Run `make check` (lint + typecheck)
8. Run `make cli-test`
9. Run `make contract-test`
10. Verify `go mod tidy` produces no changes (in `cli/`)
11. Print summary with release notes
12. Prompt for confirmation
13. Create annotated tag: `git tag -a vX.Y.Z -m "Release vX.Y.Z"`
14. Push tag: `git push origin vX.Y.Z`

## `.claude/hooks/block-publish.sh`

PreToolUse hook that blocks these commands inside Claude Code:

- `make release`
- `bash scripts/release.sh`
- `git push origin v*` (version tags)
- `gh release create`

Human terminal usage is unaffected.

## Post-release: update fixture repo workflow

Once the first release is published, update the fixture repo's
`upload-test-artifact.yml` to download the release binary instead of building
from source:

```yaml
- name: Install quarantine CLI
  run: |
    gh release download --repo mycargus/quarantine \
      --pattern 'quarantine_*_linux_amd64.tar.gz' --output quarantine.tar.gz
    tar xzf quarantine.tar.gz
    mv quarantine /usr/local/bin/quarantine
```

Remove the `actions/setup-go` step — no longer needed.

## One-time setup: GitHub environment

Configure the `release` environment in the quarantine repo:

1. **Settings → Environments → New environment** → name it `release`
2. Under **Deployment branches and tags**, change to **Selected branches and tags**
3. **Add deployment branch or tag rule** → enter `v*` as a **tag** pattern
4. Save

---

*References: [riteway-golang RELEASING.md](https://github.com/mycargus/riteway-golang/blob/main/RELEASING.md),
[riteway-golang release.yml](https://github.com/mycargus/riteway-golang/blob/main/.github/workflows/release.yml),
[M8 milestone](docs/milestones/m8.md)*
