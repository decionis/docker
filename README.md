# Decionis for Docker

<!-- Badge sweep 2026-08-18: every URL below verified HTTP 200 (rules/discovery.rules.md Rule 1.1),
     with two branch-lifecycle caveats that resolve at merge: the decionis.yml badge endpoint
     registers when the workflow reaches the default branch (this README publishes in the same
     merge), and coverage populates on the first master CI run after the codecov upload landed. -->

[![CI](https://github.com/decionis/docker/actions/workflows/decionis.yml/badge.svg)](https://github.com/decionis/docker/actions/workflows/decionis.yml)
[![License](https://img.shields.io/github/license/decionis/docker)](https://github.com/decionis/docker/blob/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/decionis/docker)](https://goreportcard.com/report/github.com/decionis/docker)
[![Coverage](https://codecov.io/gh/decionis/docker/graph/badge.svg)](https://codecov.io/gh/decionis/docker)
[![Go version](https://img.shields.io/github/go-mod/go-version/decionis/docker)](https://github.com/decionis/docker/blob/master/go.mod)
[![Release](https://img.shields.io/github/v/release/decionis/docker)](https://github.com/decionis/docker/releases)
[![decionis/mcp pulls](https://img.shields.io/docker/pulls/decionis/mcp?label=decionis%2Fmcp%20pulls&logo=docker)](https://hub.docker.com/r/decionis/mcp)
[![desktop-extension pulls](https://img.shields.io/docker/pulls/decionis/desktop-extension?label=desktop-extension%20pulls&logo=docker)](https://hub.docker.com/r/decionis/desktop-extension)

**Deterministic execution authority for AI agents and automated workflows running with Docker.**

Decionis adds an explicit authority boundary between an AI agent's intent and consequential execution.

When an agent attempts a sensitive action—such as deploying infrastructure, modifying production data, publishing a package, invoking a privileged MCP tool, or performing another governed operation—Decionis evaluates the proposed action against deterministic, versioned policy **before execution**.

The result is an explicit decision:

**PROCEED · HOLD · BLOCK**

When policy requires human authority, **Decionis Presence** can request cryptographically verifiable human approval before execution continues.

Every governed decision can produce a signed **Decision Dossier** containing the policy, evidence, reason codes, evaluation metadata, and cryptographic proof behind the decision.

---

## Why Decionis for Docker?

AI coding agents increasingly operate inside containers, development environments, CI/CD pipelines, and MCP-connected toolchains.

They can write code.

They can also:

```text
terraform apply
npm publish
git push --force
docker run --privileged
database.drop
github.create_release
```

The question is no longer only:

> **Can the agent execute this tool?**

It is:

> **Is the agent authorized to execute this action, under this policy, in this context, right now?**

Decionis provides that authority layer.

```text
AI Agent / Automation
        │
        │ proposes action
        ▼
┌──────────────────────┐
│       DECIONIS       │
│  Execution Authority │
│                      │
│ deterministic policy │
└──────────┬───────────┘
           │
    ┌──────┼──────┐
    │      │      │
 PROCEED  HOLD   BLOCK
    │      │
    │      ▼
    │   PRESENCE
    │   Human Authority
    │      │
    │   verified
    │      │
    └──────┴───────────► Execute
                         │
                         ▼
                  Decision Dossier
```

## What this repository provides

`decionis/docker` is the open integration surface for running and experiencing Decionis in Docker-based developer environments.

The project is designed to include:

### Decionis MCP Server

Run **Decionis Execution Authority** as a containerized MCP service for compatible agent environments and MCP clients.

Agents can request deterministic authorization before consequential tool execution and retrieve the resulting Decision Dossier.

### Docker Desktop Extension

A native Docker Desktop experience for observing governed agent activity.

The extension is designed to surface:

- live execution-authority decisions
- PROCEED, HOLD, and BLOCK outcomes
- policy and reason codes
- pending Presence approvals
- policy drift and overrides
- signed Decision Dossiers

### Presence approval

A `HOLD` decision can require human authority before an agent continues.

Presence provides action-bound human verification using supported mechanisms such as passkeys and trusted-device approval.

```text
Agent requests production deployment
              │
              ▼
          Decionis
              │
             HOLD
              │
              ▼
          Presence
              │
       Human approval
              │
              ▼
           PROCEED
              │
              ▼
           Execute
```

The agent does not decide whether human approval is necessary.

**Policy does.**

### Dev Container governance

Decionis can be incorporated into Dev Container environments so execution governance travels with the development environment.

This enables teams to establish authority boundaries for AI-assisted development without relying on every developer or agent to implement governance independently.

### Containerized execution gates

For agent architectures requiring an execution boundary, Decionis components can sit between an agent and consequential tools or APIs.

The goal is not generic network filtering.

Decionis evaluates the **meaning of the proposed action**—who or what requested it, what it intends to do, its risk and business context, and which policy governs execution.

---

## Example

An AI coding agent decides that a production infrastructure change is required:

```text
Action:       terraform.apply
Environment:  production
Requested by: AI coding agent
Risk:         HIGH
```

Before the operation executes, Decionis evaluates it:

```text
Verdict:      HOLD
Policy:       production-infrastructure-v12

Reason:
Human authorization required for
production infrastructure changes.
```

Presence requests approval from an authorized human.

After successful verification:

```text
Presence:     VERIFIED
Verdict:      PROCEED

Dossier:      dos_...
Evidence:     SIGNED
Algorithm:    Ed25519
```

Only then can the downstream system continue execution.

---

## Architecture

Decionis deliberately separates **intent, authority, human approval, and execution**.

```text
┌──────────────────────────────────────────┐
│              AI AGENT                   │
│ Claude · Cursor · VS Code · Custom      │
└────────────────────┬─────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────┐
│          TOOL / MCP LAYER                │
│ MCP · CLI · SDK · Agent Runtime          │
└────────────────────┬─────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────┐
│             DECIONIS                     │
│                                          │
│       Deterministic Authority            │
│                                          │
│ Policy · Context · Risk · Evidence       │
└────────────────────┬─────────────────────┘
                     │
               HOLD  │
                     ▼
┌──────────────────────────────────────────┐
│             PRESENCE                     │
│                                          │
│         Human Authority                  │
│                                          │
│ Passkey · Device · Verified Approval     │
└────────────────────┬─────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────┐
│             EXECUTION                    │
│                                          │
│ GitHub · AWS · DB · Terraform · APIs     │
└────────────────────┬─────────────────────┘
                     │
                     ▼
             DECISION DOSSIER
```

## Decionis does not execute the action

Decionis is an **execution-authorization control plane**.

It evaluates proposed actions and returns an authority decision. It does not execute payments, deployments, database operations, trades, infrastructure changes, or other downstream actions.

Connected agents and systems remain responsible for execution.

This separation allows Decionis to govern consequential execution without becoming the system performing it.

---

## Decision Dossiers

A governed action can produce an immutable Decision Dossier containing evidence such as:

```text
Decision
Evaluation ID
Decision type
Verdict
Reason codes
Policy version
Execution context
Presence verification
Evidence hash
Signature
Timestamp
```

Dossiers are cryptographically signed using Ed25519 and can be independently verified.

This turns:

> "The agent was allowed to do it."

into:

> **"Here is cryptographic evidence showing exactly why execution was authorized."**

---

## Use cases

Decionis for Docker is intended for consequential agent and automation workflows including:

- production deployments
- infrastructure changes
- privileged MCP tool execution
- database mutations
- package publishing
- release creation
- CI/CD operations
- cloud administration
- financial and enterprise API operations
- autonomous agent workflows

---

## Project status

This repository contains the Docker integration surface for Decionis.

Components will be released incrementally as the Docker developer experience evolves.

The initial focus is:

1. containerized Decionis MCP execution authority
2. Docker Desktop visibility into authority decisions
3. Presence approval for held actions
4. signed Decision Dossier inspection
5. Dev Container integration

---

## Security

Decionis is designed around a simple principle:

**An AI agent's ability to call a tool is not the same as authority to execute the action.**

Sensitive execution should cross an explicit, inspectable authority boundary.

If you discover a security issue, please follow the security reporting process described in `SECURITY.md` rather than opening a public issue.

---

## Contributing

Issues, integration examples, documentation improvements, and contributions to the Docker integration layer are welcome.

The Docker integration surface is intentionally separated from Decionis's hosted control-plane implementation.

See `CONTRIBUTING.md` for contribution guidelines.

---

## License

See `LICENSE` for licensing terms.

---

**Decionis for Docker**

*Your agents have tools. Decionis decides when they can use them.*%   
