# Runbook — publishing `decionis/mcp` to Docker Hub

Owner-facing runbook per `rules/discovery.rules.md` Rule 4.5: account
custody, orderings, and every user-owned manual step live here.

## Account custody (fill in before first publish)

- Docker Hub organization: `decionis` — owner: _record account owner + 2FA
  custody here_.
- CI credentials: repository secrets `DOCKERHUB_USERNAME` and
  `DOCKERHUB_TOKEN` (a scoped access token with read/write on `decionis/mcp`
  only — never an account password). Until these exist, `mcp-publish.yml`
  verifies the release and skips pushing (silent no-op, Rule 3.2).

## Preconditions for the FIRST publish (Rule 1.1 gate)

- [x] `https://github.com/decionis/docker` is public — verified HTTP 200 on
      the 2026-08-18 publish-day sweep.
- [x] Docker Hub repo `decionis/mcp` created under the org (Hub API 200,
      2026-08-18).
- [x] `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` repo secrets present
      (verified via `gh secret list`, 2026-08-18).
- [x] Every URL in `mcp-server/overview.md` re-verified live on publish day
      (2026-08-18); sweep comment updated.

## Versioning

- The image version tracks the wrapped `@decionis/mcp` version: package
  `0.1.2` → tag `mcp-v0.1.2` → image `decionis/mcp:0.1.2`.
- Image-only changes on the same upstream version append a revision:
  `mcp-v0.1.2-r1` → `decionis/mcp:0.1.2-r1`.
- Version tags are immutable (Rule 3.3): never rebuild a published tag;
  publish a new revision instead. `latest` follows the newest release.

## Publish flow

1. Upstream bump: edit `mcp-server/package.json` to the new exact version,
   regenerate `package-lock.json`
   (`cd mcp-server && npm install --package-lock-only --ignore-scripts`),
   open a PR. The smoke test is the tool-inventory drift gate (Rule 1.4) —
   if upstream added or renamed tools, update `SmokeClient.mjs`'s
   `EXPECTED_TOOLS`, the overview's Tools table, and the catalog entry in the
   same PR.
2. Merge, then push tag `mcp-vX.Y.Z`. `.github/workflows/mcp-publish.yml`
   re-verifies (hadolint ran on PR; smoke + trivy run again), then — with
   credentials present — pushes linux/amd64 + linux/arm64 with provenance
   and SBOM attestations and signs the digest with keyless cosign.
3. Manual, user-owned: paste `mcp-server/overview.md` into the Docker Hub
   repository description (the committed file is the source of truth,
   Rule 2.1 — never edit only in the Hub UI).
4. Post-publish verification:
   - `docker pull decionis/mcp:X.Y.Z` from a clean machine.
   - `cosign verify docker.io/decionis/mcp:X.Y.Z --certificate-identity-regexp 'github.com/decionis/docker' --certificate-oidc-issuer https://token.actions.githubusercontent.com`
   - Record the release date and digest below.

## Release log

| Date | Tag | Digest | Notes |
| ---- | --- | ------ | ----- |
| 2026-08-18 | `mcp-v0.1.2` → `decionis/mcp:0.1.2` + `:latest` | `sha256:494710a805399eafedf2c2e77b75249835afd0c159a26ef02f58911561135a44` | First publish. Multi-arch (linux/amd64 + linux/arm64), provenance + SBOM attestations, cosign keyless signature (created in CI; verify locally with the cosign command above once cosign is installed). Full smoke suite re-run green against the published image. Overview paste into the Hub description still pending (user-owned). |
