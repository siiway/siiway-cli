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

& gofmt -w $files
Write-Host 'Formatting complete.'
