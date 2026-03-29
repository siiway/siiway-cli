# Script to build siiway-cli for multiple platforms

# Default values
$OutputDir = "bin"
$Version = "dev"
$BuildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$GitCommit = if (Get-Command git -ErrorAction SilentlyContinue) { git rev-parse --short HEAD } else { "unknown" }

# Platforms to build for
$Platforms = @(
    @{GOOS="linux"; GOARCH="amd64"},
    @{GOOS="linux"; GOARCH="arm64"},
    @{GOOS="darwin"; GOARCH="amd64"},
    @{GOOS="darwin"; GOARCH="arm64"},
    @{GOOS="windows"; GOARCH="amd64"}
)

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
    Write-Host "Usage: .uild-all.ps1 [options]"
    Write-Host "Options:"
    Write-Host "  --output-dir, -o   Output directory for binaries (default: bin)"
    Write-Host "  --version, -v      Version string to embed in binary (default: dev)"
    Write-Host "  --help, -h         Show this help message"
}

# Create output directory if it doesn't exist
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

# Build for each platform
Write-Host "Building for multiple platforms..."
foreach ($Platform in $Platforms) {
    $GOOS = $Platform.GOOS
    $GOARCH = $Platform.GOARCH

    $OutputName = "siiway-cli-${GOOS}-${GOARCH}"
    if ($GOOS -eq "windows") {
        $OutputName = "${OutputName}.exe"
    }

    Write-Host "Building for $GOOS/$GOARCH..."
    $env:CGO_ENABLED = "0"
    $env:GOOS = $GOOS
    $env:GOARCH = $GOARCH

    go build `
        -ldflags="-X 'github.com/SiiWay/siiway-cli/cmd.Version=$Version' -X 'github.com/SiiWay/siiway-cli/cmd.BuildTime=$BuildTime' -X 'github.com/SiiWay/siiway-cli/cmd.GitCommit=$GitCommit'" `
        -o "$OutputDir\$OutputName" `
        .
}

Write-Host "Build complete! Binaries are in $OutputDir"
