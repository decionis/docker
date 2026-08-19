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
-->

# Decionis — execution authority for AI agents, inside Docker Desktop

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

- **Live decision evaluations** — outcomes `APPROVE / ESCALATE / REJECT /
  REVIEW` with the policy version, execution action, and reason behind each
  one, plus summary counts.
- **Signed Decision Dossiers, verified offline** — the extension fetches a
  decision's dossier and verifies its Ed25519 proof bundle against the
  published JWKS locally; it also reports whether the dossier carries
  everything needed to independently reproduce the decision.
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
docker extension install decionis/desktop-extension:0.1.4
```

Then open the **Decionis** tab and press **Continue in browser** — sign in
(your workspace is created on first sign-in) and Docker Desktop connects
itself. A pasted single-use **enrollment token** and manual org ID + API
key remain available under Settings. Every credential is minted
server-side and held by the extension backend only.

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
