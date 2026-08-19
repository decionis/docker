# Docker MCP Gateway interceptors — verified contract

Phase 3 asks whether Docker's MCP Gateway is a viable insertion point for
Decionis enforcement: **ride Docker's gateway, never rebuild it**. This is
the verification the plan requires before any claim is made about it.

**Verdict: viable for enforcement, with one adapter.** A `before`
interceptor can refuse a tool call, not merely observe it. But the
allow/deny signal is the opposite shape to the authority gate's current
HTTP surface, so wiring the gate in unchanged would block every call —
including approved ones. See "The trap" below.

## What was verified, and how

Two independent sources, not documentation prose:

1. `docker mcp` **v0.28.0** installed locally — flags and the accepted
   values read out of the CLI's own parser errors.
2. `docker/mcp-gateway` `pkg/interceptors/interceptors.go` at commit
   `d15eb210f1a3ba9f5123d19a8217f0f5028fde62` (last touched 2026-06-18), read line by line.

Both re-check before relying on this: the API is young and unversioned.

## The contract

Registered per gateway run:

```
docker mcp gateway run --interceptor 'before:http:http://127.0.0.1:PORT/path'
```

- `when` — `before` or `after` (CLI parser, verified by rejection).
- `type` — `exec`, `docker`, or `http` (same). The published help text
  names only `exec`; `http` and `docker` exist but are undocumented there.
- **Only `tools/call` is intercepted.** Every other MCP method passes
  through untouched — `if method != "tools/call" { return next(...) }`.

**Payload.** The gateway marshals the MCP request and sends it as the POST
body: `json.Marshal(req)` → `http.NewRequestWithContext(ctx,
http.MethodPost, i.Argument, bytes.NewBuffer(message))`. For `exec` and
`docker` the same JSON arrives on stdin.

**Allow / deny.** Decided entirely by the response **body**:

| Interceptor returns | Gateway does |
| ------------------- | ------------ |
| empty body | calls the tool (allow) |
| non-empty body parsing as `mcp.CallToolResult` | returns that result **instead of** calling the tool (deny/replace) |
| non-empty body that fails to parse | fails the call with an error |
| transport error | fails the call with an error |

**Fail closed.** Every error path returns `nil, err` — an unreachable
interceptor, a malformed response, or a cancelled context blocks the tool
call rather than letting it through. That matches this repository's own
doctrine (a timeout is never an approval), so the gateway does not weaken
it.

## The trap

`runHTTP` **ignores the HTTP status code entirely** — it reads the body and
returns it. Nothing inspects `response.StatusCode`.

The authority gate answers `200 {"allowed":true,...}` and
`403 {"allowed":false,...}` — a JSON body either way, by design, so a
caller reading only the status still fails closed. Pointed at the gateway
unchanged, **every call would be blocked**, approvals included, because
every response body is non-empty.

An adapter endpoint is therefore required, and it must invert the habit:

- authorized → **empty body** (any status)
- not authorized → an `mcp.CallToolResult` describing the refusal
  (`isError` plus the outcome and reason), which the caller sees in place
  of the tool's output

## Verified end to end (2026-08-19)

The adapter (`POST /v1/mcp/before-tool-call`) was exercised against a real
`docker mcp gateway run --servers time` with
`--interceptor before:http:http://127.0.0.1:9098/v1/mcp/before-tool-call`,
backed by a fake control plane:

- **Approved** — the plane returned APPROVE, the adapter wrote an empty
  body, and the tool ran: `{"timezone":"UTC","datetime":"2026-08-19T…"}`.
- **Refused** — the plane returned REJECT and the tool did **not** run. The
  agent received instead:
  `Decionis did not authorize get_current_time. Outcome: REJECT. clock
  reads are restricted by policy Decision dossier: dos-gw.`

So enforcement genuinely rides the gateway; a refusal replaces the tool's
output rather than merely being logged.

What the adapter sent upstream in that run, showing the withheld values:

```json
{"decision_type":"agent-action","channel":"get_current_time",
 "context":{"source":"docker-mcp-gateway","tool_name":"get_current_time",
            "argument_names":["timezone"]}}
```

## Other constraints worth designing around

- **No authentication.** No headers are set on the request at all, so the
  endpoint must not be reachable beyond loopback; anything that can reach
  it can drive it.
- **No client timeout.** `&http.Client{Transport: desktop.ProxyTransport()}`
  sets no `Timeout`; only the passed context bounds the call. A slow gate
  slows every governed tool call, so the adapter must bound itself.
- **Per-run, not per-tool.** Interceptors are a gateway run flag, so
  enabling Decionis means configuring the gateway, not the individual
  servers.
