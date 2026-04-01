#!/usr/bin/env bash
# Local pre-flight checks and tag creation for releases.
# This script does NOT perform the release itself -- that happens in the
# GitHub Actions release workflow triggered by the tag push.
set -euo pipefail

VERSION="${1:-}"

# 1. Validate version format
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Usage: $0 <version>"
  echo "Example: $0 v0.1.0"
  exit 1
fi

# 2. Extract version without v prefix
VERSION_NUM="${VERSION#v}"

# 3. Verify CHANGELOG.md has entry for this version
if ! grep -q "^## \[${VERSION_NUM}\]" CHANGELOG.md; then
  echo "Error: CHANGELOG.md has no entry for [${VERSION_NUM}]"
  echo "Add a '## [${VERSION_NUM}]' section to CHANGELOG.md before releasing."
  exit 1
fi

# 4. Extract release notes for this version
RELEASE_NOTES=$(awk "/^## \[${VERSION_NUM}\]/{found=1; next} found && /^## \[/{exit} found{print}" CHANGELOG.md)

# 5. Verify working tree is clean
if [[ -n "$(git status --porcelain)" ]]; then
  echo "Error: Working tree is not clean. Commit or stash changes first."
  git status --short
  exit 1
fi

# 6. Verify tag doesn't already exist
if git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo "Error: Tag $VERSION already exists."
  exit 1
fi

# 7. Run lint + typecheck
echo "Running make check..."
make check

# 8. Run CLI tests
echo "Running make cli-test..."
make cli-test

# 9. Run contract tests
echo "Running make contract-test..."
make contract-test

# 10. Verify go mod tidy produces no changes
echo "Verifying go.mod is tidy..."
go mod tidy
if [[ -n "$(git status --porcelain go.mod go.sum)" ]]; then
  echo "Error: go mod tidy produced changes. Commit them first."
  git checkout -- go.mod go.sum
  exit 1
fi

# 11. Print summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Release: $VERSION"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Release notes:"
echo "$RELEASE_NOTES"
echo ""

# 12. Prompt for confirmation
read -r -p "Create and push tag $VERSION? [y/N] " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
  echo "Aborted."
  exit 0
fi

# 13. Create annotated tag
git tag -a "$VERSION" -m "Release $VERSION"

# 14. Push tag
git push origin "$VERSION"

REMOTE_URL=$(git remote get-url origin)
REPO_URL=$(echo "$REMOTE_URL" | sed -E 's|git@github\.com:|https://github.com/|; s|\.git$||')

echo ""
echo "Tag $VERSION pushed."
echo "Monitor the release workflow at:"
echo "${REPO_URL}/actions"
