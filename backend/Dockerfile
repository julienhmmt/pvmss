# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Download dependencies first (better layer caching)
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Build the application
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags='-w -s' -o /pvmss-backend .

# Final stage
FROM alpine:3.22

# Install CA certificates and timezone data
RUN apk --no-cache upgrade && \
    apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 -S pvmssuser && \
    adduser -u 1000 -S pvmssuser -G pvmssuser && \
    mkdir -p /app && \
    chown -R pvmssuser:pvmssuser /app

WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=pvmssuser:pvmssuser /pvmss-backend .

# Switch to non-root user
USER pvmssuser

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:50000/health || exit 1

# Expose the port the app runs on
EXPOSE 50000

# Command to run the application
CMD ["/app/pvmss-backend"]
