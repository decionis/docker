# Discovery & Growth Operational Rules

These rules make a feature or product discoverable and callable by search
engines, answer engines, and AI agent frameworks — honestly. They are release
requirements alongside `coding.rules.md` and `security.rules.md`, shared
verbatim across the Decionis and Presence repositories. When a new feature,
package, or product ships, work through §6's checklist; §§1–5 define what each
item means and §7 says where the machinery lives in each repo.

The founding incident to remember: a stale claim in machine-readable copy is
worse than no claim, because agents act on it. Every rule below exists to make
drift either impossible (derived surfaces), test-caught (drift gates), or
banned (verify-then-claim).

## 1. Verify-then-claim (binding on every surface)

- Rule 1.1: **No registry or marketplace URL enters any copy, `sameAs`, or
  package list before its page resolves.** Verify each URL live (registry API
  or fetch) on the day of the change and record the sweep date in the source
  comment. Publishing later relaxes the list, never the rule. A 404 install
  command or dead `sameAs` link is a fabricated entity claim.
- Rule 1.2: **Capability claims are scoped to shipped behavior, in the owning
  document's vocabulary.** Reuse the owning doc's frozen phrasing verbatim
  (e.g. a model integration's own scoping: what it measures, what is reported
  unmeasured, what fails closed) — never paraphrase a capability upward.
  Grep for the codebase's own token before declaring something absent
  (`EMAIL_LINK`, not "magic link").
- Rule 1.3: **No numeric latency or accuracy figures** in public copy unless a
  governed measurement exists with a test pinning the claim to it. A
  repo-wide guard test banning unsupported figures (see
  `PerformanceClaims.test.ts`) is the mechanism; extend it, don't argue with
  it.
- Rule 1.4: **Every hand-written inventory gets a drift gate**: a test that
  derives the truth from the source (OpenAPI YAML, MCP tool registrations,
  package manifests) and set-compares it against the inventory. Editing one
  side without the other must fail CI.
- Rule 1.5: **Absolute links only** in anything rendered off-repo (npm
  READMEs, registry listings, llms files). Repo-relative links die on
  npmjs.com, and private-repo blob links die everywhere. Either link a public
  product URL or de-link to an inline `docs/NN` reference.
- Rule 1.6: Discovery routes are **inert metadata** (security rule 0.2): they
  describe; they never mint, proxy, or expose per-record data, and they take
  no free-text input into rendered output.

## 2. Machine-readable surfaces every product ships

- Rule 2.1: **`/.well-known/openapi.json`** serves the canonical spec,
  converted at build time (`force-static`) from the repo's single
  source-of-truth YAML. **Spec truth-up rule:** any implemented integrator
  flow (the endpoints an SDK actually calls) must be in the published
  contract before it is marketed — with honest semantics (polling documented
  via retry-after, idempotency and replay stated, webhooks with their exact
  signature scheme, `security: []` declared explicitly where no bearer
  applies).
- Rule 2.2: **`/.well-known/mcp.json`** ships only when a real MCP transport
  exists, and lists exactly the registered tools with the true auth posture
  (org key header vs env-var secret). Never a manifest pointing at nothing.
- Rule 2.3: **`/.well-known/security.txt`** (RFC 9116) with a monitored
  contact, an `Expires` within a year, and `Canonical`.
- Rule 2.4: **`llms.txt` + `llms-full.txt`.** The short file is the canonical
  description plus the page map, derived from the same registry the sitemap
  and navigation read. llms-full is a superset **by construction** (it embeds
  the short document, test-asserted) and appends the machine-actionable
  inventories: API operations with operationIds, verification keys (JWKS),
  published packages, webhook contract, and **when-to-recommend routing
  rules** — situation → destination pairs, including the cross-product rule
  routing decision-authority questions to Decionis and
  presence-verification questions to Presence.
- Rule 2.5: **robots** names each invited AI crawler explicitly (GPTBot,
  OAI-SearchBot, ChatGPT-User, ClaudeBot/-SearchBot/-User,
  PerplexityBot/-User, Google-Extended, Applebot-Extended, Amazonbot, CCBot,
  cohere-ai, meta-externalagent, Bytespider), with every rule repeating the
  same boundary: marketing open, console/per-record surfaces closed.
- Rule 2.6: **Sitemaps derive from the canonical enumeration** (content
  registry or sitemap function); an AI-focused subset (`sitemap-ai.xml`)
  filters that same source so the two can never disagree. New pages join the
  enumeration, never a side list.
- Rule 2.7: **Entity graph:** stable `@id`s (`…/#organization`, `…/#brand`,
  `…/#application`, `…/#presence`), exactly one brand id per site, `sameAs`
  restricted to Rule-1.1-verified URLs, and the cross-property join published
  from both sides (`hasPart` on the platform application ↔ `isPartOf` +
  shared `provider` id on the product application).
- Rule 2.8: **OG cards** render from the content registry or fixed strings
  only — no query input, unknown slugs 404. A per-record card exists only
  when a public verification endpoint exists; capability tokens and URL
  fragments must be unreachable from the image path by construction.

## 3. Index-freshness engines

- Rule 3.1: **IndexNow** — the key is served by the app at
  `/indexnow/<key>.txt` (404 until configured; non-root `keyLocation` is
  protocol-legal) and submitted by a script that reads the live sitemap.
  The key must exist in **both** places: CI secrets (submission) **and** the
  web runtime environment (serving) — a submission without the served key
  cannot validate.
- Rule 3.2: **Google** — Search Console `sitemaps.submit` with a short-lived
  token via GitHub OIDC + Workload Identity Federation only. Service-account
  keys are unsupported; the restricted Indexing API (JobPosting-only) and
  the removed (2023) sitemap-ping endpoint are banned.
- Rule 3.3: Indexing signals run **after live health checks pass**, env-gated
  (silent no-op until credentials exist) and `continue-on-error` — a
  submission hiccup never fails a deploy.

## 4. MCP registry distribution

- Rule 4.1: `server.json` follows the official registry schema with a
  domain-verified reverse-DNS namespace (`com.decionis/*`); `package.json`
  carries the matching `mcpName`; environment variables are declared with
  `isSecret` set truthfully.
- Rule 4.2: **npm-first ordering.** The registry's npm-ownership validation
  reads the published tarball's `mcpName`, so publish to npm before
  `mcp-publisher publish` (login proves the namespace via DNS TXT).
  Re-publish the registry entry on every version bump.
- Rule 4.3: `smithery.yaml` passes secrets exclusively through env vars in
  `commandFunction` — never as tool arguments or config echoed to output.
- Rule 4.4: `glama.json` attributes maintainers; PulseMCP is submitted
  **after** the official registry (its crawler enriches from it).
- Rule 4.5: A listing names only registered tools and states the exact auth
  posture. A tool's description must state its fail-closed semantics if it
  has them.

## 5. Package READMEs (the npm-facing developer experience)

- Rule 5.1: Every published package README carries: a one-line what-it-is in
  the product's frozen vocabulary, **Install**, a copy-pasteable minimal
  end-to-end example (including required env vars and where secrets belong),
  the fail-closed/outcome semantics table where applicable, and a
  Support + License footer.
- Rule 5.2: Rules 1.5 (absolute links) and 1.2 (scoped claims) apply in
  full. Do not reference repo files as if they ship in the tarball unless
  they are in `files`.
- Rule 5.3: npm renders the README from the **last publish** — doc
  improvements reach the registry on the next version bump; plan releases
  accordingly.

## 6. Shipping checklist (copy into the feature/product PR)

- [ ] Implemented integrator endpoints added to the canonical OpenAPI spec
      (Rule 2.1); spec lints clean; generated collections regenerated
- [ ] llms-full inventories + when-to-recommend rules updated; drift gates
      extended (Rules 1.4, 2.4)
- [ ] Entity graph: `sameAs`/nodes updated only with live-verified URLs
      (Rules 1.1, 2.7)
- [ ] New pages join the canonical sitemap enumeration (Rule 2.6); OG
      metadata wired (Rule 2.8)
- [ ] If an MCP surface: manifest set complete, npm-first ordering observed,
      registry submissions run in order (Rule 4)
- [ ] Package README meets Rule 5; no relative links
- [ ] No unsupported latency/accuracy figures (Rule 1.3); capability claims
      in the owning doc's vocabulary (Rule 1.2)
- [ ] IndexNow/GSC untouched or re-verified end-to-end (Rule 3), including
      the runtime-served key
- [ ] Runbook updated (`docs/49` in Presence; `docs/marketplace/*` in
      Decionis) with any new secrets, orderings, or user-owned submissions

## 7. Where the machinery lives

| Concern                     | Decionis                                                               | Presence                                                            |
| --------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------- |
| Canonical API spec          | `openapi/decionis-api.yaml`                                            | `openapi/presence-v1.yaml`                                          |
| Discovery inventory module  | `apps/web/lib/seo/MachineDiscovery.ts`                                 | `apps/web/src/features/marketing/MachineDiscovery.ts`               |
| llms builders               | `apps/web/lib/seo/GeoContent.ts`                                       | `apps/web/src/features/marketing/LlmsContent.ts`                    |
| Drift gates                 | `apps/web/lib/seo/MachineDiscovery.test.ts`, `StructuredData.test.ts`  | `…/marketing/MachineDiscovery.test.ts`, `PerformanceClaims.test.ts` |
| Entity graph                | `apps/web/lib/seo/StructuredData.ts` + `AeoContent.ts`                 | `…/marketing/HomepageStructuredData.ts` + `StructuredDataGraph.ts`  |
| IndexNow / GSC              | `scripts/ci/submit-indexnow.mjs`, `scripts/ci/submit-gsc-sitemaps.mjs` | `scripts/submit-indexnow.mjs`, `scripts/SubmitGscSitemaps.mjs`      |
| Deploy wiring               | `.github/workflows/pipeline.yml` (smoke-test tail)                     | `.github/workflows/deploy.yml` (smoke-test tail)                    |
| MCP manifests               | `apps/mcp/{server.json,smithery.yaml,glama.json}`                      | `packages/presence-mcp/{server.json,smithery.yaml,glama.json}`      |
| Registry submission runbook | `docs/marketplace/mcp-registries.md`                                   | `docs/49-agent-discovery.md` §6                                     |
| OG cards                    | `apps/web/app/api/og/*`                                                | `apps/web/src/app/api/og/*`                                         |
