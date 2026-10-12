# Home Assistant Add-on Dockerfile for Loxone2HUE Gateway
# Supports multi-architecture builds (aarch64, amd64, armv7)

# Build argument for base image (set by Home Assistant build system)
ARG BUILD_FROM
FROM ${BUILD_FROM:-ghcr.io/home-assistant/aarch64-base:3.18} AS base

#---------------------------------------------
# Build stage for Go backend
FROM golang:1.22-alpine AS go-builder

ARG TARGETARCH

WORKDIR /build

# Copy source from build context
COPY src/ .

RUN apk add --no-cache ca-certificates

# Download dependencies and build
RUN rm -f go.sum && go mod tidy

RUN VERSION=$(cat VERSION) && BUILD_DATE=$(date -u +%Y-%m-%d) && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-arm64} go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_DATE}" \
    -o gateway ./cmd/gateway

#---------------------------------------------
# Build stage for frontend
FROM node:20-alpine AS web-builder

WORKDIR /build

# Copy source from build context
COPY src/web/package*.json ./
RUN npm ci --no-audit --no-fund

COPY src/web/ ./
COPY src/VERSION /build/VERSION

RUN VERSION=$(cat /build/VERSION) && BUILD_DATE=$(date -u +%Y-%m-%d) && \
    VITE_APP_VERSION=${VERSION} VITE_BUILD_DATE=${BUILD_DATE} npm run build

#---------------------------------------------
# Final stage - Home Assistant base image
FROM base

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata

WORKDIR /app

# Copy built artifacts
COPY --from=go-builder /build/gateway ./gateway
COPY --from=web-builder /build/dist ./web/dist

# Copy rootfs (run script)
COPY rootfs /

# Copy example config
COPY src/configs/config.example.yaml ./config.example.yaml

# Create required directories and set permissions
RUN mkdir -p /app/configs /data /config/loxone2hue && \
    chmod +x /run.sh && \
    chmod +x /app/gateway

# Expose port
EXPOSE 8080

# Labels for Home Assistant
LABEL \
    io.hass.name="Loxone2HUE Gateway" \
    io.hass.description="Gateway für Loxone zu Philips HUE Kommunikation" \
    io.hass.type="addon" \
    io.hass.version="1.3.3" \
    io.hass.arch="aarch64|amd64|armv7"

# Start gateway
CMD ["/run.sh"]
