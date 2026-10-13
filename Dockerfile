# Build stage for Go backend
FROM golang:1.27-alpine AS go-builder

WORKDIR /app

# Dependencies first, pinned by go.sum, so a rebuild cannot pick up other
# module versions and the layer is cached across source changes.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY cmd ./cmd
COPY internal ./internal
COPY VERSION ./VERSION

RUN VERSION=$(tr -d '[:space:]' < VERSION) && BUILD_DATE=$(date -u +%Y-%m-%d) && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_DATE}" \
    -o gateway ./cmd/gateway

# Build stage for React frontend
FROM node:22-alpine AS web-builder

WORKDIR /app/web

# Copy package files and install the locked dependency tree
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund

# Copy source code
COPY web/ ./
# Release notes are bundled from the add-on changelog (see ReleaseNotes.tsx)
COPY loxone2hue/CHANGELOG.md /app/loxone2hue/CHANGELOG.md

# Copy VERSION file for build-time injection
COPY VERSION /tmp/VERSION

# Build the frontend with version info
RUN export VITE_APP_VERSION=$(cat /tmp/VERSION | tr -d '[:space:]') && \
    export VITE_BUILD_DATE=$(date -u +%Y-%m-%d) && \
    npm run build

# Final stage
FROM alpine:3.22

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from go-builder
COPY --from=go-builder /app/gateway .

# Copy frontend build from web-builder
COPY --from=web-builder /app/web/dist ./web/dist

# Copy example config
COPY configs/config.example.yaml ./config.example.yaml

# Create config directory and drop root.
#
# UID/GID 1000 is the usual first non-root account on Linux, so a bind-mounted
# ./configs owned by the host user stays writable. If the gateway cannot write
# its configuration after an update, adjust the host directory:
#   sudo chown -R 1000:1000 ./configs
RUN mkdir -p /app/configs \
    && addgroup -g 1000 loxone2hue \
    && adduser -D -u 1000 -G loxone2hue loxone2hue \
    && chown -R loxone2hue:loxone2hue /app

USER loxone2hue

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# Run the binary
ENTRYPOINT ["./gateway"]
CMD ["-config", "/app/configs/config.yaml"]
