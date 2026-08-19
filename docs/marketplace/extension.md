# Runbook — Decionis Docker Desktop extension

Owner-facing runbook per `rules/discovery.rules.md` Rule 4.5 for building,
locally verifying, and submitting the extension.

## Local build, validate, install

```bash
docker build -f extension/Dockerfile -t decionis/desktop-extension:0.1.0 .
docker extension validate decionis/desktop-extension:0.1.0
docker extension install decionis/desktop-extension:0.1.0
```

- Docker Desktop blocks non-marketplace installs by default. For local
  testing: Docker Desktop → Settings → Extensions → uncheck **"Allow only
  extensions distributed through the Docker Marketplace"**, then install.
  (Verified blocked on this machine 2026-08-18 until that setting changes.)
- Iterate with `docker extension update <image>` after rebuilds;
  `docker extension rm decionis/desktop-extension` removes it.
- UI-only iteration without Desktop: `internal` daemon on loopback
  (`go run ./cmd/decionis-daemon -listen 127.0.0.1:8787 -data /tmp/decionis-dev`)
  plus `npm run dev` in `extension/ui` (vite proxies `/api`).

## Validation status (sweep 2026-08-18)

`docker extension validate` passes metadata, labels, and semver locally. Two
checks remain **publish-gated** and fail on any local single-arch build:

- [ ] `com.docker.extension.screenshots` — requires real captured screenshots
      at publicly accessible URLs. Plan: commit captures under
      `docs/screenshots/` and reference their `raw.githubusercontent.com`
      URLs once the repo is public; the validator live-checks them
      (Rule 1.1 enforced by Docker's own tooling).
- [x] Multi-platform image — `extension-publish.yml` pushed
      `linux/amd64,linux/arm64` on 2026-08-18; `docker extension validate`
      against the published `0.1.0` now passes the multiplatform check.
      Screenshots are the single remaining validate item.

## Docker Hub home

The extension's Hub repository is **`decionis/desktop-extension`** — created
2026-08-18 (public). The stray `decionis/docker` Hub repository was deleted
by the owner the same day (Docker Hub does not support renaming
repositories).

First release, 2026-08-18: `ext-v0.1.0` → `decionis/desktop-extension:0.1.0`
+ `:latest`, multi-arch (linux/amd64 + linux/arm64), provenance + SBOM,
cosign keyless; index digest
`sha256:8894ebb49feb49ceac4f07ab2c624789bc11ca8b2ebce846df7b414828dfa563`.
Short description and overview (from `extension/overview.md`) set the same
day and verified via the Hub API.

Later releases (same pipeline, same guarantees):

- 2026-08-18 `ext-v0.1.1` → `:0.1.1` — screenshots label; first fully green
  `docker extension validate`.
- 2026-08-18 `ext-v0.1.2` → `:0.1.2` — enrollment-token connect (single-use
  `dcn_enroll_*` token exchanged by the daemon; org ID + API key demoted to
  Advanced).
- 2026-08-18 `ext-v0.1.3` → `:0.1.3` — update banner: the daemon checks the
  extension's own public Docker Hub tags listing (anonymous, cached 6 h,
  fail-open — an unreachable listing claims nothing) and the UI shows a
  dismissible per-version banner with the `docker extension update` command.
- 2026-08-19 `ext-v0.1.4` → `:0.1.4` — one-click connect: "Continue in
  browser" starts the control plane's docker-desktop flow (sign-in +
  auto-provision, RFC 8252 loopback redirect to the daemon's listener on
  host `127.0.0.1:53719`, state-bound single-use `/enroll`, minted token
  exchanged exactly like a pasted one). Also fixes the enrollment exchange
  endpoint path (`/connectors/…` → `/v1/connectors/…`): pasted-token
  connects against the hosted plane had been failing since 0.1.2 — earlier
  test fakes mirrored the client's wrong path, so only a test that drove
  the real HTTP client caught it. First release to arrive through the Hub
  release webhook (announcement email to connected owners).

## Before marketplace submission (Rule 1.1 gates)

- [x] Repo public: `https://github.com/decionis/docker` resolves (HTTP 200,
      2026-08-18) and is restored in `com.docker.extension.additional-urls`.
- [ ] Icon: `com.docker.desktop.extension.icon` currently points at the only
      verified hosted asset (`https://decionis.com/favicon.ico`). Host the
      512×512 `extension/decionis-logo.png` at a public decionis.com URL and
      swap the label.
- [ ] Screenshots captured from the real extension inside Docker Desktop
      (feed + dossier inspector), hosted, label added.
- [ ] Multi-arch image pushed to Docker Hub with provenance/SBOM (extend the
      publish workflow when the Hub repo exists).
- [x] `docker extension validate decionis/desktop-extension:0.1.1` — **fully
      green** (2026-08-18): all checks pass, including screenshots
      (SHA-pinned raw URLs) and multi-platform.
- [ ] Submit via Docker's extension submission form (URL given by the
      validator itself): https://www.docker.com/products/extensions/submissions/
      — a Docker web form, owner-owned; record the submission below.

## Record of submissions

| Date | PR | Status | Notes |
| ---- | -- | ------ | ----- |
| 2026-08-18 | https://github.com/docker/extensions-submissions/issues/257 | Submitted; **blocked by Docker's own pipeline** | Owner submitted via the web form. TOS accepted, issue parsed; the `validate` job fails at *Set up job* because the workflow's transitive actions (`mavrosxristoforos/get-xml-info@1.1.1`, `actions/cache@v3`) violate the repo's SHA-pinning policy — every submission since at least 2026-07-22 fails identically (#248, #255). Diagnosis commented on the issue (2026-08-18); the workflow supports revalidation via comment once Docker fixes the pins. Our side is clean: `docker extension validate` fully green on 0.1.1. |
