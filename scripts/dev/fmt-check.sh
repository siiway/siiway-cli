#!/usr/bin/env sh
set -eu

if ! command -v gofmt >/dev/null 2>&1; then
  echo "gofmt not found in PATH"
  exit 1
fi

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

unformatted=$(echo "$files" | xargs gofmt -l)

if [ -n "$unformatted" ]; then
  echo "Go files need formatting. Run one of:"
  echo "  ./scripts/fmt.sh"
  echo "  python3 scripts/fmt.py"
  echo "  pwsh -File scripts/fmt.ps1"
  echo "$unformatted"
  exit 1
fi

echo "All Go files are properly formatted."
