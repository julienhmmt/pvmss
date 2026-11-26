# Build stage - Go. Runs on the build host platform: cross-compiling Go is
# free, and this keeps multi-arch builds off QEMU/Rosetta emulation.
FROM --platform=$BUILDPLATFORM golang:1.27.4-alpine AS builder

WORKDIR /app

ARG TARGETARCH

# Copy server dependency files first for better layer caching.
# The module is pvmss/server (server/go.mod), so go.mod must sit at /app/
# and source files must live directly under /app/ (not /app/server/) for
# import paths to resolve correctly.
COPY server/go.mod server/go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy server source code (module root = /app/)
COPY server/ ./

# Build the server binary (static, CGO disabled - no C toolchain needed)
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -trimpath -ldflags='-w -s' -tags netgo -o pvmss ./cmd/pvmss

# Empty /data directory carried into the final image. Without it the image has
# no /data, and a fresh named Docker volume mounted there is initialized
# root:root 0755 - unwritable by the nonroot runtime uid (SQLITE_CANTOPEN).
RUN mkdir /data

# Build SvelteKit SPA. Also on the build host platform: the output is static
# files with no arch, and emulation breaks the build (Rosetta has no AVX2, so
# Bun disables wasm-simd and the vite/rolldown wasm fails to parse).
FROM --platform=$BUILDPLATFORM oven/bun:1.4.5-alpine AS svelte-builder
WORKDIR /app/web

# Copy lockfile first for better layer caching
COPY web/package.json web/bun.lock ./

RUN bun install --frozen-lockfile

COPY web/ ./

RUN bun run build

# Final stage - using distroless for minimal attack surface and size
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app

# Copy Go binary
COPY --from=builder --chown=nonroot:nonroot /app/pvmss /app/pvmss
# SvelteKit build output
COPY --from=svelte-builder --chown=nonroot:nonroot /app/web/build/ /app/web/build/
# Database directory owned by the runtime user (see builder stage).
COPY --from=builder --chown=nonroot:nonroot /data /data

# Default database path (override at runtime with -e PVMSS_DB_PATH=...)
ENV PVMSS_DB_PATH=/data/pvmss.db
# Bind all interfaces inside the container; the host port mapping in
# docker-compose / k8s controls external exposure. Override with
# -e PVMSS_HOST=127.0.0.1 for bare-metal/loopback-only deployments.
ENV PVMSS_HOST=0.0.0.0
# Web build directory - the binary resolves this via PVMSS_WEB_DIR
# or falls back to a path relative to the executable.
ENV PVMSS_WEB_DIR=/app/web/build

# Expose the port the app runs on
EXPOSE 50000

# Entry point - the server binary (no -templates flag; web dir is
# resolved via PVMSS_WEB_DIR env var or relative to the executable).
ENTRYPOINT ["/app/pvmss"]
