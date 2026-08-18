# Discovery & Growth Operational Rules — decionis/docker

These rules make everything this repository ships discoverable and callable by
search engines, answer engines, marketplaces, and AI agent frameworks —
honestly. They are release requirements alongside `coding.rules.md` and
`security.rules.md`. They are the `decionis/docker` localization of the rules
shared across the Decionis and Presence repositories: §1 and §§5–6 carry over
nearly verbatim; the surfaces in §§2–4 are this repo's own (container images,
the Docker Desktop extension listing, the Dev Container Feature) rather than
the product websites'; and §7 maps the machinery to this repo's layout.

The founding incident to remember: a stale claim in machine-readable copy is
worse than no claim, because agents act on it. Every rule below exists to make
drift either impossible (derived surfaces), test-caught (drift gates), or
banned (verify-then-claim).

## 1. Verify-then-claim (binding on every surface)

- Rule 1.1: **No registry, marketplace, or catalog URL enters any copy,
  overview, or install snippet before its page resolves.** Verify each URL
  live (registry API or fetch) on the day of the change and record the sweep
  date in the source comment. Publishing later relaxes the list, never the
  rule. A 404 install command, dead Docker Hub link, or unresolvable image
  reference is a fabricated entity claim.
- Rule 1.2: **Capability claims are scoped to shipped behavior, in the owning
  document's vocabulary.** The evaluator's claims belong to `@decionis/mcp`
  and the protocol service — reuse their frozen phrasing verbatim (e.g. "zero
  network, zero credentials, nothing recorded"; the verdict vocabulary exactly
  as the protocol publishes it) and never paraphrase a capability upward. This
  repo does not coin its own verdict or capability vocabulary; surfaces render
  what the API actually returns, with at most the officially documented
  display mapping. Grep for the codebase's own token before declaring
  something absent.
- Rule 1.3: **No numeric latency or accuracy figures** in public copy unless a
  governed measurement exists with a test pinning the claim to it. A repo-wide
  guard test banning unsupported figures is the mechanism; extend it, don't
  argue with it.
- Rule 1.4: **Every hand-written inventory gets a drift gate**: a test that
  derives the truth from the source (the pinned `@decionis/mcp` version's tool
  registrations, extension `metadata.json`, image labels, the feature
  manifest) and set-compares it against the inventory. Editing one side
  without the other must fail CI.
- Rule 1.5: **Absolute links only** in anything rendered off-repo (Docker Hub
  overviews, extension marketplace description, feature README, catalog
  entries). Repo-relative links die on hub.docker.com, and private-repo blob
  links die everywhere. Either link a public product URL or de-link to an
  inline reference.
- Rule 1.6: Discovery surfaces are **inert metadata** (security rule 0.5):
  they describe; they never mint, proxy, or expose per-record data, and they
  take no free-text input into rendered output.
- Rule 1.7: **One owner per listing.** The npm package `@decionis/mcp`, the
  official MCP registry entry (`com.decionis/mcp`), `smithery.yaml`, and
  `glama.json` are owned by `decionis/decionis` (`apps/mcp`). This repo never
  duplicates or re-publishes them; it owns only the Docker-native surfaces in
  §2 and links to the upstream listings.

## 2. Machine-readable surfaces this repo ships

- Rule 2.1: **Docker Hub image overviews** for every published image (the MCP
  runtime image, the extension image, the authority proxy). Each overview
  carries a one-line what-it-is in the product's frozen vocabulary, the exact
  `docker run` / MCP Toolkit install path, required env vars and where secrets
  belong, the fail-closed semantics table where applicable, and absolute links
  to decionis.com docs. The overview is committed in-repo and pushed by CI —
  never edited only in the Hub UI (that is an untracked inventory).
- Rule 2.2: **Extension metadata** (`extension/metadata.json`, image labels,
  screenshots, changelog) lists exactly what the extension does and every
  privilege it uses (backend service, host binaries, socket mounts — each
  justified in `SECURITY.md`). `docker extension validate` passes in CI. No
  capability appears in the listing before it is shipped and demonstrable.
- Rule 2.3: **MCP catalog entry** (Docker MCP Toolkit/Catalog via the
  `docker/mcp-registry` process): the entry names only the tools the pinned
  upstream version registers, with the true auth posture (the local evaluator
  requires no credentials; connected surfaces state their key requirement
  exactly). Re-submit whenever an image version bump changes tools or claims.
- Rule 2.4: **Dev Container Feature manifest**
  (`devcontainer-feature.json`) published as an OCI artifact with documented
  options; the feature README states exactly what gets installed and wired
  (CLI, agent hooks, `.mcp.json`) and what does not.
- Rule 2.5: **Repository README** is the canonical narrative for the Docker
  integration surface. Governance files it references (`LICENSE`,
  `SECURITY.md`, `CONTRIBUTING.md`) must exist — a referenced-but-missing file
  is a Rule 1.1 violation in-repo.
- Rule 2.6: **Vocabulary pinning:** every surface renders verdicts in the
  vocabulary the layer it fronts actually returns. UI and CLI verdict labels
  derive from one mapping module with a drift gate against the protocol schema
  enums; marketing narrative vocabulary never leaks into rendered decision
  data.

## 3. Freshness engines

Web indexing machinery (IndexNow, Search Console, llms.txt, sitemaps) lives
with the product websites in the Decionis and Presence repos — this repo has
no website and must not grow one silently.

- Rule 3.1: This repo's freshness signals are image tags, extension version
  bumps, feature versions, and catalog re-submissions. Every release
  re-publishes the affected overviews and manifests from the committed
  sources.
- Rule 3.2: Publication steps run **after CI health checks pass**, are
  env-gated (silent no-op until credentials exist), and are `continue-on-error`
  — a registry hiccup never fails the build that produced the artifact.
- Rule 3.3: Tags are immutable evidence: a published version tag is never
  rebuilt in place. `latest` may move; version tags may not.

## 4. Marketplace & catalog distribution

- Rule 4.1: **Image-first ordering.** The image is published and its overview
  live on Docker Hub before any catalog or marketplace submission references
  it — reviewers resolve the reference on their side, exactly as npm-first
  ordering works for the MCP registry upstream.
- Rule 4.2: The MCP catalog submission pins the upstream `@decionis/mcp`
  version the image wraps; bumping the wrapped version regenerates the tool
  inventory and re-runs the drift gate before re-submission.
- Rule 4.3: Extension marketplace submission follows the same ordering: image
  pushed and `docker extension validate` green before the submission PR;
  screenshots and copy come from the committed sources.
- Rule 4.4: A listing names only registered tools and capabilities and states
  the exact auth posture. A tool's description must state its fail-closed
  semantics if it has them.
- Rule 4.5: Submission runbooks live in `docs/marketplace/` in this repo,
  recording account ownership, orderings, and every user-owned manual step.

## 5. Published overviews & READMEs (the developer-facing experience)

- Rule 5.1: Every published surface (Hub overview, extension description,
  feature README) carries: a one-line what-it-is in the product's frozen
  vocabulary, **Install/Run**, a copy-pasteable minimal end-to-end example
  (including required env vars and where secrets belong), the
  fail-closed/outcome semantics table where applicable, and a Support +
  License footer.
- Rule 5.2: Rules 1.5 (absolute links) and 1.2 (scoped claims) apply in full.
  Do not reference repo files as if they ship in an image unless they do.
- Rule 5.3: Registries render what was last pushed — doc improvements reach
  users on the next push or version bump; plan releases accordingly.

## 6. Shipping checklist (copy into the feature/component PR)

- [ ] Image builds reproducibly; overview committed and pushed with it
      (Rules 2.1, 3.3)
- [ ] Drift gates extended: tool inventory vs pinned upstream, metadata vs
      shipped behavior, verdict-label mapping vs protocol enums
      (Rules 1.4, 2.6)
- [ ] All marketplace/catalog URLs live-verified on change day (Rule 1.1)
- [ ] Extension `docker extension validate` green; every privilege justified
      (Rule 2.2)
- [ ] Image-first ordering observed for any submission (Rule 4.1)
- [ ] Overviews/READMEs meet Rule 5; no relative links
- [ ] No unsupported latency/accuracy figures (Rule 1.3); capability claims in
      the owning doc's vocabulary (Rule 1.2)
- [ ] Runbook updated (`docs/marketplace/*`) with any new secrets, orderings,
      or user-owned submissions

## 7. Where the machinery lives (as components land)

| Concern                 | decionis/docker location                                                          |
| ----------------------- | --------------------------------------------------------------------------------- |
| MCP runtime image       | `mcp-server/Dockerfile` + `mcp-server/overview.md`                                 |
| Image/tool drift gates  | `mcp-server/test/` (tool inventory vs pinned upstream version)                     |
| Extension metadata      | `extension/metadata.json` + extension image labels                                 |
| Verdict-label mapping   | `extension/ui/src/protocol/VerdictLabels.ts` + its drift test                      |
| Dev Container Feature   | `features/govern/devcontainer-feature.json`                                        |
| Hub overviews           | committed beside each Dockerfile, pushed by CI                                     |
| Submission runbooks     | `docs/marketplace/`                                                                |
| Upstream-owned listings | `decionis/decionis` → `apps/mcp/{server.json,smithery.yaml,glama.json}` (Rule 1.7) |

Paths named for components not yet landed are their planned homes; the
component PR that lands one must land its drift gate in the same change.
