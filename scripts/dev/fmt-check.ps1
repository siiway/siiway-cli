$ErrorActionPreference = 'Stop'

if (-not (Get-Command gofmt -ErrorAction SilentlyContinue)) {
    Write-Error 'gofmt not found in PATH'
}

$files = @()
if (Get-Command git -ErrorAction SilentlyContinue) {
    try {
        $files = git ls-files '*.go' | Where-Object { $_ -and $_.Trim() -ne '' }
    } catch {
        $files = @()
    }
}

if (-not $files -or $files.Count -eq 0) {
    $files = Get-ChildItem -Path . -Filter *.go -Recurse -File | ForEach-Object { $_.FullName }
}

if (-not $files -or $files.Count -eq 0) {
    Write-Host 'No Go files found.'
    exit 0
}

$unformatted = (& gofmt -l $files)
if ($unformatted -and $unformatted.Count -gt 0) {
    Write-Host 'Go files need formatting. Run one of:'
    Write-Host '  ./scripts/fmt.sh'
    Write-Host '  python3 scripts/fmt.py'
    Write-Host '  pwsh -File scripts/fmt.ps1'
    $unformatted | ForEach-Object { Write-Host $_ }
    exit 1
}

Write-Host 'All Go files are properly formatted.'
