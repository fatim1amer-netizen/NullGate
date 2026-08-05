# Vulnerable demo backend — used ONLY behind NullGate to demonstrate blocking.
# This image is intentionally minimal; the app is deliberately insecure and must
# never be exposed directly to a network.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
        -o /out/backend ./examples/vulnerable-backend

FROM scratch
COPY --from=build /out/backend /backend
USER 65534:65534
EXPOSE 3000
ENTRYPOINT ["/backend"]
