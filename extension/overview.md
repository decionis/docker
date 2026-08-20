<!--
  Docker Hub overview for decionis/desktop-extension — this committed file is
  the source of truth (rules/discovery.rules.md Rule 2.1); paste/push to Hub
  on every release, never edit only in the Hub UI. The narrative section was
  authored by the owner on Hub (2026-08-18) and folded back in here.

  Link sweep 2026-08-18:
    - https://decionis.com/docs/protocol-mcp -> HTTP 200
    - https://github.com/decionis/docker -> HTTP 200
    - https://hub.docker.com/r/decionis/mcp -> verified via Hub API
      (v2/repositories 200)
  Link sweep 2026-08-20:
    - screenshot raw URL below (SHA-pinned, commit dce49dd6) -> HTTP 200
-->

# Decionis — execution authority for AI agents, inside Docker Desktop

![The Decionis console after ten live policy checks: three proposals APPROVED, three REJECTED, four ESCALATED to a person, with the review queue and governed-decision counters](https://raw.githubusercontent.com/decionis/docker/dce49dd6e12afa9e068e4709812f70466113c41c/docs/screenshots/desktop-extension-0.1.8-policy-pack.png)

Decionis adds an execution-authority layer to Docker-based AI and automation
workflows.

Before a consequential action proceeds, Decionis evaluates it against
deterministic policy and returns an explicit outcome such as PROCEED, HOLD,
or BLOCK.

Use Decionis with containerized AI agents, MCP servers, CI/CD workflows,
developer tools, and automated services that need clear execution boundaries.

When policy requires human authority, Decionis Presence can request verified
approval before execution continues.

Every governed decision can produce a signed Decision Dossier containing the
policy version, reason codes, evaluation context, evidence hash, and
cryptographic signature.

Typical use cases include:

- AI agent and MCP tool governance
- production deployment approval
- infrastructure changes
- database mutations
- package publishing
- CI/CD execution controls
- privileged developer operations
- autonomous workflow governance

Decionis does not execute downstream actions. It evaluates whether an action
is authorized to proceed; the connected agent, tool, or system remains
responsible for execution.

**Your agents have tools. Decionis provides the authority boundary.**

## What the extension shows

- **Investigable decision evaluations** — search by type, domain, reason,
  policy, rule, evaluation ID, or dossier ID; combine outcome, execution,
  policy, and domain/type filters; then open structured decision details.
  Summary cards include the protocol's policy-mismatch and near-miss counts
  and rates.
- **Approval-to-dossier navigation** — decisions awaiting human review link
  directly to their Decision Dossiers when a dossier ID is available. The
  approval itself remains in the established Decionis Presence flow.
- **A runnable Docker policy pack** — ten fixed action proposals (benign
  reads, destructive actions, high-value changes) evaluated live by the
  connected workspace's policy, with the exact returned outcome. The daemon
  owns every proposal; nothing the proposals describe is ever executed. Runs
  when enforcement is enabled, and each check uses one governed decision.
- **Signed Decision Dossiers, verified offline** — the extension fetches a
  decision's dossier and verifies its Ed25519 proof bundle against the
  published JWKS locally; it also reports whether the dossier carries
  everything needed to independently reproduce the decision. The complete
  dossier, verification result, and reproducibility assessment can be copied
  or downloaded locally as JSON.
- **Credential custody done right** — the org API key is held by the
  extension's backend only, never the UI, and the extension mounts no Docker
  Engine socket, no host paths, and ships no host binaries.
- **Knows when it's stale** — the extension checks its own public Docker Hub
  tags listing (anonymously) and shows a dismissible banner when a newer
  version is published.

## Install

Until the extension is listed in the Docker Desktop Extensions Marketplace,
install it directly (Docker Desktop → Settings → Extensions → allow
non-marketplace extensions):

```bash
docker extension install decionis/desktop-extension:0.1.8
```

Then open the **Decionis** tab. That is the whole setup: a workspace is
created for you on first run and the decision feed is live — no account,
no sign-in, nothing to paste. Already have a Decionis account? Open
Settings and sign in with your email and password; Decionis finds your
workspace and issues the extension its own credential, so there is no
workspace ID or API key to copy. Every credential is minted server-side
and held by the extension backend only.

**Shadow by default.** Decionis records decisions and gates nothing until
you tick **Enforce decisions**. A new workspace includes 25 free governed
(enforcing) decisions; shadow evaluations are unlimited.

## The local evaluator (no account required)

The zero-credential sibling of this extension is the
[`decionis/mcp`](https://hub.docker.com/r/decionis/mcp) image: the Decionis
local MCP evaluator that lets AI coding agents evaluate candidate actions
against a repository's `DECIONIS_POLICY.md` — zero network, zero credentials,
nothing recorded.

## Support & license

- Documentation: https://decionis.com/docs/protocol-mcp
- Source & issues: https://github.com/decionis/docker
- License: Apache-2.0
