#!/bin/bash
set -euo pipefail

# Build script for faire
# Usage: ./scripts/build.sh [version]
# If no version is specified, uses git describe

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

VERSION="${1:-$(git -C "$PROJECT_ROOT" describe --tags --always --dirty 2>/dev/null || echo "dev")}"
COMMIT="$(git -C "$PROJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo "unknown")"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
BINARY_NAME="faire"
MAIN_PATH="./cmd/gitsavgy"
BUILD_DIR="$PROJECT_ROOT/bin"

echo "Building ${BINARY_NAME}..."
echo "Version: ${VERSION}"
echo "Commit: ${COMMIT}"
echo "Build date: ${BUILD_DATE}"

mkdir -p "$BUILD_DIR"

go build \
  -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Date=${BUILD_DATE}" \
  -o "$BUILD_DIR/${BINARY_NAME}" \
  "$MAIN_PATH"

echo "Built $BUILD_DIR/${BINARY_NAME}"
