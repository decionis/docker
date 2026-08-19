# Decionis authority gate (image decionis/authority): the action-aware
# execution gate. Built from the repo root:
#   docker build -f cmd/authority-proxy/Dockerfile .
#
# Bases pinned by digest per rules/security.rules.md Rule 1.1.
ARG GO_BASE=golang:1.26-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83
ARG ALPINE_BASE=alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce

FROM --platform=${BUILDPLATFORM} ${GO_BASE} AS build
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
    -o /authority ./cmd/authority-proxy

FROM ${ALPINE_BASE}
ARG IMAGE_VERSION=dev
LABEL org.opencontainers.image.title="Decionis authority gate" \
    org.opencontainers.image.description="Action-aware execution gate: describe an action, get a verdict from Decionis, enforce it. No traffic interception, no local policy evaluation." \
    org.opencontainers.image.vendor="Decionis" \
    org.opencontainers.image.licenses="Apache-2.0" \
    org.opencontainers.image.source="https://github.com/decionis/docker" \
    org.opencontainers.image.version="${IMAGE_VERSION}"

# Credentials arrive as environment variables (DECIONIS_ORG_ID,
# DECIONIS_API_KEY) — never baked in, never flags.
COPY --from=build /authority /authority

# Unprivileged: the gate binds a port and calls out. It needs nothing else,
# no Docker socket, no host paths (rules/security.rules.md Rule 4.2).
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/authority"]
