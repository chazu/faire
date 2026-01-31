#!/bin/bash
set -euo pipefail

# Package script for faire
# Usage: ./scripts/package.sh [version]
# Builds binaries for all supported platforms and generates checksums

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

VERSION="${1:-$(git -C "$PROJECT_ROOT" describe --tags --always --dirty 2>/dev/null || echo "dev")}"
COMMIT="$(git -C "$PROJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo "unknown")"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
BINARY_NAME="faire"
MAIN_PATH="./cmd/gitsavgy"
DIST_DIR="$PROJECT_ROOT/dist"

PLATFORMS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
)

echo "Packaging ${BINARY_NAME} ${VERSION}..."
echo "Commit: ${COMMIT}"
echo "Build date: ${BUILD_DATE}"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

for platform in "${PLATFORMS[@]}"; do
  IFS='/' read -r GOOS GOARCH <<< "$platform"
  output_name="${BINARY_NAME}-${VERSION}-${GOOS}-${GOARCH}"

  echo "Building ${GOOS}/${GOARCH}..."

  GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build \
    -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Date=${BUILD_DATE}" \
    -o "$DIST_DIR/${output_name}" \
    "$MAIN_PATH"
done

echo "Generating checksums..."
cd "$DIST_DIR"
for file in ${BINARY_NAME}-${VERSION}-*; do
  if [ -f "$file" ]; then
    shasum -a 256 "$file" >> "${BINARY_NAME}-${VERSION}-sha256.txt"
  fi
done
cd "$PROJECT_ROOT"

echo "Packaging complete!"
echo "Artifacts in ${DIST_DIR}:"
ls -la "$DIST_DIR"
