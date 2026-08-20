# Decionis Docker Desktop extension image: React UI + the Go daemon backend.
# Built from the repo root: docker build -f extension/Dockerfile .
#
# Bases pinned by digest per rules/security.rules.md Rule 1.1
# (digest sweep 2026-08-18).
ARG NODE_BASE=node:24-alpine@sha256:d32cdf619f63fe0471182d08996dd516c6275bb5fd31ae06e55a570bd9e1ad43
ARG GO_BASE=golang:1.26-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83
ARG ALPINE_BASE=alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce

# Build stages run on the build platform (no QEMU): the UI output is
# platform-independent and the Go daemon cross-compiles via TARGETOS/TARGETARCH.
FROM --platform=${BUILDPLATFORM} ${NODE_BASE} AS ui-build
WORKDIR /ui
COPY extension/ui/package.json extension/ui/package-lock.json ./
RUN npm ci --ignore-scripts
COPY extension/ui/ ./
RUN npm run build

FROM --platform=${BUILDPLATFORM} ${GO_BASE} AS daemon-build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG IMAGE_VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags "-s -w -X github.com/decionis/docker/internal/version.Version=${IMAGE_VERSION}" \
    -o /backend ./cmd/decionis-daemon

FROM ${ALPINE_BASE}
ARG IMAGE_VERSION=dev
LABEL org.opencontainers.image.title="Decionis" \
    org.opencontainers.image.description="Execution authority for AI agents: live Decionis decision evaluations, verdicts, and offline Ed25519 verification of signed Decision Dossiers, inside Docker Desktop." \
    org.opencontainers.image.vendor="Decionis" \
    org.opencontainers.image.licenses="Apache-2.0" \
    org.opencontainers.image.source="https://github.com/decionis/docker" \
    org.opencontainers.image.version="${IMAGE_VERSION}" \
    com.docker.desktop.extension.api.version=">= 0.3.4" \
    com.docker.desktop.extension.icon="https://decionis.com/favicon.ico" \
    com.docker.extension.detailed-description="<h1>Decionis — execution authority for AI agents</h1><p>Decionis evaluates consequential agent actions against deterministic, versioned policy before execution. This extension connects to your Decionis org and shows live decision evaluations — outcomes APPROVE, ESCALATE, REJECT, and REVIEW — with the policy version and reason behind each one.</p><p>Every governed decision can produce a signed Decision Dossier. The extension fetches dossiers and verifies their Ed25519 proof bundles offline against the published JWKS, and reports whether a dossier carries everything needed to independently reproduce the decision.</p><p>The org API key is held by the extension backend only; the local evaluator workflow (the decionis/mcp image) requires no credentials at all.</p>" \
    com.docker.extension.screenshots="[{\"alt\":\"Decision evaluations feed with outcome verdicts and summary counts\",\"url\":\"https://raw.githubusercontent.com/decionis/docker/00e21830ed295dbf1a820be56efe8bef20860946/docs/screenshots/decisions-feed.png\"},{\"alt\":\"Decision Dossier inspector with offline Ed25519 verification checks and reproducibility posture\",\"url\":\"https://raw.githubusercontent.com/decionis/docker/00e21830ed295dbf1a820be56efe8bef20860946/docs/screenshots/dossier-inspector.png\"}]" \
    com.docker.extension.publisher-url="https://decionis.com" \
    com.docker.extension.additional-urls="[{\"title\":\"Documentation\",\"url\":\"https://decionis.com/docs/protocol-mcp\"},{\"title\":\"Repository\",\"url\":\"https://github.com/decionis/docker\"}]" \
    com.docker.extension.categories="security" \
    com.docker.extension.changelog="<p>0.1.8: investigate decisions in place — search plus outcome, execution, policy, and domain/type filters over the loaded reports, a complete decision-details view, and policy-mismatch and near-miss counts from the protocol summary. Approvals link straight to their Decision Dossier, and a dossier's payload, verification, and reproducibility data export locally as JSON. New Docker policy pack: ten fixed action proposals — benign reads, destructive actions, high-value changes — evaluated live by your workspace policy, with the exact returned outcome and nothing executed. 0.1.7: see what is waiting on a person — a new approvals panel lists the decisions held for human review, so nothing sits unnoticed. Claim workspace turns the workspace created on first run into one owned by your Decionis account, keeping every decision already recorded in it. 0.1.6: repairs 0.1.5, which was published without its application files and could not run — install or update to 0.1.6. 0.1.5: works the moment you install it — a workspace is created automatically on first run, so the decision feed is live with no account and no sign-in. Enforcement is now a checkbox (shadow by default) with 25 free governed decisions; shadow evaluations stay unlimited. Already have a Decionis account? Sign in with your email and password — no workspace ID or API key to copy. 0.1.4: one-click connect. 0.1.3: update banner. 0.1.2: enrollment-token connect. Initial preview: live decision evaluations, decision summary, offline Ed25519 dossier verification, reproducibility posture.</p>"

COPY extension/metadata.json /metadata.json
COPY extension/icon.svg /icon.svg
COPY extension/decionis-logo.png /decionis-logo.png
COPY extension/compose.yaml /compose.yaml
COPY --from=ui-build /ui/build /ui
COPY --from=daemon-build /backend /backend

# Runs as root inside the extension VM: required today to bind the
# /run/guest-services extension socket; enumerated and justified in
# SECURITY.md (rules/security.rules.md Rule 4.1).
CMD ["/backend", "-socket", "/run/guest-services/backend.sock", "-data", "/data"]
