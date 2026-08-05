# NullGate WAF — Production Dockerfile
# Multi-stage build for a minimal, secure container

# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go.mod and go.sum first to cache dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=2.0.0" \
    -o nullgate \
    cmd/nullgate/main.go

# Stage 2: Minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -g 1000 nullgate && \
    adduser -D -u 1000 -G nullgate nullgate

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/nullgate /app/nullgate

# Create directories
RUN mkdir -p /var/lib/nullgate/autocert && \
    mkdir -p /var/log/nullgate && \
    chown -R nullgate:nullgate /var/lib/nullgate /var/log/nullgate

# Drop privileges
USER nullgate

# Expose ports
EXPOSE 80 443

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:80/healthz || exit 1

# Entrypoint
ENTRYPOINT ["/app/nullgate"]
CMD ["--config", "/etc/nullgate/config.yaml"]
