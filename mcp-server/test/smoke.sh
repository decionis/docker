#!/usr/bin/env bash
# Smoke tests for the decionis/mcp image (rules/security.rules.md Rule 7.4):
# image posture (non-root) plus the stdio protocol suite in SmokeClient.mjs
# (handshake, exact tool inventory, blocked-evaluate round-trip under
# --network none --read-only).
# Usage: smoke.sh <image>
set -euo pipefail

IMAGE="${1:?usage: smoke.sh <image>}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "== non-root user"
uid="$(docker run --rm --entrypoint id "$IMAGE" -u)"
if [ "$uid" != "1000" ]; then
  echo "FAIL: image runs as uid $uid, expected non-root uid 1000" >&2
  exit 1
fi
echo "ok: runs as uid 1000 (node)"

echo "== stdio handshake, tool inventory, evaluate (--network none --read-only)"
node "$HERE/SmokeClient.mjs" "$IMAGE"

echo "All smoke tests passed for $IMAGE"
