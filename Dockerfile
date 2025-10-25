# Enterprise-Grade Multi-stage build for Go load balancer
FROM golang:1.24-alpine AS builder

# Install necessary packages
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with production flags
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s -X main.version=2.0.0 -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o loadbalancer .

# Final stage with security hardening
FROM alpine:3.18

# Install ca-certificates for HTTPS requests and monitoring tools
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    && rm -rf /var/cache/apk/*

# Create app user (non-root for security)
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create and set up working directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/loadbalancer .

# Copy loadbalancer modules
COPY --from=builder /app/loadbalancer/ ./loadbalancer/

# Create necessary directories
RUN mkdir -p /root/loadbalancer /tmp/logs

# Change ownership
RUN chown -R appuser:appgroup /root/

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Enhanced health check with circuit breaker verification
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:8080/health && \
      curl -f http://localhost:8080/metrics || exit 1

# Environment variables
ENV LOG_LEVEL=INFO
ENV CIRCUIT_BREAKER_THRESHOLD=5
ENV ENABLE_TELEMETRY=true
ENV SERVICE_NAME=go-loadbalancer
ENV SERVICE_VERSION=2.0.0

# Labels for container identification
LABEL maintainer="Enterprise Load Balancer" \
      version="2.0.0" \
      features="circuit-breaker,redis-sessions,telemetry" \
      security.enabled=true

# Run the application with optimized flags
CMD ["./loadbalancer"]
