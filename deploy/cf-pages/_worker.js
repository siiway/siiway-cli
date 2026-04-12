const REPO = "SiiWay/siiway-cli";

function unixInstallScript() {
  const lines = [
    "#!/usr/bin/env sh",
    "set -eu",
    "",
    `REPO=\"${REPO}\"`,
    'VERSION="${SIIWAY_VERSION:-latest}"',
    "",
    "OS=\"$(uname -s | tr '[:upper:]' '[:lower:]')\"",
    'case "$OS" in',
    '  linux) GOOS="linux" ;;',
    '  darwin) GOOS="darwin" ;;',
    "  *)",
    '    echo "Unsupported OS: $OS"',
    "    exit 1",
    "    ;;",
    "esac",
    "",
    'ARCH="$(uname -m)"',
    'case "$ARCH" in',
    '  x86_64|amd64) GOARCH="amd64" ;;',
    '  aarch64|arm64) GOARCH="arm64" ;;',
    "  *)",
    '    echo "Unsupported architecture: $ARCH"',
    "    exit 1",
    "    ;;",
    "esac",
    "",
    'ASSET="siiway-cli-$GOOS-$GOARCH"',
    'if [ "$VERSION" = "latest" ]; then',
    '  URL="https://github.com/$REPO/releases/latest/download/$ASSET"',
    "else",
    '  URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET"',
    "fi",
    "",
    'TMP_FILE="$(mktemp)"',
    "cleanup() {",
    '  rm -f "$TMP_FILE"',
    "}",
    "trap cleanup EXIT",
    "",
    "if command -v curl >/dev/null 2>&1; then",
    '  curl -fsSL "$URL" -o "$TMP_FILE"',
    "elif command -v wget >/dev/null 2>&1; then",
    '  wget -qO "$TMP_FILE" "$URL"',
    "else",
    '  echo "curl or wget is required"',
    "  exit 1",
    "fi",
    "",
    'chmod +x "$TMP_FILE"',
    "",
    'INSTALL_DIR="/usr/local/bin"',
    'if [ ! -w "$INSTALL_DIR" ]; then',
    '  INSTALL_DIR="$HOME/.local/bin"',
    '  mkdir -p "$INSTALL_DIR"',
    "fi",
    "",
    'TARGET="$INSTALL_DIR/siiway"',
    'mv "$TMP_FILE" "$TARGET"',
    'chmod +x "$TARGET"',
    'LINK_TARGET="$INSTALL_DIR/sw"',
    'ln -sfn "siiway" "$LINK_TARGET"',
    "",
    'echo "Installed to: $TARGET"',
    'echo "Alias created: $LINK_TARGET -> siiway"',
    'echo "Run: siiway --help or sw --help"',
    "",
    'case ":$PATH:" in',
    '  *":$INSTALL_DIR:"*) ;;',
    "  *)",
    '    echo ""',
    '    echo "Add this directory to PATH if needed:"',
    '    echo "  export PATH=\\"$INSTALL_DIR:\\\$PATH\\""',
    "    ;;",
    "esac",
  ];

  return lines.join("\n") + "\n";
}

function windowsInstallScript() {
  const lines = [
    '$ErrorActionPreference = "Stop"',
    "",
    `$Repo = \"${REPO}\"`,
    '$Version = if ($env:SIIWAY_VERSION) { $env:SIIWAY_VERSION } else { "latest" }',
    "",
    '$Arch = if ($env:PROCESSOR_ARCHITECTURE -match "ARM64") { "arm64" } else { "amd64" }',
    '$Asset = "siiway-cli-windows-$Arch.exe"',
    "",
    'if ($Version -eq "latest") {',
    '  $Url = "https://github.com/$Repo/releases/latest/download/$Asset"',
    "} else {",
    '  $Url = "https://github.com/$Repo/releases/download/$Version/$Asset"',
    "}",
    "",
    '$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\\\\siiway"',
    '$Target = Join-Path $InstallDir "siiway.exe"',
    '$AliasTarget = Join-Path $InstallDir "sw.exe"',
    "",
    "New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null",
    "Invoke-WebRequest -Uri $Url -OutFile $Target",
    "try {",
    "  if (Test-Path $AliasTarget) { Remove-Item -Force $AliasTarget }",
    "  New-Item -ItemType HardLink -Path $AliasTarget -Target $Target | Out-Null",
    "} catch {",
    "  Copy-Item -Path $Target -Destination $AliasTarget -Force",
    "}",
    "",
    '$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")',
    "if ([string]::IsNullOrWhiteSpace($UserPath)) {",
    '  $UserPath = ""',
    "}",
    "",
    '$CurrentPaths = $UserPath -split ";" | ForEach-Object { $_.Trim().TrimEnd("\\\\") } | Where-Object { $_ -ne "" }',
    '$NormalizedInstall = $InstallDir.TrimEnd("\\\\")',
    "",
    "if (-not ($CurrentPaths -contains $NormalizedInstall)) {",
    '  $NewPath = if ([string]::IsNullOrWhiteSpace($UserPath)) { $InstallDir } else { "$UserPath;$InstallDir" }',
    '  [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")',
    '  Write-Host "Added to user PATH: $InstallDir"',
    "}",
    "",
    'Write-Host "Installed to: $Target"',
    'Write-Host "Alias created: $AliasTarget"',
    'Write-Host "Open a new terminal, then run: siiway --help or sw --help"',
  ];

  return lines.join("\n") + "\n";
}

function wantsPowerShell(request) {
  const url = new URL(request.url);
  const shell = (url.searchParams.get("shell") || "").toLowerCase();

  if (shell === "ps1" || shell === "powershell" || shell === "pwsh") {
    return true;
  }
  if (shell === "sh" || shell === "bash") {
    return false;
  }

  const userAgent = (request.headers.get("user-agent") || "").toLowerCase();
  return userAgent.includes("powershell") || userAgent.includes("windows");
}

function renderHomePage() {
  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>siiway-cli installer</title>
  <style>
    body {
      margin: 0;
      font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, sans-serif;
      background: #0f172a;
      color: #e2e8f0;
      padding: 40px 20px;
    }
    main {
      max-width: 860px;
      margin: 0 auto;
      background: #111827;
      border: 1px solid #1f2937;
      border-radius: 12px;
      padding: 24px;
    }
    h1 { margin-top: 0; }
    pre {
      background: #020617;
      border: 1px solid #1e293b;
      border-radius: 8px;
      padding: 12px;
      overflow-x: auto;
    }
    code { color: #93c5fd; }
    a { color: #60a5fa; }
  </style>
</head>
<body>
  <main>
    <h1>siiway-cli install</h1>
    <p>The <code>/cli</code> endpoint auto-detects shell type from request headers.</p>
    <h2>Windows (PowerShell)</h2>
    <pre><code>irm https://sh.wss.moe/cli?shell=ps1 | iex</code></pre>
    <h2>Linux / macOS</h2>
    <pre><code>curl -fsSL https://sh.wss.moe/cli?shell=sh | sh</code></pre>
    <h2>Pin version</h2>
    <pre><code>curl -fsSL https://sh.wss.moe/cli | SIIWAY_VERSION=v1.0.0 sh</code></pre>
    <p>Installer downloads binaries from <a href="https://github.com/SiiWay/siiway-cli/releases">GitHub Releases</a>.</p>
  </main>
</body>
</html>`;
}

export default {
  async fetch(request) {
    const url = new URL(request.url);

    switch (url.pathname) {
      case "/cli" || "/cli.sh" || "/cli.ps1":
        const script = wantsPowerShell(request)
          ? windowsInstallScript()
          : unixInstallScript();
        return new Response(script, {
          headers: {
            "content-type": "text/plain; charset=utf-8",
            "cache-control": "no-store",
          },
        });

      case "/cli.help" || "/help/cli" || "/cli.txt":
        return new Response(renderHomePage(), {
          headers: {
            "content-type": "text/html; charset=utf-8",
          },
        });

      default: {
        const upstreamUrl = new URL(request.url);
        upstreamUrl.protocol = "https:";
        upstreamUrl.host = "sh-wss-moe.pages.dev";
        return fetch(new Request(upstreamUrl.toString(), request));
      }
    }
  },
};
