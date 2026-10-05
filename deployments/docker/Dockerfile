# ==============================================================================
# Stage 1: Build Frontend Assets (Vue 3 / TypeScript SPA)
# ==============================================================================
FROM node:22-alpine AS frontend-builder

WORKDIR /app/frontend-vue

# Cache dependencies
COPY frontend-vue/package.json frontend-vue/package-lock.json ./
RUN npm ci --prefer-offline --no-audit

# Build production SPA assets into /app/frontend-vue/dist
COPY frontend-vue/ ./
RUN npm run build

# ==============================================================================
# Stage 2: Build Go Backend & Static Probe Binaries
# ==============================================================================
FROM golang:1.26.6-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata tini-static

WORKDIR /app

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy backend source
COPY . .

# Build statically linked binaries with stripped debug symbols
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -extldflags '-static'" \
    -o /app/bin/server \
    ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -extldflags '-static'" \
    -o /app/bin/probe \
    ./cmd/probe

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -extldflags '-static'" \
    -o /app/bin/agent-runner \
    ./cmd/agent-runner

# ==============================================================================
# Stage 3: Minimal, Hardened Runtime Image
# ==============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

# TLS certificates and timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Tini init system for PID 1 signal forwarding & zombie reaping
COPY --from=builder /sbin/tini-static /sbin/tini

# Compiled binaries
COPY --from=builder /app/bin/server /server
COPY --from=builder /app/bin/server /app/server
COPY --from=builder /app/bin/probe /probe
COPY --from=builder /app/bin/probe /app/probe
COPY --from=builder /app/bin/agent-runner /agent-runner
COPY --from=builder /app/migrations /migrations

# Compiled frontend SPA assets from Stage 1
COPY --from=frontend-builder /app/frontend-vue/dist /frontend
COPY --from=frontend-builder /app/frontend-vue/dist /app/frontend

# Environment configuration
ENV K8S_SERVER_HOST="0.0.0.0" \
    K8S_SERVER_PORT="8080" \
    FRONTEND_DIR="/app/frontend"

# Unprivileged port binding
EXPOSE 8080

# Enforce explicit non-root execution (UID:GID 10001:10001)
USER 10001:10001

# Healthcheck targeting /healthz with static health probe binary
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/probe", "http://localhost:8080/healthz"]

# Tini entrypoint for graceful shutdown signal propagation (SIGTERM/SIGINT)
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/server"]
