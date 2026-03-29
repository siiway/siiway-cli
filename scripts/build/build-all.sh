#!/usr/bin/env sh
set -eu

# Script to build siiway-cli for multiple platforms

# Default values
OUTPUT_DIR="bin"
VERSION="dev"
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Platforms to build for
PLATFORMS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"

# Parse command line arguments
while [ $# -gt 0 ]; do
  case "$1" in
    --output-dir|-o)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --version|-v)
      VERSION="$2"
      shift 2
      ;;
    --platforms|-p)
      PLATFORMS="$2"
      shift 2
      ;;
    --help|-h)
      echo "Usage: $0 [options]"
      echo "Options:"
      echo "  --output-dir, -o   Output directory for binaries (default: bin)"
      echo "  --version, -v      Version string to embed in binary (default: dev)"
      echo "  --platforms, -p    Comma-separated list of platforms (default: linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64)"
      echo "  --help, -h         Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Build for each platform
echo "Building for multiple platforms..."
for platform in $(echo "$PLATFORMS" | tr "," " "); do
  GOOS=$(echo "$platform" | cut -d'/' -f1)
  GOARCH=$(echo "$platform" | cut -d'/' -f2)

  OUTPUT_NAME="siiway-cli-${GOOS}-${GOARCH}"
  if [ "$GOOS" = "windows" ]; then
    OUTPUT_NAME="${OUTPUT_NAME}.exe"
  fi

  echo "Building for $GOOS/$GOARCH..."
  CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build     -ldflags="-X 'github.com/SiiWay/siiway-cli/cmd.Version=$VERSION' -X 'github.com/SiiWay/siiway-cli/cmd.BuildTime=$BUILD_TIME' -X 'github.com/SiiWay/siiway-cli/cmd.GitCommit=$GIT_COMMIT'"     -o "$OUTPUT_DIR/$OUTPUT_NAME"     .
done

echo "Build complete! Binaries are in $OUTPUT_DIR"
