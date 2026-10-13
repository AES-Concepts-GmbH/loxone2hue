# Standalone Dockerfile for Synology NAS / Generic Docker
# Does not depend on Home Assistant base images

# Version args (passed by build.sh or docker-compose)
ARG VERSION=dev
ARG BUILD_DATE=unknown

#---------------------------------------------
# Build stage for Go backend
FROM golang:1.27-alpine AS go-builder

ARG VERSION
ARG BUILD_DATE
ARG TARGETARCH=amd64

WORKDIR /build
RUN apk add --no-cache git ca-certificates

# Dependencies first, pinned by go.sum
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY cmd ./cmd
COPY internal ./internal

# Build for target architecture with version info
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_DATE}" \
    -o gateway ./cmd/gateway

#---------------------------------------------
# Build stage for frontend
FROM node:22-alpine AS web-builder

ARG VERSION
ARG BUILD_DATE

WORKDIR /build

# Copy web source
COPY web/package*.json ./
RUN npm ci --no-audit --no-fund

COPY web/ ./
# Release notes are bundled from the add-on changelog (see ReleaseNotes.tsx)
COPY loxone2hue/CHANGELOG.md /loxone2hue/CHANGELOG.md

# Build with version info
ENV VITE_APP_VERSION=${VERSION}
ENV VITE_BUILD_DATE=${BUILD_DATE}
RUN npm run build

#---------------------------------------------
# Final stage
FROM alpine:3.22

ARG VERSION
ARG BUILD_DATE

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata

WORKDIR /app

# Copy built artifacts
COPY --from=go-builder /build/gateway ./gateway
COPY --from=web-builder /build/dist ./web/dist

# Copy example config
COPY configs/config.example.yaml ./config.example.yaml

# Create config directory
RUN mkdir -p /data && chmod +x /app/gateway

# Create startup script
RUN echo '#!/bin/sh' > /start.sh && \
    echo 'CONFIG_PATH=/data/config.yaml' >> /start.sh && \
    echo '' >> /start.sh && \
    echo 'if [ -f "$CONFIG_PATH" ]; then' >> /start.sh && \
    echo '  echo "Existing config found, preserving: $CONFIG_PATH"' >> /start.sh && \
    echo 'else' >> /start.sh && \
    echo '  echo "No config found, creating default: $CONFIG_PATH"' >> /start.sh && \
    echo '  cp /app/config.example.yaml $CONFIG_PATH' >> /start.sh && \
    echo 'fi' >> /start.sh && \
    echo '' >> /start.sh && \
    echo 'exec /app/gateway -config $CONFIG_PATH' >> /start.sh && \
    chmod +x /start.sh

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# Labels
LABEL \
    version="${VERSION}" \
    build_date="${BUILD_DATE}" \
    description="Loxone2HUE Gateway for Synology NAS"

# Start gateway
CMD ["/start.sh"]
