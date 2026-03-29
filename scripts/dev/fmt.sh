#!/usr/bin/env sh
set -eu

if ! command -v gofmt >/dev/null 2>&1; then
  echo "gofmt not found in PATH"
  exit 1
fi

# Use git-tracked Go files when available; fallback to filesystem discovery.
files=""
if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  files=$(git ls-files '*.go')
fi

if [ -z "$files" ]; then
  files=$(find . -type f -name '*.go')
fi

if [ -z "$files" ]; then
  echo "No Go files found."
  exit 0
fi

echo "$files" | xargs gofmt -w
echo "Formatting complete."
