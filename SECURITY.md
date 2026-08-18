# Security

## Reporting a vulnerability

Report suspected vulnerabilities in this repository's components privately to
`security@decionis.ai`. Please do not open public issues for security reports.

Do not include production API keys, customer payloads, private policy
documents, or unredacted dossier contents in reports or public issues.

## Scope

This repository is the Docker integration surface for Decionis: the
containerized MCP evaluator image, the Docker Desktop extension, and the Go
daemon, CLI, authority proxy, and Dev Container helper as they land. Reports
about the Decionis hosted control plane or the `@decionis/mcp` package itself
are welcome at the same address; they are triaged together with the upstream
`decionis/decionis` repository.

## Security posture of this repository

The operational rules this repository is built and released under live in
[`rules/security.rules.md`](https://github.com/decionis/docker/blob/master/rules/security.rules.md).
The load-bearing ones:

- No component here mints authority. Policy evaluation happens only in the
  containerized `@decionis/mcp` evaluator or the hosted control plane.
- Fail closed: an unreachable evaluator, timeout, or unverifiable response is
  treated as a blocking outcome. A timeout is never an approval.
- Verification-only cryptography: dossiers and Presence proofs are verified
  against published JWKS; signing material never exists in this repository.
- Images: digest-pinned bases, non-root, multi-stage builds, SBOM and
  provenance attestations on release, and a vulnerability-scan gate.
- Secrets: the local evaluator requires none by design; connected-mode org
  API keys live only in the daemon/backend — never in images, the extension
  UI, logs, or this repository.

## Docker Desktop extension privileges

Per `rules/security.rules.md` Rule 4.1, every privilege the extension
requests must be enumerated and justified here. The extension has not shipped
yet; the pull request that lands it must populate this section.
