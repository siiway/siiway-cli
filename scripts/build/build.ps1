# Script to build siiway-cli for current platform

# Default values
$OutputDir = "bin"
$Version = "dev"
$BuildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$GitCommit = if (Get-Command git -ErrorAction SilentlyContinue) { git rev-parse --short HEAD } else { "unknown" }

# Parse command line arguments
while ($args.Count -gt 0) {
    switch ($args[0]) {
        "--output-dir" { $OutputDir = $args[1]; $args = $args[2..$args.Count] }
        "-o" { $OutputDir = $args[1]; $args = $args[2..$args.Count] }
        "--version" { $Version = $args[1]; $args = $args[2..$args.Count] }
        "-v" { $Version = $args[1]; $args = $args[2..$args.Count] }
        "--help" { Show-Help; exit 0 }
        "-h" { Show-Help; exit 0 }
        default { Write-Error "Unknown option: $($args[0])"; exit 1 }
    }
}

function Show-Help {
    Write-Host "Usage: .uild.ps1 [options]"
    Write-Host "Options:"
    Write-Host "  --output-dir, -o   Output directory for binaries (default: bin)"
    Write-Host "  --version, -v      Version string to embed in binary (default: dev)"
    Write-Host "  --help, -h         Show this help message"
}

# Create output directory if it doesn't exist
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

# Build for current platform
Write-Host "Building for current platform..."
$env:CGO_ENABLED = "0"
go build `
    -ldflags="-X 'github.com/SiiWay/siiway-cli/cmd.Version=$Version' -X 'github.com/SiiWay/siiway-cli/cmd.BuildTime=$BuildTime' -X 'github.com/SiiWay/siiway-cli/cmd.GitCommit=$GitCommit'" `
    -o "$OutputDir\siiway-cli.exe" `
    .

Write-Host "Build complete: $OutputDir\siiway-cli.exe"
