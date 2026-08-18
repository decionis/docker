<!--
  Docker Hub overview for decionis/mcp — this committed file is the source of
  truth (rules/discovery.rules.md Rule 2.1); paste/push to Hub on every
  release, never edit only in the Hub UI.

  Link sweep 2026-08-18:
    - https://decionis.com/docs/protocol-mcp -> HTTP 200
    - https://www.npmjs.com/package/@decionis/mcp -> verified via npm registry
      API (`npm view @decionis/mcp`; npmjs.com returns 403 to non-browser
      fetches)
    - https://github.com/decionis/docker -> PENDING: repo not public yet.
      MUST resolve before this overview is pushed to Hub (Rule 1.1) — see
      docs/marketplace/docker-hub.md publish checklist.
-->

# decionis/mcp — Decionis execution authority, local MCP evaluator

A stdio [Model Context Protocol](https://modelcontextprotocol.io) server that
lets AI coding agents (Claude Code, Cursor, Codex, OpenHands, …) read a
repository's `DECIONIS_POLICY.md` and evaluate candidate actions **before**
they commit, deploy, or migrate anything.

This image wraps the published npm package
[`@decionis/mcp`](https://www.npmjs.com/package/@decionis/mcp) at an exact
pinned version — packaging only, no fork. The evaluator boots the real
Decionis protocol service in-process: the verdict an agent sees locally is the
verdict the platform would produce — with **zero network, zero database, zero
credentials, and nothing recorded**.

## Run

Mount your policy read-only; the evaluator needs no network and writes no
state:

```bash
docker run -i --rm \
  --network none --read-only \
  -v "$PWD/DECIONIS_POLICY.md:/work/DECIONIS_POLICY.md:ro" \
  decionis/mcp
```

## Wire it up (any stdio MCP client)

Claude Code (`.mcp.json` at the repo root), Cursor, and other stdio MCP
clients use the same shape — replace the absolute path with your repo's:

```json
{
  "mcpServers": {
    "decionis": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm", "--network", "none", "--read-only",
        "-v", "/absolute/path/to/your/repo/DECIONIS_POLICY.md:/work/DECIONIS_POLICY.md:ro",
        "decionis/mcp"
      ]
    }
  }
}
```

## Tools

| Tool                    | Purpose                                                                                                                |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `decionis_read_policy`  | Path, sha256, and compiled rules (or compile errors) of the repo's `DECIONIS_POLICY.md`.                               |
| `decionis_evaluate`     | Evaluate a candidate action payload against the policy through the real evaluator; returns the verdict + matched rule. |
| `decionis_verdict_help` | The verdict vocabulary, rules-block grammar, and how an agent should behave on each verdict.                           |

## Verdicts and outcomes

From the server's own `decionis_verdict_help`:

> **Verdicts** (a rule's `action`): `allow` (execute), `block` (never
> execute), `restrain` (hold for human review), `escalate` (require an
> authorized approver). The protocol reports outcomes as APPROVE / REJECT /
> REVIEW / ESCALATE; this server also returns the normalized verdict (allow /
> block / review / escalate). When no rule matches, the platform holds the
> decision for review (outcome REVIEW).

Agent behavior on each verdict: allow → proceed; block → do not proceed;
review/escalate → stop and involve a human.

## Configuration

| Variable               | Required | Secret | Purpose                                                                                     |
| ---------------------- | -------- | ------ | ------------------------------------------------------------------------------------------- |
| `DECIONIS_POLICY_PATH` | no       | no     | Policy file path. The image presets `/work/DECIONIS_POLICY.md` — mount your policy there.   |

No credentials are required.

## Image

- Wraps `@decionis/mcp` at an exact, lockfile-pinned version; the image
  version tracks the wrapped package version.
- Runs as a non-root user on a digest-pinned base; release images publish
  SBOM and provenance attestations and are signed.

## Support & license

- Documentation: https://decionis.com/docs/protocol-mcp
- Issues: https://github.com/decionis/docker/issues
- License: Apache-2.0
