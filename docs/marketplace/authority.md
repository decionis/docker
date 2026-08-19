# Runbook — Decionis authority gate (`decionis/authority`)

The action-aware execution gate: an SDK, middleware, or Docker's MCP
Gateway describes an action, Decionis returns a verdict, the gate enforces
it. Owner-facing per `rules/discovery.rules.md` Rule 4.5.

## Release

```bash
git tag authority-v0.1.0 && git push origin authority-v0.1.0
```

`.github/workflows/authority-publish.yml` then builds, **smoke-tests**,
scans, and — with Docker Hub credentials present — pushes
`decionis/authority:X.Y.Z` and `:latest` multi-arch with provenance + SBOM
attestations and a keyless cosign signature. Without credentials the
publish steps no-op silently (Rule 3.2); verification still runs.

The smoke step is deliberately about refusal, because that is the property
that matters: it starts the image pointed at an unreachable control plane
and asserts

- `POST /v1/gate` answers **403** with `"allowed":false` — an unreachable
  plane is never an approval, and
- `POST /v1/mcp/before-tool-call` answers with a **non-empty** body marked
  `isError`. Empty is how Docker's MCP Gateway is told to run the tool, so
  a gate that fell back to silence would permit everything it could not
  evaluate.

The vulnerability scan (CRITICAL/HIGH, unfixed ignored) is the blocking
release gate.

## Running it

```bash
docker run -d --name decionis-authority -p 127.0.0.1:8080:8080 \
  -e DECIONIS_ORG_ID=<org-uuid> \
  -e DECIONIS_API_KEY=<scoped-org-key> \
  decionis/authority:0.1.0
```

Credentials come from the environment only — never flags, which are
readable from the host's process list. Bind to loopback or an internal
network: the gate answers whoever can reach it.

- `POST /v1/gate` — the SDK/middleware surface. Send an action descriptor,
  get a decision. 200 only when authorized; 403 for every refusal whatever
  the cause, so a caller reading only the status still fails closed.
- `POST /v1/mcp/before-tool-call` — Docker MCP Gateway `before:http:`
  interceptor. Different dialect on purpose; see
  `docs/mcp-gateway-interceptor.md`.
- `GET /healthz` — liveness. Authorizes nothing.

## Wiring it into Docker's MCP Gateway

```bash
docker mcp gateway run \
  --interceptor 'before:http:http://127.0.0.1:8080/v1/mcp/before-tool-call'
```

Verified end to end on `docker mcp` v0.28.0: an approved call runs the
tool; a refused call does not run, and the agent receives the outcome,
reason, and dossier id instead of the tool's output.

Two flags shape what the control plane is told about intercepted calls:

- `-decision-type` (default `agent-action`) — how a tool call is described.
- `-forward-arguments` (default **off**) — send argument *values*. Off by
  default because arguments are the agent's payload and can carry anything;
  the tool name and argument *names* are always sent, which describes the
  shape of the action without its contents.

## Docker Hub presence

Repository `decionis/authority` (public, created by the owner 2026-08-19).
Short description and overview are pasted from `cmd/authority-proxy/overview.md`
— that committed file is the source of truth (Rule 2.1); never edit only in
the Hub UI. Note the short description is capped at **100 characters**.

A `decionis-release-announce` webhook points at the release receiver, so
pushes are recorded. Authority releases are deliberately **recorded but not
emailed**: the gate is infrastructure an operator deploys, while the email
audience is people whose Docker Desktop workspace is connected — a
different group.

## Release log

| Date | Tag | Digest | Notes |
| ---- | --- | ------ | ----- |
| 2026-08-19 | `authority-v0.1.0` → `decionis/authority:0.1.0` + `:latest` | `sha256:5eea7b5f33d2ee836634e5d3e226d3b7c49875911fbfca6cd7882e1d043e3fb6` | First publish. Multi-arch (linux/amd64 + linux/arm64), provenance + SBOM, cosign keyless. Verified against the PUBLISHED image, not the local build: reports `version 0.1.0`, runs as uid 65532, `/v1/gate` answers 403 with `evaluation_unavailable` against an unreachable plane, and `/v1/mcp/before-tool-call` answers a non-empty `isError` result — the shape that stops Docker's MCP Gateway running the tool. |
