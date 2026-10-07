# =============================================================================
# PRODUCTION DOCKERFILE
# Multi-stage build for minimal final image
# =============================================================================

# The two build stages run on the machine doing the build ($BUILDPLATFORM), not
# under emulation for each target: the frontend bundle is the same for every
# architecture, and Go cross-compiles. Under QEMU, npm ci for arm64 could hang
# for the better part of an hour.

# Stage 1: Build frontend
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-builder

WORKDIR /app/web

# Install dependencies first (cache layer)
COPY web/package*.json ./
RUN npm ci --production=false

# Build frontend
COPY web/ ./
RUN npm run build

# Stage 2: Build backend
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS backend-builder

# Set by buildx for each image it builds; a plain docker build fills them in
# for the machine it runs on.
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Install dependencies first (cache layer)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Copy built frontend for embedding
COPY --from=frontend-builder /app/web/dist ./web/dist

# Build optimized binary for the image's architecture. This was fixed to amd64,
# which put an x86-64 binary into the arm64 image as well.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags="-w -s -X main.Version=$(date +%Y%m%d)" \
    -o /jfshare ./cmd/server

# Stage 3: Final minimal image
FROM alpine:3.20

LABEL org.opencontainers.image.title="JFShare"
LABEL org.opencontainers.image.description="Jellyfin one-time share link system"
LABEL org.opencontainers.image.source="https://github.com/jellyfin-share/jellyfin-share-backend"

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /jfshare .

# Copy migrations for embedded FS
COPY --from=backend-builder /app/migrations ./migrations

# Copy frontend dist for embedded FS
COPY --from=frontend-builder /app/web/dist ./web/dist

# Create non-root user
RUN addgroup -g 1000 jfshare && \
    adduser -D -u 1000 -G jfshare jfshare && \
    chown -R jfshare:jfshare /app

USER jfshare

EXPOSE 8097

# Follow JFSHARE_PORT rather than hard-coding it: a fixed port here left the
# container permanently unhealthy whenever the port was overridden.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider "http://localhost:${JFSHARE_PORT:-8097}/health" || exit 1

ENTRYPOINT ["/app/jfshare"]
