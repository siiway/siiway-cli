# Cloudflare Pages (Workers) Installer

This directory contains the Cloudflare Pages (Workers) worker entry used to serve one-line install scripts.

## Endpoints

- `/cli` : auto-detected install script
- `/cli.help` : installer instructions page
- Any other path: proxied to `https://sh-wss-moe.pages.dev` with the same path and query string

## Behavior

- If request appears to come from PowerShell/Windows, `/cli` returns a PowerShell installer.
- Otherwise `/cli` returns a POSIX shell installer for Linux/macOS.
- `shell` query parameter can override auto-detection:
	- PowerShell: `shell=ps1`, `shell=powershell`, `shell=pwsh`
	- POSIX shell: `shell=sh`, `shell=bash`
- Installers download binaries from GitHub Releases.

## Required Release Asset Naming

- `siiway-cli-linux-amd64`
- `siiway-cli-linux-arm64`
- `siiway-cli-darwin-amd64`
- `siiway-cli-darwin-arm64`
- `siiway-cli-windows-amd64.exe`
- `siiway-cli-windows-arm64.exe`
