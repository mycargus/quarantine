#!/usr/bin/env bash
# Download and install the quarantine CLI binary.
# Usage:
#   curl -sSL https://raw.githubusercontent.com/mycargus/quarantine/main/scripts/install.sh | bash
#   curl -sSL .../install.sh | VERSION=v0.1.0 bash
#   curl -sSL .../install.sh | INSTALL_DIR=./bin bash
set -euo pipefail

REPO="mycargus/quarantine"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

cleanup() {
  if [[ -n "${WORK_DIR:-}" ]]; then
    rm -rf "$WORK_DIR"
  fi
}
trap cleanup EXIT

die() {
  echo "Error: $1" >&2
  exit 1
}

# Detect OS
case "$(uname -s)" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *)      die "Unsupported OS: $(uname -s). Only linux and darwin are supported." ;;
esac

# Detect architecture
case "$(uname -m)" in
  x86_64)       ARCH="amd64" ;;
  amd64)        ARCH="amd64" ;;
  aarch64)      ARCH="arm64" ;;
  arm64)        ARCH="arm64" ;;
  *)            die "Unsupported architecture: $(uname -m). Only amd64 and arm64 are supported." ;;
esac

# Require curl
command -v curl >/dev/null 2>&1 || die "curl is required but not installed."

# Resolve latest version if needed
if [[ "$VERSION" == "latest" ]]; then
  VERSION=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
  if [[ -z "$VERSION" ]]; then
    die "Could not resolve latest version. Check https://github.com/${REPO}/releases"
  fi
  echo "Latest version: ${VERSION}"
fi

# Strip v prefix for the binary filename (GoReleaser uses version without v)
VERSION_NUM="${VERSION#v}"
BINARY_NAME="quarantine_${VERSION_NUM}_${OS}_${ARCH}"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

# Create temp directory
WORK_DIR=$(mktemp -d)

echo "Downloading quarantine ${VERSION} for ${OS}/${ARCH}..."

# Download binary and checksums
curl -sSL "${BASE_URL}/${BINARY_NAME}" -o "${WORK_DIR}/${BINARY_NAME}" \
  || die "Failed to download binary. Check that ${VERSION} exists at https://github.com/${REPO}/releases"
curl -sSL "${BASE_URL}/checksums.txt" -o "${WORK_DIR}/checksums.txt" \
  || die "Failed to download checksums."

# Verify checksum
echo "Verifying checksum..."
cd "$WORK_DIR"
if command -v sha256sum >/dev/null 2>&1; then
  grep "${BINARY_NAME}" checksums.txt | sha256sum --check --quiet
elif command -v shasum >/dev/null 2>&1; then
  grep "${BINARY_NAME}" checksums.txt | shasum -a 256 --check --quiet
else
  die "Neither sha256sum nor shasum found. Cannot verify checksum."
fi

# Install
mkdir -p "$INSTALL_DIR"
mv "${BINARY_NAME}" "${INSTALL_DIR}/quarantine"
chmod +x "${INSTALL_DIR}/quarantine"

echo "Installed quarantine ${VERSION} to ${INSTALL_DIR}/quarantine"
