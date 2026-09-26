# ── Stage 1: Build ──────────────────────────────
FROM golang:1.27-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with optimizations
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w \
      -X github.com/ovander/backendkit/buildinfo.Version=${VERSION} \
      -X github.com/ovander/backendkit/buildinfo.BuildTime=${BUILD_TIME} \
      -X github.com/ovander/backendkit/buildinfo.GitCommit=${GIT_COMMIT}" \
    -o /bin/kerplan-api ./cmd/server

# ── Stage 2: Runtime ───────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

# Non-root user
RUN addgroup -S kerplan && adduser -S kerplan -G kerplan
USER kerplan

WORKDIR /app

# Copy binary and migrations
COPY --from=builder /bin/kerplan-api /app/kerplan-api
COPY --from=builder /app/migrations /app/migrations

# Default environment
ENV APP_ENV=production
ENV PORT=8080

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/kerplan-api"]
