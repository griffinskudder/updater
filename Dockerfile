# Multi-stage Dockerfile for Go Updater Service
# Uses distroless base image for maximum security

# =============================================================================
# Build Stage
# =============================================================================
FROM golang:1.25-alpine AS builder

# Security: Create non-root user for build process
RUN adduser -D -s /bin/sh -u 1001 appuser

# Security: Install only necessary packages and clean up
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/*

# Set working directory
WORKDIR /build

RUN mkdir -p /app/data && chown appuser:appuser /app/data

# Security: Change ownership of build directory
RUN chown appuser:appuser /build
USER appuser

# Copy go mod files first for better caching
COPY --chown=appuser:appuser go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY --chown=appuser:appuser . .

# Security: Build with security flags and static linking
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
    -a \
    -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -tags netgo \
    -o updater \
    ./cmd/updater

# Build the health check binary (used by Docker Compose / distroless HEALTHCHECK)
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
    -a \
    -ldflags='-w -s -extldflags "-static"' \
    -o healthcheck \
    ./cmd/healthcheck

# =============================================================================
# Runtime Stage - Distroless (OS-less)
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# Security: Use non-root user (ID 65532 from distroless)
USER 65532:65532

# Security: Set read-only root filesystem
# Note: This will be enforced via docker run flags or Kubernetes securityContext

# Copy timezone data and certificates from builder
COPY --from=builder --chown=65532:65532 /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder --chown=65532:65532 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the statically linked binary and health check helper
COPY --from=builder --chown=65532:65532 /build/updater /usr/local/bin/updater
COPY --from=builder --chown=65532:65532 /build/healthcheck /usr/local/bin/healthcheck

COPY --from=builder --chown=65532:65532 /app /app

# Set working directory
WORKDIR /app

# Security: Expose only necessary port
EXPOSE 8080

# Security: Set resource limits and security options via labels
LABEL \
    org.opencontainers.image.title="Updater Service" \
    org.opencontainers.image.description="Secure update service for desktop applications" \
    org.opencontainers.image.vendor="griffinskudder" \
    org.opencontainers.image.version="1.0.0" \
    org.opencontainers.image.created="2024-01-01T00:00:00Z" \
    org.opencontainers.image.source="https://github.com/griffinskudder/updater" \
    org.opencontainers.image.licenses="Apache-2.0" \
    security.scan.enabled="true" \
    security.nonroot="true" \
    security.readonly="true"

# Security: Use exec form for better signal handling
ENTRYPOINT ["/usr/local/bin/updater"]

# Default command: load config from the standard container path.
# Both docker-compose.yml and docker-compose.observability.yml mount their
# respective config files to /app/config.yaml.
CMD ["-config", "/app/config.yaml"]
