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
requests is enumerated and justified here, and this list is re-reviewed on
any change to `extension/metadata.json` or `extension/compose.yaml`.

| Privilege | Requested | Justification |
| --------- | --------- | ------------- |
| Backend VM service (`vm.composefile`) | Yes | The Go daemon (`decionis-daemon`) holds the org API key, calls the published hosted API, and verifies dossiers. Credentials must live in the backend, never the UI (Rules 2.2–2.3). |
| Extension socket (`vm.exposes.socket`) | Yes | The only channel between the UI and the daemon. Every request over it is schema-validated with a 100 KB bound (Rule 4.4). |
| Private named volume (`decionis-data:/data`) | Yes | Connection settings and the org API key at rest, `0600` inside the backend's private volume (Rule 2.5). No host paths are mounted. |
| Root user inside the extension VM container | Yes (today) | Required to bind `/run/guest-services/backend.sock`. Revisit for a non-root bind once verified against current Docker Desktop. |
| Docker Engine socket | **No** | The extension does not mount or use the Engine socket (Rule 4.2). |
| Host binaries | **No** | None are shipped. |
| Host filesystem mounts | **No** | None. |
| Outbound network | Daemon only | HTTPS to the connected org's Decionis control plane and the dossier JWKS URL; certificate verification always on (Rule 2.6). Plus one anonymous, credential-free GET of the extension's own public Docker Hub tags listing for the update banner (cached 6 h; carries no identifiers; fail-open — an unreachable listing claims nothing). |
