# decionis/mcp — the published @decionis/mcp evaluator (Decionis local MCP
# server) as a container image. Packaging only: evaluation logic lives
# entirely in the pinned upstream artifact (rules/coding.rules.md §2,
# rules/security.rules.md Rule 1.3).
#
# Base pinned by digest per rules/security.rules.md Rule 1.1
# (node:24-alpine, digest sweep 2026-08-18).
ARG NODE_BASE=node:24-alpine@sha256:d32cdf619f63fe0471182d08996dd516c6275bb5fd31ae06e55a570bd9e1ad43

FROM ${NODE_BASE} AS build
WORKDIR /build
COPY package.json package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts

FROM ${NODE_BASE}
ARG IMAGE_VERSION=dev
LABEL org.opencontainers.image.title="decionis/mcp" \
      org.opencontainers.image.description="Decionis local MCP evaluator: evaluate a candidate action against the repository's DECIONIS_POLICY.md before anything commits, deploys, or migrates. Zero network, zero credentials, nothing recorded." \
      org.opencontainers.image.source="https://github.com/decionis/docker" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.vendor="Decionis" \
      org.opencontainers.image.version="${IMAGE_VERSION}"

ENV NODE_ENV=production \
    DECIONIS_POLICY_PATH=/work/DECIONIS_POLICY.md

# The runtime is plain `node` — drop npm/corepack/yarn from the final image so
# their bundled dependencies stop being scan surface (rules/security.rules.md
# Rules 1.2, 1.6).
RUN rm -rf /usr/local/lib/node_modules /opt/yarn* \
    /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack \
    /usr/local/bin/yarn /usr/local/bin/yarnpkg

COPY --from=build /build/node_modules /app/node_modules

# stdio transport: run with `docker run -i`. The evaluator needs no network
# (--network none works) and writes no state (--read-only works).
WORKDIR /work
# Numeric uid:gid (the node user) so non-root is verifiable by the runtime.
USER 1000:1000
ENTRYPOINT ["node", "/app/node_modules/@decionis/mcp/dist/index.js"]
