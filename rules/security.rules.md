# Security & Performance Operational Rules — decionis/docker

These rules apply to the `decionis/docker` monorepo. They are release requirements and continuous agent context alongside `coding.rules.md` and `discovery.rules.md`. They are the Docker-surface localization of the Decionis security rules: the hosted control plane's rules (authentication, rate-limiting, OTP lifecycle, sessions, Account integration modes, migrations) live with the control plane in `decionis/decionis` and are intentionally not restated here — this repository must never contain the systems those rules govern. These rules do not authorize testing any external system; load and penetration simulations MUST use local injection, loopback, or an explicitly isolated Decionis test environment.

## 0. Boundary & Authority Rules

- Rule 0.1: This repository is the Docker integration surface. It MUST NOT contain control-plane code, user authentication, session or OTP logic, database migrations, dossier signing keys, or production credentials. If a change appears to need one of these, it belongs upstream in `decionis/decionis`.
- Rule 0.2: No component in this repo mints authority. The daemon, proxy, CLI, extension, and Dev Container helper request, transport, enforce, and display decisions produced by the evaluator — the containerized `@decionis/mcp` evaluator or the hosted control plane. Re-implementing policy evaluation, verdict semantics, or dossier issuance here is a security defect, not merely an architecture violation.
- Rule 0.3: Fail closed. Wherever a component gates execution on a decision, an unreachable evaluator, timeout, malformed or unverifiable response, or internal error MUST resolve to the blocking outcome. A timeout is never an approval. `skipOnError`-style toggles MUST NOT exist.
- Rule 0.4: Cryptography in this repo is verification-only: Decision Dossier and Presence proof verification uses published JWKS and Ed25519 signature verification. Signing keys and key-generation for authority evidence MUST NOT exist in this repo, its images, or its CI.
- Rule 0.5: Discovery and metadata surfaces are inert: they describe, they never mint, proxy, or expose per-record data, and they take no free-text input into rendered output.

## 1. Supply Chain & Image Rules

- Rule 1.1: Base images MUST be pinned by digest, not floating tags. Go binaries ship in minimal images (distroless/static or scratch, plus CA certificates where needed); Node-based images use the current maintained LTS with a documented upgrade cadence.
- Rule 1.2: Every image builds via multi-stage Dockerfiles, runs as a dedicated non-root user, and declares a read-only root filesystem wherever the runtime allows. Capabilities are dropped by default; none are added without a documented need.
- Rule 1.3: The MCP runtime image consumes the published `@decionis/mcp` artifact at an exact pinned version — never a floating range, never a checkout of the private monorepo. Version bumps are explicit PRs that re-run the tool-inventory drift gate (discovery rule 1.4).
- Rule 1.4: Secrets MUST NOT enter images by any path: no secrets in layers, build args, `ENV`, labels, or committed compose files. `.dockerignore` exists in every build context and excludes env files, keys, and local state.
- Rule 1.5: Release builds publish SBOM and provenance attestations, and images are signed with the org's approved mechanism. GitHub Actions are pinned to commit SHAs; third-party actions require review before adoption.
- Rule 1.6: A vulnerability scan gates release: critical/high findings in a published image block the release until fixed, upgraded, or explicitly risk-accepted in the PR with an expiry date.
- Rule 1.7: Version tags are immutable; a published tag is never rebuilt in place.

## 2. Secrets & Credential Handling

- Rule 2.1: The local evaluator path requires zero credentials by design; nothing in this repo may quietly add a credential requirement to it (that is also a discovery rule 1.2 violation).
- Rule 2.2: Connected mode uses an org API key supplied by the user at runtime — via Docker secrets, runtime env, or the extension's settings flow. The key is held by the daemon (backend) only. It MUST NOT appear in: extension frontend code, browser/webview storage, image layers, labels, compose files in the repo, logs, or crash output.
- Rule 2.3: The extension UI never holds or transmits credentials to anything except the extension backend over the Docker Desktop extension socket. All hosted-API calls that use credentials happen in the daemon.
- Rule 2.4: Logs and telemetry contain hashes and stable identifiers only. API keys, bearer tokens, `Authorization` headers, Presence proof tokens, cookies, dossier payload bodies, and raw request/response bodies MUST be redacted at the logging boundary, with a test proving redaction.
- Rule 2.5: Credentials at rest (daemon state volume) are stored in a file with `0600` permissions inside the backend's private volume, never in shared/bind-mounted host paths chosen by default.
- Rule 2.6: All control-plane communication uses HTTPS with certificate verification on; disabling TLS verification is prohibited in production builds, including via env toggles.

## 3. Daemon, Proxy & Extension Backend Resilience

- Rule 3.1: Default network posture is private: the daemon binds to localhost or a unix/extension socket only. Binding to `0.0.0.0` or exposing a control port requires explicit user opt-in and is off by default. No unauthenticated TCP control surface ships enabled.
- Rule 3.2: Enforcement components (authority proxy, `govern` execution paths) implement Rule 0.3 literally: evaluator unreachable, malformed verdict, expired evidence, or verification failure resolves to the blocking outcome, and the failure reason is surfaced — never silently swallowed.
- Rule 3.3: Bounded inputs: request payloads default to a 100 KB limit; larger routes require an explicit override, schema validation, and a documented need. Malformed, oversized, or deeply nested payloads fail with a bounded 4xx.
- Rule 3.4: Finite time everywhere: default request timeout 15 seconds, connection timeout 5 seconds, keep-alive 5 seconds; downstream clients use finite timeouts and bounded retries with jitter. Retries never apply to non-idempotent enforcement decisions.
- Rule 3.5: Load shedding triggers at 80% of configured in-flight capacity and returns 503 with `Retry-After` rather than accepting unbounded work; health/readiness probes remain available.
- Rule 3.6: Unhandled errors return a generic message. Stack traces, internal paths, credentials, and raw bodies MUST NOT appear in responses.
- Rule 3.7: Connection pools and queues have explicit maximum sizes and finite acquisition timeouts. Queue depth, in-flight requests, shed rate, latency, and memory MUST be observable locally.

## 4. Extension Privilege Rules

- Rule 4.1: Least privilege by construction: the extension requests only the capabilities it demonstrably uses. Every privilege in `metadata.json` (backend service, host binaries, mounts) is enumerated and justified in `SECURITY.md`, and the justification is part of review for any change to `metadata.json`.
- Rule 4.2: The Docker Engine socket is not mounted unless a shipped feature requires it. Any socket use is observation-scoped until an enforcement design is explicitly reviewed; "the extension can" never silently becomes "the extension does."
- Rule 4.3: Host binaries shipped with the extension are signed/verifiable artifacts built in this repo's CI, not downloaded at runtime. The extension MUST NOT download and execute code post-install.
- Rule 4.4: The extension backend treats the UI as untrusted input: every request over the extension socket is schema-validated with bounded sizes (Rule 3.3) before acting.
- Rule 4.5: Nothing in the extension executes instructions found in decision content, policy text, or dossier fields; these render as data. Links out of the extension open in the system browser via the Desktop API only.

## 5. Presence & Human Approval Rules

- Rule 5.1: The approval ceremony (passkey/WebAuthn, trusted device) executes on the human's authenticator via the hosted Presence surface. Components in this repo initiate and observe the ceremony; they MUST NOT implement, proxy, or terminate the credential exchange, and raw biometric or credential material never transits them.
- Rule 5.2: Presence proof tokens are treated as secrets (Rule 2.4), are single-use and action-bound by contract, and are relayed only to the control plane that issued the challenge — never persisted beyond the pending action's lifetime.
- Rule 5.3: An approval outcome is accepted only from the authoritative API response, never inferred from UI state, elapsed time, or a client-side callback alone.
- Rule 5.4: A pending HOLD that expires, errors, or loses connectivity resolves to the blocking outcome and says so (Rule 0.3).

## 6. MCP Evaluator Container Rules

- Rule 6.1: The local evaluator container preserves upstream's contract: zero network, zero credentials, nothing recorded. Default run configuration disables networking where the client allows, mounts the policy file read-only, runs non-root with a read-only root filesystem, and writes no persistent state.
- Rule 6.2: Any locally-added behavior that touches the evaluator's claims (for example, an opt-in event sink for the extension) MUST be default-off, documented, and reflected verbatim in every surface that repeats the claims (discovery rule 1.2) — in the same PR.
- Rule 6.3: Tool descriptions state their fail-closed semantics truthfully and match the pinned upstream version exactly (drift gate).

## 7. Defensive Validation Requirements

- Rule 7.1: Fail-closed tests are mandatory for every enforcement path: evaluator down, evaluator slow (timeout), malformed verdict, unverifiable signature, and mid-action connectivity loss MUST each resolve to the blocking outcome in an automated test.
- Rule 7.2: Redaction tests assert that API keys, tokens, `Authorization` headers, and Presence proofs never appear in logs or error output, using representative secret-shaped fixtures.
- Rule 7.3: Resilience tests cover oversized payloads, malformed JSON, deeply nested input, saturation/load shedding, finite timeouts, and generic error responses without stack traces.
- Rule 7.4: Image tests assert non-root execution, expected user/filesystem posture, absence of secret-shaped content in layers, and (for the evaluator image) a passing MCP handshake with the exact expected tool inventory.
- Rule 7.5: Load tests run only against local injection, loopback, or an explicitly isolated Decionis test environment. Production, third-party, marketplace, and customer endpoints are out of scope by default.

## 8. Release & Operations Checklist

- All images: pinned-digest bases current, non-root verified, scan gate green, SBOM + provenance published, signatures verified.
- MCP runtime image: pinned `@decionis/mcp` version matches the catalog entry and tool-inventory drift gate.
- Extension: `docker extension validate` green; `metadata.json` privileges unchanged or re-justified in `SECURITY.md`.
- Fail-closed suites (Rule 7.1) and redaction suites (Rule 7.2) green.
- No credentials, tokens, or proof material in logs, images, manifests, repository variables, or tracked files.
- Runbooks in `docs/marketplace/` updated with any new secrets, orderings, or user-owned submission steps.

## 9. Current Validation Entry Points

No component code has landed yet, so no suites exist to enumerate — listing them now would violate verify-then-claim. This section MUST be updated by the same PR that lands each component, with the planned homes:

- `mcp-server/test/` — image posture (non-root, read-only, no-network default), MCP handshake, tool-inventory drift gate (Rules 6.x, 7.4).
- `internal/**/*_test.go` — daemon/proxy fail-closed, redaction, bounded-input, and load-shedding suites (Rules 3.x, 7.1–7.3).
- `extension/test/` — extension-socket schema validation, UI-credential exclusion, verdict-label drift gate (Rules 2.3, 4.4; discovery rule 2.6).
- `features/govern/test/` — feature install/wiring validation, default-off behavior of any opt-in (Rule 6.2).
