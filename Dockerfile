# ── NullGate WAF — multi-stage build ─────────────────────────────────────────
# Stage 1 builds a fully static binary; stage 2 ships it on a distroless-style
# scratch image so the runtime attack surface is a single ~11 MB executable.

FROM golang:1.22-alpine AS build

# Build tooling and CA certs (copied into the final image for outbound TLS).
RUN apk add --no-cache ca-certificates git && update-ca-certificates

WORKDIR /src

# Cache dependencies separately from source for fast incremental builds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO disabled → static binary that runs on scratch. Trim debug info to shrink.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/nullgate ./cmd/nullgate

# ── Stage 2: minimal runtime ────────────────────────────────────────────────
FROM scratch AS runtime

# CA roots so the proxy can dial HTTPS backends.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/nullgate /nullgate
COPY configs/config.yaml /configs/config.yaml

# Run as an unprivileged, non-root UID. scratch has no /etc/passwd, so we use a
# numeric UID that maps to nobody on most hosts.
USER 65534:65534

EXPOSE 8080 9090 9091

ENTRYPOINT ["/nullgate"]
CMD ["--config", "/configs/config.yaml"]
