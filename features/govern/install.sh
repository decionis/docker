#!/bin/sh
# Dev Container Feature install script: installs the decionis CLI from this
# repo's GitHub releases (asset naming per .github/workflows/cli-publish.yml),
# with mandatory checksum verification. DECIONIS_CLI_TARBALL overrides the
# download with a local tarball (tests, air-gapped installs).
set -eu

VERSION="${VERSION:-latest}"
REPO="decionis/docker"
BIN_DIR="/usr/local/bin"

log() { echo "decionis feature: $1"; }
fail() { echo "decionis feature: ERROR: $1" >&2; exit 1; }

install_from_tarball() {
  tarball="$1"
  workdir="$(mktemp -d)"
  tar -xzf "$tarball" -C "$workdir" decionis || fail "tarball $tarball does not contain the decionis binary"
  install -m 0755 "$workdir/decionis" "$BIN_DIR/decionis"
  rm -rf "$workdir"
  log "installed $("$BIN_DIR/decionis" version 2>/dev/null || echo decionis) to $BIN_DIR/decionis"
}

if [ -n "${DECIONIS_CLI_TARBALL:-}" ]; then
  log "installing from local tarball $DECIONIS_CLI_TARBALL"
  install_from_tarball "$DECIONIS_CLI_TARBALL"
  exit 0
fi

case "$(uname -m)" in
  x86_64 | amd64) ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *) fail "unsupported architecture $(uname -m)" ;;
esac
ASSET="decionis_linux_${ARCH}.tar.gz"

if [ "$VERSION" = "latest" ]; then
  BASE="https://github.com/${REPO}/releases/latest/download"
else
  BASE="https://github.com/${REPO}/releases/download/cli-v${VERSION}"
fi

fetch() {
  url="$1"
  out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$out" "$url" || fail "download failed: $url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url" || fail "download failed: $url"
  else
    fail "neither curl nor wget is available"
  fi
}

command -v tar >/dev/null 2>&1 || fail "tar is required"
command -v sha256sum >/dev/null 2>&1 || fail "sha256sum is required"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

log "downloading ${BASE}/${ASSET}"
fetch "${BASE}/${ASSET}" "${workdir}/${ASSET}"
fetch "${BASE}/SHA256SUMS" "${workdir}/SHA256SUMS"

( cd "$workdir" && grep " ${ASSET}\$" SHA256SUMS | sha256sum -c - ) \
  || fail "checksum verification failed for ${ASSET} — refusing to install"

install_from_tarball "${workdir}/${ASSET}"
