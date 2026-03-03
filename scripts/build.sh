#!/bin/bash

# Build script for Anyfeed
# Cross-compiles for Linux, macOS, and Windows

set -e

VERSION=${VERSION:-"1.0.0"}
BUILD_DIR="./build"
CMD_DIR="./cmd/anyfeed"
BINARY_NAME="anyfeed"

# Clean previous builds
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

echo "Building Anyfeed v${VERSION}..."

# Build for multiple platforms
platforms=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

for platform in "${platforms[@]}"; do
    GOOS=${platform%/*}
    GOARCH=${platform#*/}
    
    output_name="${BUILD_DIR}/${BINARY_NAME}-${GOOS}-${GOARCH}"
    if [ "${GOOS}" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    echo "Building for ${GOOS}/${GOARCH}..."
    
    GOOS=${GOOS} GOARCH=${GOARCH} go build \
        -ldflags="-s -w -X main.Version=${VERSION}" \
        -o "${output_name}" \
        "${CMD_DIR}"
done

echo ""
echo "Build complete! Binaries are in ${BUILD_DIR}/"
ls -la "${BUILD_DIR}/"
