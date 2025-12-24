# Dockerfile Contract for dsops
# This file documents the expected Dockerfile structure
# Actual implementation will be at repository root: Dockerfile

# ============================================================================
# Build stage
# ============================================================================
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=docker -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" \
    -o dsops \
    ./cmd/dsops

# ============================================================================
# Production stage
# ============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /build/dsops /usr/local/bin/dsops

# Set working directory for volume mounts
WORKDIR /work

# Run as non-root user (UID 65532 in distroless)
USER nonroot:nonroot

# Entrypoint
ENTRYPOINT ["/usr/local/bin/dsops"]

# Default command (show help)
CMD ["--help"]

# ============================================================================
# Labels (OCI annotations)
# ============================================================================
LABEL org.opencontainers.image.title="dsops"
LABEL org.opencontainers.image.description="Secret management for development and production environments"
LABEL org.opencontainers.image.source="https://github.com/systmms/dsops"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.vendor="systmms"

# ============================================================================
# Usage examples:
# ============================================================================
# Build:
#   docker build -t dsops .
#
# Run with config:
#   docker run --rm -v $(pwd):/work dsops plan --env production
#
# Run with secrets:
#   docker run --rm -v $(pwd):/work -v $HOME/.config:/root/.config dsops exec --env production -- command
#
# Check version:
#   docker run --rm dsops --version
#
# ============================================================================
# Size target: Under 50MB (distroless base ~2MB + Go binary ~15MB)
# ============================================================================
