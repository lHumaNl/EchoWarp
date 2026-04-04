# Multi-platform Dockerfile for EchoWarp
# Supports: linux/amd64, linux/arm64, linux/arm/v7
# 
# Build command:
#   docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t echowarp:latest .
#
# For single platform:
#   docker buildx build --platform linux/amd64 -t echowarp:latest .
#   docker build -t echowarp:latest .  # backward compatible

# Build stage
FROM golang:1.22-alpine AS builder

# Build arguments for multi-platform support (automatically set by Docker BuildKit)
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies with cache mount for faster rebuilds
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Build the binary with platform-specific settings and cache mount
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH} \
    GOARM=$(echo ${TARGETVARIANT} | sed 's/v//') \
    go build -ldflags="-s -w" -o echowarp ./cmd/echowarp

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/echowarp .

# Expose application ports
EXPOSE 4415 8080

# Healthcheck to verify the application is running and responsive
# Uses /healthz endpoint on port 8080 for container health monitoring
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Run the application
ENTRYPOINT ["./echowarp"]
