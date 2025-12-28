# Dockerfile for dsops
# Multi-stage build for minimal production image

# ============================================================================
# Build stage
# ============================================================================
FROM golang:1.25.5-alpine AS builder

WORKDIR /build

# Install CA certificates and git for version info
RUN apk add --no-cache ca-certificates git

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
ARG VERSION=docker
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
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
