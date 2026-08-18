# Runbook — Docker MCP Toolkit/Catalog submission for `decionis/mcp`

Goal: make Decionis discoverable inside Docker Desktop's MCP Toolkit so
Claude, Cursor, VS Code, and other clients can add it through their Docker
MCP profile.

## Ownership boundaries (discovery rule 1.7)

The npm package `@decionis/mcp`, the official MCP registry entry
(`com.decionis/mcp`), `smithery.yaml`, and `glama.json` are owned by
`decionis/decionis` (`apps/mcp`) and are **not** re-published from this repo.
This repo owns exactly one MCP listing: the Docker catalog entry for the
`decionis/mcp` image.

## Ordering (discovery rule 4.1)

Image-first: `decionis/mcp` must be published on Docker Hub with its overview
live before the catalog submission references it. See
`docs/marketplace/docker-hub.md` for that gate.

## Submission

Submissions go through a pull request to
`https://github.com/docker/mcp-registry`. **Re-read that repo's contribution
guide on submission day** (Rule 1.1 — do not trust this runbook's memory of
Docker's process); verify at that time:

- whether the Docker-built tier (Docker builds, signs, and maintains the
  image from this repo's Dockerfile, published under Docker's `mcp/`
  namespace) or the self-provided-image tier fits, and what each requires;
- the current entry schema (server metadata, tools inventory, env/secrets
  declarations).

The entry must state:

- tools exactly as the pinned `@decionis/mcp` version registers them
  (`decionis_read_policy`, `decionis_evaluate`, `decionis_verdict_help` at
  0.1.2) — the smoke test is the drift gate; re-submit whenever a version
  bump changes tools or claims (Rule 4.2);
- the true auth posture: the local evaluator requires **no credentials**
  (`isSecret` declarations truthful, none expected);
- capability claims in upstream's frozen vocabulary ("zero network, zero
  credentials, nothing recorded").

## Record of submissions

| Date | PR | Status | Notes |
| ---- | -- | ------ | ----- |
| 2026-08-18 | https://github.com/docker/mcp-registry/pull/4718 | Submitted, awaiting Docker review | `servers/decionis/server.yaml`: self-provided image `decionis/mcp`, `disableNetwork: true`, `policy_path` config → read-only mount at `/work/DECIONIS_POLICY.md`. `task validate` all green; `task build -- --tools --pull-community decionis` → 3 tools found. No test credentials needed (zero-credential evaluator). |
