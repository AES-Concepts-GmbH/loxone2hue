# Home Assistant Add-on image for the Loxone2HUE Gateway.
#
# Built by .github/workflows/addon-image.yml with the repository root as the
# build context, once per architecture, and published to GHCR. Home Assistant
# pulls the finished image (config.yaml: image) and no longer builds on the
# host, so the source tree is not copied into this directory any more.
#
# Local build for one architecture: sh loxone2hue/build-addon.sh [amd64|aarch64|armv7]
ARG BUILD_FROM=ghcr.io/home-assistant/amd64-base:3.22

#---------------------------------------------
# Go backend. Cross-compiled on the build host, so this stage runs natively
# even when the target is arm.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS go-builder

ARG TARGETARCH
ARG TARGETVARIANT
ARG BUILD_VERSION=dev
ARG BUILD_DATE=unknown

WORKDIR /build

# Dependencies first, pinned by go.sum. Regenerating go.sum inside the build
# (the previous `rm -f go.sum && go mod tidy`) let every build pick whatever
# module versions were current at the time.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY cmd ./cmd
COPY internal ./internal

RUN GOARM="${TARGETVARIANT#v}" CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH}" go build \
    -trimpath \
    -ldflags="-w -s -X main.version=${BUILD_VERSION} -X main.buildTime=${BUILD_DATE}" \
    -o gateway ./cmd/gateway

#---------------------------------------------
# Web UI. Architecture independent, also built natively.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web-builder

ARG BUILD_VERSION=dev
ARG BUILD_DATE=unknown

WORKDIR /build

COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY web/ ./
RUN VITE_APP_VERSION="${BUILD_VERSION}" VITE_BUILD_DATE="${BUILD_DATE}" npm run build

#---------------------------------------------
# Final stage on the Home Assistant base image of the target architecture
FROM ${BUILD_FROM}

ARG BUILD_VERSION=dev
ARG BUILD_ARCH=amd64

RUN apk add --no-cache \
    ca-certificates \
    tzdata

WORKDIR /app

COPY --from=go-builder /build/gateway ./gateway
COPY --from=web-builder /build/dist ./web/dist
COPY loxone2hue/rootfs /
COPY configs/config.example.yaml ./config.example.yaml

RUN mkdir -p /app/configs /data /config/loxone2hue && \
    chmod +x /run.sh /app/gateway

EXPOSE 8080

LABEL \
    io.hass.name="Loxone2HUE Gateway" \
    io.hass.description="Gateway für Loxone zu Philips HUE Kommunikation" \
    io.hass.type="addon" \
    io.hass.version="${BUILD_VERSION}" \
    io.hass.arch="${BUILD_ARCH}"

CMD ["/run.sh"]
