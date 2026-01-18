# ModelGate Dockerfile
# Multi-stage build for minimal image size with Web UI included

# =============================================================================
# Stage 1: Build Web UI
# =============================================================================
FROM node:20-alpine AS web-builder

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /app/web

# Copy package files
COPY web/package.json web/pnpm-lock.yaml ./

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy web source code
COPY web/ .

# Build the web UI
RUN pnpm run build

# =============================================================================
# Stage 2: Build Go Backend
# =============================================================================
FROM golang:1.24-alpine AS go-builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy go.mod and go.sum first for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /app/modelgate \
    ./cmd/modelgate

# =============================================================================
# Stage 3: Runtime Image
# =============================================================================
FROM alpine:3.19

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 modelgate && \
    adduser -u 1000 -G modelgate -s /bin/sh -D modelgate

WORKDIR /app

# Copy binary from go-builder
COPY --from=go-builder /app/modelgate /app/modelgate

# Copy migrations for auto-schema
COPY --from=go-builder /app/migrations /app/migrations

# Copy web UI from web-builder
COPY --from=web-builder /app/web/dist /app/web/dist

# Create config directory
RUN mkdir -p /app/config && chown -R modelgate:modelgate /app

# Switch to non-root user
USER modelgate

# Expose unified API port (OpenAI + GraphQL + Web UI + MCP + Metrics all on 8080)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/app/modelgate"]
