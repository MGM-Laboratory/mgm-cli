# Megumi Code / mgm CLI installer for Windows PowerShell.
#
# Served from the marketing site (https://cli.labmgm.org); pulls the matching
# binary from GitHub Releases.
#
# One-liner:
#   irm https://cli.labmgm.org/install.ps1 | iex
#
# Pin a version:
#   $env:MGM_VERSION='v0.1.0'; irm https://cli.labmgm.org/install.ps1 | iex
#
# Custom install dir:
#   $env:MGM_INSTALL_DIR="$HOME\bin"; irm https://cli.labmgm.org/install.ps1 | iex
#
# Knobs: MGM_REPO (default MGM-Laboratory/mgm-cli), MGM_VERSION (default latest),
# MGM_INSTALL_DIR.

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

    # Pre-flight: HEAD the URL so we can tell the user clearly when there is
    # no release yet, instead of a generic 404 from the actual download.
    $statusCode = 0
    try {
        $head = Invoke-WebRequest -Uri $url -Method Head -UseBasicParsing -MaximumRedirection 5 -ErrorAction Stop
        $statusCode = [int]$head.StatusCode
    } catch [System.Net.WebException] {
        if ($_.Exception.Response) {
            $statusCode = [int]$_.Exception.Response.StatusCode
        } else {
            Write-Err "could not reach github.com - check your network/proxy"
            throw
        }
    } catch {
        Write-Err "preflight failed: $($_.Exception.Message)"
        throw
    }

    if ($statusCode -eq 404) {
        if ($Version -eq 'latest') {
            Write-Err "no release published yet for $Repo."
            Write-Host "    Visit https://github.com/$Repo/releases - once a release exists, re-run this command."
            Write-Host "    To install a specific version: `$env:MGM_VERSION='v0.1.0'; irm .../install.ps1 | iex"
            throw "no release available"
        } else {
            Write-Err "release $Version not found for $Repo."
            Write-Host "    See https://github.com/$Repo/releases for available versions."
            throw "release not found"
        }
    } elseif ($statusCode -ne 200 -and $statusCode -ne 0) {
        Write-Err "unexpected HTTP $statusCode from $url"
        throw "unexpected status"
    }

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
Write-Host "mgm auth" -ForegroundColor Yellow -NoNewline
Write-Host " to sign in, then " -NoNewline
Write-Host "mgm megumi" -ForegroundColor Yellow -NoNewline
Write-Host " to start Megumi Code."
Write-Host "Secrets/Infisical users: " -NoNewline
Write-Host "mgm env configure" -ForegroundColor Yellow -NoNewline
Write-Host "."
