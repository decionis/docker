# Decionis governance (`govern`)

Installs the [`decionis` CLI](https://github.com/decionis/docker) inside a dev
container so execution governance travels with the repository:

- `decionis govern -- <command>` — evaluates the intent against the
  repository's `DECIONIS_POLICY.md` through the real Decionis evaluator and
  runs the command only on an `allow` verdict. Verdict behavior follows the
  evaluator's own vocabulary: allow → proceed; block → do not proceed;
  review/escalate → stop and involve a human. Evaluation failures fail
  closed: the command is not executed.
- `decionis verify <dossier.json>` — verifies a Decision Dossier's Ed25519
  proof bundle offline against the public JWKS.
- `decionis hooks install` — wires the containerized `decionis/mcp` evaluator
  into stdio MCP clients (`.mcp.json`).
- `decionis status` — evaluator and policy health.

## Usage

```json
{
  "features": {
    "ghcr.io/decionis/features/govern:0": {}
  }
}
```

Pin a CLI version explicitly:

```json
{
  "features": {
    "ghcr.io/decionis/features/govern:0": { "version": "0.1.0" }
  }
}
```

## Options

| Option    | Type   | Default  | Description                                              |
| --------- | ------ | -------- | -------------------------------------------------------- |
| `version` | string | `latest` | decionis CLI release version to install, or `latest`.    |

## Notes

- Binaries download from this repository's GitHub releases with mandatory
  SHA-256 checksum verification; the install fails rather than installing an
  unverified binary.
- `decionis govern`'s default evaluator is the `decionis/mcp` container
  (zero network, zero credentials, nothing recorded); inside dev containers
  without Docker, pass `-evaluator-cmd "npx -y @decionis/mcp"` instead.

## Support & license

- Documentation: https://decionis.com/docs/protocol-mcp
- Issues: https://github.com/decionis/docker/issues
- License: Apache-2.0
