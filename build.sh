#!/bin/bash

# OpenTrail Build Script
# Builds the application for multiple platforms with embedded static assets

set -e

# Configuration
APP_NAME="opentrail"
VERSION=${VERSION:-"1.0.0"}
BUILD_DIR="build"
CMD_PATH="./cmd/opentrail"

# Build information
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go build flags
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

# Supported platforms
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/arm64"
)

echo "Building OpenTrail v${VERSION}"
echo "Build time: ${BUILD_TIME}"
echo "Git commit: ${GIT_COMMIT}"
echo ""

# Clean build directory
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}

# Build for each platform
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r -a platform_split <<< "$platform"
    GOOS="${platform_split[0]}"
    GOARCH="${platform_split[1]}"
    
    output_name="${APP_NAME}-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    echo "Building for ${GOOS}/${GOARCH}..."
    
    env GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=1 go build \
        -ldflags="$LDFLAGS" \
        -o "${BUILD_DIR}/${output_name}" \
        "$CMD_PATH"
    
    if [ $? -ne 0 ]; then
        echo "Error building for ${GOOS}/${GOARCH}"
        exit 1
    fi
    
    echo "✓ Built ${BUILD_DIR}/${output_name}"
done

echo ""
echo "Build completed successfully!"
echo "Binaries available in ${BUILD_DIR}/"
ls -la ${BUILD_DIR}/

# Create checksums
echo ""
echo "Generating checksums..."
cd ${BUILD_DIR}
sha256sum * > checksums.txt
echo "✓ Checksums saved to ${BUILD_DIR}/checksums.txt"
cd ..

echo ""
echo "Build summary:"
echo "- Version: ${VERSION}"
echo "- Build time: ${BUILD_TIME}"
echo "- Git commit: ${GIT_COMMIT}"
echo "- Platforms: ${#PLATFORMS[@]}"
echo "- Output directory: ${BUILD_DIR}/"