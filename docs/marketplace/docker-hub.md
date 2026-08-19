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

## Webhooks (release announcements)

Docker Hub can call a webhook whenever a tag is pushed to a repository. The
Decionis control plane ships a receiver (upstream PR decionis/Decionis#910,
`POST /v1/public/webhooks/docker-hub/:token`) that turns a verified release
tag on `decionis/desktop-extension` into one announcement email per
connected workspace owner (consumed docker-desktop enrollment, minus
opt-outs); pushes to `decionis/mcp` are recorded but never emailed.

Security shape (Hub webhooks carry **no signature**):

- the URL's final segment is a secret (`DOCKER_HUB_WEBHOOK_TOKEN` on the
  control plane; unset disables the endpoint, wrong token answers 404);
- the payload is treated as a hint only — the tag is re-verified against
  Docker Hub's public tags API before anything announces;
- the payload's `callback_url` is never followed;
- a unique `(repo, tag)` claim makes Hub's retries and multi-arch
  double-fires idempotent (at most one email per release);
- unsubscribe links carry opaque tokens, never the address.

Setup status — **LIVE end to end (2026-08-19)**:

1. **Done** — `DOCKER_HUB_WEBHOOK_TOKEN` is a GitHub **environment secret**
   on `decionis/Decionis`'s `production` environment (repo secret slots are
   exhausted); the Deploy-to-GKE job forwards it into the api runtime. The
   same value sits in the owner's macOS keychain:
   `security find-generic-password -a decionis -s decionis-docker-hub-webhook-token -w`
2. **Done** — receiver deployed: the webhook code missed #910's merge
   snapshot by minutes and shipped as decionis/Decionis#914 (squash
   3061cd24; migration `0132` applied by the pipeline). A production 401
   was then found in front of ALL `/v1/public` docker-desktop routes — the
   auth hook's `PublicPaths` allowlist never listed them (this also broke
   #910's one-click connect flow live) — fixed in decionis/Decionis#915
   (squash 447a030e). Post-deploy probes verified: connect start → 302 to
   sign-in; wrong webhook token → 404 `not_found`; real token + bogus repo
   → 200 `ignored_repo`; unsubscribe → 200.
3. **Done** — Hub webhooks created (2026-08-19) on both repositories:
   `decionis-release-announce` on `decionis/desktop-extension` and
   `decionis/mcp`, pointing at the tokened receiver URL.
4. **Verified live (2026-08-19, ext-v0.1.4):** the tag push produced seven
   Hub deliveries, all `success 200` — one carried tag `0.1.4` and won the
   `(repo, tag)` claim (single announcement email), the rest landed as
   ignored/duplicate exactly as designed. (The 447a030e pipeline's one
   hiccup — a `decionis-agentops` rollout timeout, unrelated — cleared on
   rerun.)

The in-product update banner (extension ≥ 0.1.3) works independently of
this: it needs no webhook, no credentials, and no deploy.
