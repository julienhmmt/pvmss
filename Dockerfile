# Build stage - Go (v0.4 server)
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Copy server dependency files first for better layer caching.
# The module is pvmss/server (server/go.mod), so go.mod must sit at /app/
# and source files must live directly under /app/ (not /app/server/) for
# import paths to resolve correctly.
COPY server/go.mod server/go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy server source code (module root = /app/)
COPY server/ ./

# Build the v0.4 server binary (static, CGO disabled — no C toolchain needed)
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags='-w -s' -tags netgo -o pvmss ./cmd/pvmss

# Build SvelteKit SPA (v0.4 web)
FROM oven/bun:1-alpine AS svelte-builder
WORKDIR /app/web

# Copy lockfile first for better layer caching
COPY web/package.json web/bun.lock ./

RUN bun install --frozen-lockfile

COPY web/ ./

RUN bun run build

# Final stage - using distroless for minimal attack surface and size
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app

# Copy v0.4 Go binary
COPY --from=builder --chown=nonroot:nonroot /app/pvmss /app/pvmss
# SvelteKit build output (v0.4 web)
COPY --from=svelte-builder --chown=nonroot:nonroot /app/web/build/ /app/web/build/

# Default database path (override at runtime with -e PVMSS_DB_PATH=...)
ENV PVMSS_DB_PATH=/data/pvmss.db
# Bind all interfaces inside the container; the host port mapping in
# docker-compose / k8s controls external exposure. Override with
# -e PVMSS_HOST=127.0.0.1 for bare-metal/loopback-only deployments.
ENV PVMSS_HOST=0.0.0.0
# Web build directory — the v0.4 binary resolves this via PVMSS_WEB_DIR
# or falls back to a path relative to the executable.
ENV PVMSS_WEB_DIR=/app/web/build

# Expose the port the app runs on
EXPOSE 50000

# v0.4 entrypoint — the server binary (no -templates flag; web dir is
# resolved via PVMSS_WEB_DIR env var or relative to the executable).
ENTRYPOINT ["/app/pvmss"]
