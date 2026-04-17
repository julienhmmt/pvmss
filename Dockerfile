# Build stage - Go
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies and create necessary directories
# Keep gcc/musl-dev present for the static build; remove later if needed.
RUN set -eux; \
    apk add --no-cache gcc musl-dev; \
    mkdir -p /app/backend /app/frontend /app/backend/i18n /app/backend/docs

# Copy dependency files first for better layer caching
COPY backend/go.mod backend/go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy backend source code
COPY backend/ ./backend/

# Build the application (static, no external libc dependency)
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    cd backend && \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags='-w -s' -tags netgo -o ../pvmss-backend .

# Build SvelteKit SPA
FROM node:lts-alpine AS svelte-builder
WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./

RUN npm install

COPY frontend/ ./

RUN npm run build

# Final stage - using distroless for minimal attack surface and size
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app

# Copy Go binary
COPY --from=builder --chown=nonroot:nonroot /app/pvmss-backend /app/pvmss-backend
# SvelteKit build output — includes static assets (noVNC, robots.txt, etc.)
COPY --from=svelte-builder --chown=nonroot:nonroot /app/frontend/build/ /app/frontend/build/
COPY --from=builder --chown=nonroot:nonroot /app/backend/i18n/ /app/backend/i18n/
COPY --from=builder --chown=nonroot:nonroot /app/backend/docs/ /app/backend/docs/

# Default database path (override at runtime with -e PVMSS_DB_PATH=...)
ENV PVMSS_DB_PATH=/data/pvmss.db

# Expose the port the app runs on
EXPOSE 50000

# Default command to run the application with template path
ENTRYPOINT ["/app/pvmss-backend","-templates","/app/frontend"]
