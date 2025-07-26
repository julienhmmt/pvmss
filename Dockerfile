# Build stage
FROM golang:1.24-alpine3.22 AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# First, copy only the files needed for dependencies
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy backend files (including i18n and docs)
COPY backend/ ./backend/
# Copy frontend files to the correct location
COPY frontend/ /app/frontend/
# Ensure docs directory exists
RUN mkdir -p /app/backend/docs

# Build the application
WORKDIR /app/backend
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags='-w -s -extldflags "-static"' -o ../pvmss-backend .
WORKDIR /app

# Final stage
FROM alpine:3.22

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata && \
    addgroup -g 1000 -S pvmssuser && \
    adduser -u 1000 -S pvmssuser -G pvmssuser && \
    mkdir -p /app/frontend /app/backend/i18n /app/data && \
    chown -R pvmssuser:pvmssuser /app

WORKDIR /app

# Copy built binary and assets from builder
COPY --from=builder /app/pvmss-backend .
COPY --from=builder /app/frontend/ /app/frontend/
COPY --from=builder /app/backend/i18n/ /app/backend/i18n/
COPY --from=builder /app/backend/docs/ /app/backend/docs/

# Create necessary directories and symlinks
RUN mkdir -p /app/i18n && \
    ln -sf /app/backend/i18n/* /app/i18n/ && \
    ln -sf /app/backend/i18n /app/i18n

# Set proper permissions
RUN chown -R pvmssuser:pvmssuser /app

# Switch to non-root user
USER pvmssuser

# Expose the port the app runs on
EXPOSE 50000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:50000/health || exit 1

# Default command to run the application with template path
CMD ["/app/pvmss-backend", "-templates", "/app/frontend"]
