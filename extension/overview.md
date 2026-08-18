<!--
  Docker Hub overview for decionis/desktop-extension — this committed file is
  the source of truth (rules/discovery.rules.md Rule 2.1); paste/push to Hub
  on every release, never edit only in the Hub UI.

  Link sweep 2026-08-18:
    - https://decionis.com/docs/protocol-mcp -> HTTP 200
    - https://github.com/decionis/docker -> HTTP 200
    - https://hub.docker.com/r/decionis/mcp -> verified via Hub API
      (v2/repositories 200)
-->

# Decionis — execution authority for AI agents, inside Docker Desktop

Decionis evaluates consequential agent actions against deterministic,
versioned policy **before** execution. This Docker Desktop extension connects
to your Decionis org and makes that authority visible where you work:

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

## Install

Until the extension is listed in the Docker Desktop Extensions Marketplace,
install it directly (Docker Desktop → Settings → Extensions → allow
non-marketplace extensions):

```bash
docker extension install decionis/desktop-extension:0.1.0
```

Then open the **Decionis** tab, choose **Settings**, and connect with your
organization ID and org API key. No credentials are needed until you connect.

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
