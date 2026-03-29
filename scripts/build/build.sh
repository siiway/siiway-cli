#!/usr/bin/env sh
set -eu

# Script to build siiway-cli for multiple platforms

# Default values
OUTPUT_DIR="bin"
VERSION="dev"
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

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
    --help|-h)
      echo "Usage: $0 [options]"
      echo "Options:"
      echo "  --output-dir, -o   Output directory for binaries (default: bin)"
      echo "  --version, -v      Version string to embed in binary (default: dev)"
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

# Build for current platform
echo "Building for current platform..."
CGO_ENABLED=0 go build   -ldflags="-X 'github.com/SiiWay/siiway-cli/cmd.Version=$VERSION' -X 'github.com/SiiWay/siiway-cli/cmd.BuildTime=$BUILD_TIME' -X 'github.com/SiiWay/siiway-cli/cmd.GitCommit=$GIT_COMMIT'"   -o "$OUTPUT_DIR/siiway-cli"   .

echo "Build complete: $OUTPUT_DIR/siiway-cli"
