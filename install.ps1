# mgm CLI installer for Windows PowerShell.
#
# One-liner:
#   irm https://raw.githubusercontent.com/MGM-Laboratory/mgm-cli/main/install.ps1 | iex
#
# Pin a version:
#   $env:MGM_VERSION='v0.1.0'; irm https://raw.githubusercontent.com/MGM-Laboratory/mgm-cli/main/install.ps1 | iex
#
# Custom install dir:
#   $env:MGM_INSTALL_DIR="$HOME\bin"; irm .../install.ps1 | iex

[CmdletBinding()]
param(
    [string]$Version    = $env:MGM_VERSION,
    [string]$Repo       = $(if ($env:MGM_REPO)        { $env:MGM_REPO }        else { 'MGM-Laboratory/mgm-cli' }),
    [string]$InstallDir = $env:MGM_INSTALL_DIR
)

$ErrorActionPreference = 'Stop'

function Write-Info($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "OK  $msg" -ForegroundColor Green }
function Write-Err($msg)  { Write-Host "ERR $msg" -ForegroundColor Red }

if (-not $Version) { $Version = 'latest' }

# --- arch detect ---
# Use PROCESSOR_ARCHITEW6432 first (set when a 32-bit process runs on a 64-bit OS),
# then PROCESSOR_ARCHITECTURE, then fall back to RuntimeInformation. We avoid
# switching directly on RuntimeInformation.OSArchitecture: in PowerShell 5.1 it
# returns an enum value that does not compare equal to the case-string literals,
# so every case misses and `default` fires with an empty `$_`.
$archRaw = $env:PROCESSOR_ARCHITEW6432
if (-not $archRaw) { $archRaw = $env:PROCESSOR_ARCHITECTURE }
if (-not $archRaw) {
    try {
        $archRaw = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    } catch {
        $archRaw = ''
    }
}
$arch = switch -Regex ($archRaw) {
    '^(AMD64|X64)$' { 'amd64'; break }
    '^ARM64$'       { 'arm64'; break }
    default         { throw "unsupported architecture: '$archRaw' (expected AMD64 / X64 / ARM64)" }
}
if ($arch -eq 'arm64') {
    throw "windows/arm64 builds aren't published yet -- use the amd64 build under emulation"
}

$asset = "mgm-windows-$arch.zip"
$url = if ($Version -eq 'latest') {
    "https://github.com/$Repo/releases/latest/download/$asset"
} else {
    "https://github.com/$Repo/releases/download/$Version/$asset"
}

# --- install dir ---
if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\mgm'
}
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# --- download + extract ---
$tmp = New-Item -ItemType Directory -Force -Path (Join-Path $env:TEMP "mgm-install-$([System.Guid]::NewGuid())")
try {
    Write-Info "Detected windows/$arch"
    Write-Info "Downloading $url"

    $zipPath = Join-Path $tmp.FullName $asset
    try {
        Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing
    } catch {
        Write-Err "download failed: $url"
        throw
    }

    Expand-Archive -Path $zipPath -DestinationPath $tmp.FullName -Force
    $src = Join-Path $tmp.FullName 'mgm.exe'
    if (-not (Test-Path $src)) { throw "archive did not contain mgm.exe" }

    $target = Join-Path $InstallDir 'mgm.exe'
    Copy-Item -Path $src -Destination $target -Force
    Write-Ok "installed: $target"
} finally {
    Remove-Item -Recurse -Force $tmp.FullName -ErrorAction SilentlyContinue
}

# --- PATH update (user scope, persistent) ---
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($null -eq $userPath) { $userPath = '' }
$alreadyOnPath = ($userPath -split ';') -contains $InstallDir
if (-not $alreadyOnPath) {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$InstallDir", 'User')
    Write-Ok "added $InstallDir to user PATH (open a new shell to pick it up)"
} else {
    Write-Info "$InstallDir already on PATH"
}
# also update the current session
if (($env:Path -split ';') -notcontains $InstallDir) {
    $env:Path = "$env:Path;$InstallDir"
}

# --- post-install hint ---
try {
    & $target version 2>$null | Select-Object -First 1
} catch {}
Write-Host ""
Write-Host "Next: " -NoNewline
Write-Host "mgm env configure" -ForegroundColor Yellow -NoNewline
Write-Host " to set Infisical credentials."
