<!--
  Docker Hub overview for decionis/authority — this committed file is the
  source of truth (rules/discovery.rules.md Rule 2.1); paste to Hub on every
  release, never edit only in the Hub UI.
-->

# Decionis authority gate — ask before the action, not after

`decionis/authority` is an execution gate for AI agents and automation.
Something about to take a consequential action describes it; Decionis
returns a verdict; the gate enforces it.

It does not inspect traffic, intercept packets, or mount the Docker socket.
It only ever knows about actions it was explicitly told about — and it
evaluates no policy itself: the verdict comes from your Decionis
organization.

## Fail closed, by construction

A decision starts denied, and only an explicit `APPROVE` from the control
plane replaces it. An unreachable control plane, a timeout, a refused
evaluation, a malformed response, or an unknown outcome all leave the
denial standing. **A timeout is never an approval.**

The HTTP status carries the same answer as the body — `200` only when the
action is authorized, `403` for every refusal whatever the cause — so a
caller that reads nothing but the status code still fails closed.

## Run it

```bash
docker run -d --name decionis-authority -p 127.0.0.1:8080:8080 \
  -e DECIONIS_ORG_ID=<your-org-uuid> \
  -e DECIONIS_API_KEY=<scoped-org-key> \
  decionis/authority:0.1.0
```

Credentials come from the environment only — never command-line flags,
which are readable from the host's process list. Bind it to loopback or an
internal network: the gate answers whoever can reach it.

## Two surfaces

**`POST /v1/gate`** — for SDKs and middleware. Send an action descriptor,
get a decision:

```json
{"decision_type": "refund", "amount": 4200, "channel": "support-console"}
```

A refusal carries the outcome, the reason, and the evaluation and dossier
ids, so it can be traced to a signed record rather than guessed at.

**`POST /v1/mcp/before-tool-call`** — a `before` interceptor for Docker's
MCP Gateway, so tool calls are governed without changing the agent:

```bash
docker mcp gateway run \
  --interceptor 'before:http:http://127.0.0.1:8080/v1/mcp/before-tool-call'
```

An approved call runs normally. A refused call does not run, and the agent
reads the outcome and reason in place of the tool's output.

Argument **values** are not sent to the control plane by default — they are
the agent's payload and can carry anything. The tool name and argument
*names* are always sent, which describes the shape of an action without its
contents; `-forward-arguments` opts in to the values.

**`GET /healthz`** — liveness. Authorizes nothing.

## What it is not

No policy evaluation happens here. No packet-level interception. No Docker
socket. The image is digest-pinned, multi-arch, runs as a non-root user
(65532), and ships with provenance and SBOM attestations plus a keyless
cosign signature.

## Related

- [`decionis/desktop-extension`](https://hub.docker.com/r/decionis/desktop-extension)
  — governance inside Docker Desktop.
- [`decionis/mcp`](https://hub.docker.com/r/decionis/mcp) — the local MCP
  evaluator: zero network, zero credentials, nothing recorded.

## Support & license

- Documentation: https://decionis.com/docs/protocol-mcp
- Source & issues: https://github.com/decionis/docker
- License: Apache-2.0
