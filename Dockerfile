# Build stage - Go
FROM golang:1.25.3-alpine3.22 AS builder

WORKDIR /app

# Install build dependencies and create necessary directories in one layer
RUN set -eux; \
    apk add --no-cache --virtual .build-deps gcc musl-dev ; \
    mkdir -p /app/backend /app/frontend /app/backend/i18n /app/backend/docs ; \
    apk del .build-deps

# Copy dependency files first for better layer caching
COPY backend/go.mod backend/go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy backend source code
COPY backend/ ./backend/

# Build the application
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    cd backend && \
    CGO_ENABLED=1 GOOS=linux \
    go build -a -trimpath -ldflags='-w -s -extldflags "-static"' -tags netgo -installsuffix netgo -o ../pvmss-backend .

# Copy frontend files in a separate stage to keep builder image smaller
FROM alpine:3.22 AS frontend
WORKDIR /app

COPY frontend/ /app/frontend/

RUN set -eux; \
    apk add --no-cache wget; \
    mkdir -p /app/frontend/components/noVNC-1.6.0; \
    wget -O /tmp/noVNC.tar.gz "https://github.com/novnc/noVNC/archive/refs/tags/v1.6.0.tar.gz"; \
    tar -xzf /tmp/noVNC.tar.gz --strip-components=1 -C /app/frontend/components/noVNC-1.6.0; \
    rm /tmp/noVNC.tar.gz; \
    rm -rf /app/frontend/components/noVNC-1.6.0/tests \
           /app/frontend/components/noVNC-1.6.0/docs \
           /app/frontend/components/noVNC-1.6.0/po \
           /app/frontend/components/noVNC-1.6.0/utils \
           /app/frontend/components/noVNC-1.6.0/snap \
           /app/frontend/components/noVNC-1.6.0/README.md; \
    find /app/frontend/components/noVNC-1.6.0/app/locale -type f ! -name 'en.json' ! -name 'fr.json' -delete; \
    apk del wget

# Final stage - using distroless for minimal attack surface and size
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app

# Copy from builder and frontend stages
COPY --from=builder --chown=nonroot:nonroot /app/pvmss-backend /app/pvmss-backend
COPY --from=frontend --chown=nonroot:nonroot /app/frontend/ /app/frontend/
COPY --from=builder --chown=nonroot:nonroot /app/backend/i18n/ /app/backend/i18n/
COPY --from=builder --chown=nonroot:nonroot /app/backend/docs/ /app/backend/docs/

# Expose the port the app runs on
EXPOSE 50000

# Default command to run the application with template path
ENTRYPOINT ["/app/pvmss-backend","-templates","/app/frontend"]
