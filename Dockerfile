# syntax=docker/dockerfile:1

# ------------------------------------------------------------------------------
# Build Stage
# ------------------------------------------------------------------------------
FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=1.0.0" \
    -o /bin/manus-mcp-gateway \
    ./cmd/manus-mcp-gateway

# ------------------------------------------------------------------------------
# Final Runtime Stage (Zero Attack Surface)
# ------------------------------------------------------------------------------
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /bin/manus-mcp-gateway /manus-mcp-gateway

# FastMCP stdio interface
ENTRYPOINT ["/manus-mcp-gateway"]
