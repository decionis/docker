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
- [ ] Multi-platform image — the publish pipeline pushes
      `linux/amd64,linux/arm64`; local builds are single-arch by design.

## Before marketplace submission (Rule 1.1 gates)

- [ ] Repo public: `https://github.com/decionis/docker` resolves (also
      re-add it to `com.docker.extension.additional-urls` — it was removed
      because the validator correctly flagged the 404).
- [ ] Icon: `com.docker.desktop.extension.icon` currently points at the only
      verified hosted asset (`https://decionis.com/favicon.ico`). Host the
      512×512 `extension/decionis-logo.png` at a public decionis.com URL and
      swap the label.
- [ ] Screenshots captured from the real extension inside Docker Desktop
      (feed + dossier inspector), hosted, label added.
- [ ] Multi-arch image pushed to Docker Hub with provenance/SBOM (extend the
      publish workflow when the Hub repo exists).
- [ ] Re-run `docker extension validate` — fully green — on submission day.
- [ ] Submit per Docker's current extension submission process (re-read their
      contribution guide on submission day; record the PR below).

## Record of submissions

| Date | PR | Status | Notes |
| ---- | -- | ------ | ----- |
