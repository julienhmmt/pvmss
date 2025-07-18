# Build stage
FROM golang:1.24-alpine3.22 AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-w -s' -o pvmss-backend .

# Final stage
FROM alpine:3.22

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    tzdata && \
    addgroup -g 1000 -S pvmssuser && \
    adduser -u 1000 -S pvmssuser -G pvmssuser && \
    mkdir -p /app/frontend/css /app/frontend/js && \
    chown -R pvmssuser:pvmssuser /app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/pvmss-backend .
COPY --from=builder /app/docs ./docs/

# Copy frontend files (will be overridden by volume in development)
COPY --from=builder /app/../frontend/ ./frontend/

# Create a symlink to make paths work in both dev and prod
RUN ln -s /app/frontend /frontend

# Switch to non-root user
USER pvmssuser

# Expose the port the app runs on
EXPOSE 50000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:50000/health || exit 1

# Command to run the executable
CMD ["/app/pvmss-backend"]
