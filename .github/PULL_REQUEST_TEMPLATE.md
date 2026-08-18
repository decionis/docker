## What & why

<!-- One or two sentences. Link issues. -->

## Boundary check (rules/coding.rules.md §2)

- [ ] No policy evaluation, verdict semantics, or dossier issuance added here; evaluator changes went upstream
- [ ] Verdict/outcome identifiers use the protocol's published vocabulary

## Discovery (rules/discovery.rules.md §6)

- [ ] Overviews, manifests, and labels updated with the change; drift gates extended
- [ ] Any new marketplace/registry URL live-verified today; absolute links only
- [ ] No unsupported latency/accuracy figures; capability claims in the owning doc's vocabulary

## Security (rules/security.rules.md §8)

- [ ] Fail-closed paths tested for anything that gates execution
- [ ] No secrets in code, images, labels, logs, or fixtures; redaction covered by tests where relevant
- [ ] Image changes: digest-pinned base, non-root, scan gate green
- [ ] Extension privilege changes justified in SECURITY.md
