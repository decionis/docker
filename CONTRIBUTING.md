# Contributing

Thanks for improving Decionis for Docker.

## What this repository is

The Docker integration surface for Decionis: a containerized build of the
published `@decionis/mcp` evaluator, a Docker Desktop extension, and the Go
daemon, CLI, authority proxy, and Dev Container helper.

The protocol/API is the stable boundary. Components here capture intent
before execution, delegate policy judgment to Decionis, enforce non-allow
decisions locally, and preserve audit-friendly error details. Do not add
threshold logic, policy rules, risk formulas, or hardcoded guardrails to
runtime code or examples — and do not re-implement the policy evaluator,
verdict semantics, or dossier issuance here. A capability that needs
evaluator changes is an upstream change to `decionis/decionis`, consumed
here at a pinned version.

## Ground rules

`rules/coding.rules.md`, `rules/discovery.rules.md`, and
`rules/security.rules.md` are release requirements, not suggestions. The
pull request template embeds their checklists.

## Local checks

- Go: `gofmt -l .` prints nothing; `go vet ./...`; `go test ./...`
- MCP image:
  `docker build -t decionis/mcp:dev mcp-server/ && bash mcp-server/test/smoke.sh decionis/mcp:dev`
- Extension: checks land together with the extension.

## Licensing

Contributions are accepted under the repository's Apache-2.0 license.
