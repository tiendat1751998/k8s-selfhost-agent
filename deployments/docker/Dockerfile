# Build stage
# TODO: Pin to digest for production: golang:1.23-alpine@sha256:<digest>
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /app/bin/server \
    ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /app/bin/agent-runner \
    ./cmd/agent-runner

# Runtime stage
# TODO: Pin to digest for production: gcr.io/distroless/static-debian12:nonroot@sha256:<digest>
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/bin/server /server
COPY --from=builder /app/bin/agent-runner /agent-runner
COPY --from=builder /app/migrations /migrations
COPY --from=builder /app/frontend /frontend

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/server"]
