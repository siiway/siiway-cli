# Cloudflare Pages Installer

This directory contains the Cloudflare Pages worker entry used to serve one-line install scripts.

## Endpoints

- `/` : install page
- `/get` : auto-detected install script

## Behavior

- If request appears to come from PowerShell/Windows, `/get` returns a PowerShell installer.
- Otherwise `/get` returns a POSIX shell installer for Linux/macOS.
- Installers download binaries from GitHub Releases.

## Required Release Asset Naming

- `siiway-cli-linux-amd64`
- `siiway-cli-linux-arm64`
- `siiway-cli-darwin-amd64`
- `siiway-cli-darwin-arm64`
- `siiway-cli-windows-amd64.exe`
- `siiway-cli-windows-arm64.exe`
