# Runbook — Dev Container Feature `ghcr.io/decionis/features/govern`

Owner-facing runbook per `rules/discovery.rules.md` Rule 4.5.

## Ordering (Rule 4.1 analog)

1. **CLI release first**: push tag `cli-vX.Y.Z` → `cli-publish.yml` creates
   the GitHub release with binaries + `SHA256SUMS`. The feature's
   `install.sh` downloads exactly those assets and refuses to install on a
   checksum mismatch.
2. **Repo public**: releases must be anonymously downloadable before the
   feature works for anyone else (and before any listing links resolve,
   Rule 1.1).
3. **Feature publication** to `ghcr.io/decionis/features/govern` as an OCI
   artifact:

   ```bash
   npx -y @devcontainers/cli features publish ./features --namespace decionis/features
   ```

   Re-read the devcontainers CLI/publishing docs on publication day
   (Rule 1.1 — do not trust this runbook's memory of their process); a
   GitHub Actions workflow (devcontainers/action) can automate this once
   publication credentials and repo visibility are settled.
4. **containers.dev index**: verify the current index submission process on
   the day; the feature's `documentationURL` must resolve publicly first.

## Local verification (no publication needed)

```bash
go build -o /tmp/decionis ./cmd/decionis
tar -czf /tmp/decionis_local.tar.gz -C /tmp decionis
docker run --rm -v /tmp/decionis_local.tar.gz:/tmp/cli.tar.gz:ro \
  -v "$PWD/features/govern:/f:ro" -e DECIONIS_CLI_TARBALL=/tmp/cli.tar.gz \
  debian:stable-slim sh /f/install.sh
```

## Record of publications

| Date | Version | Where | Notes |
| ---- | ------- | ----- | ----- |
| 2026-08-18 | CLI `cli-v0.1.0` (prerequisite, ordering step 1) | GitHub release | linux/darwin × amd64/arm64 + `SHA256SUMS`; `decionis_darwin_arm64` checksum-verified and executed locally (`decionis version` → 0.1.0). |
| 2026-08-18 | Feature `feature-govern-v0.1.0` | `ghcr.io/decionis/features/govern` via `feature-publish.yml` (green) | **Package starts private** — owner must make it public once in the ghcr package settings before anyone can install; containers.dev index submission after that (step 4). |
