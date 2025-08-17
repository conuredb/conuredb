# syntax=docker/dockerfile:1

## Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /src

# Enable CGO for raft compatibility. If CGO_ENABLED=0, Go creates a completely static binary with no C dependencies.
# The github.com/hashicorp/raft library has some low-level networking and file I/O operations. On certain platforms
# (like ARM64), the pure Go networking stack can have compatibility issues with container networking.
ENV CGO_ENABLED=1

# Install build dependencies for CGO
RUN apk add --no-cache gcc musl-dev
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source and build
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    go build -ldflags "-s -w" -o /out/conure-db ./cmd/conure-db

## Runtime stage
FROM alpine:3.19

# Install essential runtime dependencies
RUN apk add --no-cache ca-certificates curl \
    && adduser -D -u 1000 conure \
    && mkdir -p /var/lib/conure \
    && chown conure:conure /var/lib/conure

# Copy binary
COPY --from=builder /out/conure-db /bin/conure-db

# Switch to non-root user
USER conure
WORKDIR /var/lib/conure

# Expose ports
EXPOSE 8081 7001

ENTRYPOINT ["/bin/conure-db"]
