#!/bin/sh
# Builds the Home Assistant add-on image locally for one architecture, the
# same way .github/workflows/addon-image.yml does it.
#
# Usage: sh loxone2hue/build-addon.sh [amd64|aarch64|armv7]
set -e

ARCH="${1:-amd64}"
case "${ARCH}" in
  amd64)   PLATFORM=linux/amd64 ;;
  aarch64) PLATFORM=linux/arm64 ;;
  armv7)   PLATFORM=linux/arm/v7 ;;
  *) echo "Unbekannte Architektur: ${ARCH} (amd64|aarch64|armv7)"; exit 1 ;;
esac

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="$(tr -d '[:space:]' < "${ROOT}/VERSION")"
BUILD_DATE="$(date -u +%Y-%m-%d)"
TAG="loxone2hue-${ARCH}:${VERSION}"

echo "Baue ${TAG} fuer ${PLATFORM} ..."
docker buildx build \
  --platform "${PLATFORM}" \
  --build-arg "BUILD_FROM=ghcr.io/home-assistant/${ARCH}-base:3.22" \
  --build-arg "BUILD_ARCH=${ARCH}" \
  --build-arg "BUILD_VERSION=${VERSION}" \
  --build-arg "BUILD_DATE=${BUILD_DATE}" \
  -f "${ROOT}/loxone2hue/Dockerfile" \
  -t "${TAG}" \
  --load \
  "${ROOT}"

echo "Fertig: ${TAG}"
