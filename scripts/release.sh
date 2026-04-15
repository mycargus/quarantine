#!/usr/bin/env bash
# Local pre-flight checks and tag creation for releases.
# This script does NOT perform the release itself -- that happens in the
# GitHub Actions release workflow triggered by the tag push.
set -euo pipefail

INPUT="${1:-}"

# Helper: run a command silently, show pass/fail, dump output on failure
run_step() {
  local label="$1"
  shift
  printf "  %-40s" "$label"
  if output=$("$@" 2>&1); then
    echo "✓"
  else
    echo "FAIL"
    echo ""
    echo "$output"
    exit 1
  fi
}

# 1. Strip leading v if present, then validate
VERSION_NUM="${INPUT#v}"
if [[ ! "$VERSION_NUM" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
  echo "Usage: $0 <version>"
  echo "Examples: $0 0.1.0   $0 0.1.0-rc1"
  exit 1
fi
TAG="v${VERSION_NUM}"
# Base version without prerelease suffix (e.g. "0.1.0" from "0.1.0-rc1")
BASE_VERSION="${VERSION_NUM%%-*}"

# 2. Verify CHANGELOG.md has entry for this version (uses base version, not rc suffix)
if ! grep -q "^## \[${BASE_VERSION}\]" CHANGELOG.md; then
  echo "Error: CHANGELOG.md has no entry for [${BASE_VERSION}]"
  echo "Add a '## [${BASE_VERSION}]' section to CHANGELOG.md before releasing."
  exit 1
fi

# 3. Extract release notes for this version
RELEASE_NOTES=$(awk "/^## \[${BASE_VERSION}\]/{found=1; next} found && /^## \[/{exit} found && /^\[.*\]: /{exit} found{print}" CHANGELOG.md)

# 4. Verify working tree is clean
if [[ -n "$(git status --porcelain)" ]]; then
  echo "Error: Working tree is not clean. Commit or stash changes first."
  git status --short
  exit 1
fi

# 5. Verify tag doesn't already exist
if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "Error: Tag $TAG already exists."
  exit 1
fi

# 6. Run lint, typecheck, and tests (skipped for final releases — rc already validated)
IS_RC=false
if [[ "$VERSION_NUM" == *-* ]]; then
  IS_RC=true
fi

if [[ "$IS_RC" == "true" ]]; then
  echo "Pre-flight checks:"
  run_step "lint (cli)" make cli-lint
  run_step "lint (dashboard)" make dash-lint
  run_step "lint (test)" make test-lint
  run_step "typecheck (dashboard)" make dash-typecheck
  run_step "test (cli)" make cli-test
  run_step "test (dashboard)" make dash-test-ci
  run_step "test (contract)" make contract-test
  run_step "go mod tidy" go mod tidy

  # 7. Verify go mod tidy produced no changes
  if [[ -n "$(git status --porcelain go.mod go.sum)" ]]; then
    echo "Error: go mod tidy produced changes. Commit them first."
    git checkout -- go.mod go.sum
    exit 1
  fi
else
  echo "Skipping local pre-flight (rc already validated)."
fi

# 8. Print summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Release: $TAG"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Release notes:"
echo "$RELEASE_NOTES"
echo ""

# 9. Prompt for confirmation (requires interactive terminal)
if [[ ! -t 0 ]]; then
  echo "Error: stdin is not a terminal. Run this script interactively."
  exit 1
fi
read -r -p "Create and push tag $TAG? [y/N] " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
  echo "Aborted."
  exit 0
fi

# 10. Create annotated tag
git tag -a "$TAG" -m "Release $TAG"

# 11. Push tag
git push origin "$TAG"

REMOTE_URL=$(git remote get-url origin)
REPO_URL=$(echo "$REMOTE_URL" | sed -E 's|git@github\.com:|https://github.com/|; s|\.git$||')

echo ""
echo "Tag $TAG pushed."
echo "Monitor the release workflow at:"
echo "${REPO_URL}/actions"
