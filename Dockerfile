# syntax=docker/dockerfile:1

## Build stage
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder
WORKDIR /src

# Target platform args provided automatically by buildx
ARG TARGETOS
ARG TARGETARCH

# Cache modules
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source and build for the target platform
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/conure-db ./cmd/conure-db && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/conuresh ./cmd/repl

## Runtime stage
FROM alpine:3.19

# Install essential runtime dependencies
RUN apk add --no-cache ca-certificates curl \
    && adduser -D -u 1000 conure \
    && mkdir -p /var/lib/conure \
    && chown conure:conure /var/lib/conure

# Copy binaries from builder
COPY --from=builder /out/conure-db /bin/conure-db
COPY --from=builder /out/conuresh /bin/conuresh

# Switch to non-root user
USER conure
WORKDIR /var/lib/conure

# Expose ports
EXPOSE 8081 7001

ENTRYPOINT ["/bin/conure-db"]
