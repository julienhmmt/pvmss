# Build stage
FROM golang:1.25-alpine3.22 AS builder

WORKDIR /app

# Install build dependencies and create necessary directories in one layer
RUN set -eux; \
    apk add --no-cache gcc musl-dev; \
    mkdir -p /app/backend /app/frontend /app/backend/i18n /app/backend/docs

# Copy dependency files first for better layer caching
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy backend source code
COPY backend/ ./backend/

# Build the application with optimizations
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    cd backend && \
    CGO_ENABLED=1 GOOS=linux \
    go build -trimpath -ldflags='-w -s -extldflags "-static"' -o ../pvmss-backend .

# Copy frontend files in a separate stage to keep builder image smaller
FROM alpine:3.22 AS frontend
WORKDIR /app
COPY frontend/ /app/frontend/

# Final stage
FROM alpine:3.22

# Install runtime dependencies and create user in one layer
RUN set -eux; \
    apk add --no-cache ca-certificates tzdata; \
    addgroup -g 1000 -S pvmssuser; \
    adduser -u 1000 -S pvmssuser -G pvmssuser; \
    mkdir -p /app/frontend /app/backend/i18n /app/data; \
    chown -R pvmssuser:pvmssuser /app

WORKDIR /app

# Copy from builder and frontend stages
COPY --from=builder --chown=pvmssuser:pvmssuser /app/pvmss-backend .
COPY --from=frontend --chown=pvmssuser:pvmssuser /app/frontend/ /app/frontend/
COPY --from=builder --chown=pvmssuser:pvmssuser /app/backend/i18n/ /app/backend/i18n/
COPY --from=builder --chown=pvmssuser:pvmssuser /app/backend/docs/ /app/backend/docs/

# Switch to non-root user
USER pvmssuser

# Expose the port the app runs on
EXPOSE 50000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:50000/health || exit 1

# Default command to run the application with template path
ENTRYPOINT ["/app/pvmss-backend"]
CMD ["-templates", "/app/frontend"]
