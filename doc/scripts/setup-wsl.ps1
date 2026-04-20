#Requires -Version 5.1
<#
.SYNOPSIS
    Installs WSL2 with Ubuntu 24.04 on Windows 10/11.
.DESCRIPTION
    Checks whether WSL and Ubuntu-24.04 are already present, installs or
    upgrades as needed, and prints next-step instructions.
.NOTES
    Must be run from an Administrator PowerShell session.
#>

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# ── Helper ─────────────────────────────────────────────────────────────────────

function Write-Step  { param($msg) Write-Host "`n==> $msg" -ForegroundColor Cyan }
function Write-Ok    { param($msg) Write-Host "    [OK] $msg" -ForegroundColor Green }
function Write-Warn  { param($msg) Write-Host "    [!!] $msg" -ForegroundColor Yellow }
function Write-Fail  { param($msg) Write-Host "    [ERROR] $msg" -ForegroundColor Red; exit 1 }

# ── Check: Administrator ───────────────────────────────────────────────────────

Write-Step "Checking privileges"
$principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Fail "This script must be run as Administrator.`n  Right-click PowerShell -> 'Run as administrator', then re-run."
}
Write-Ok "Running as Administrator"

# ── Check: Windows build ───────────────────────────────────────────────────────

Write-Step "Checking Windows version"
$build = [System.Environment]::OSVersion.Version.Build
if ($build -lt 19041) {
    Write-Fail "WSL2 requires Windows 10 build 19041 or later (you have $build).`n  Please update Windows before continuing."
}
Write-Ok "Windows build $build is supported"

# ── Check: WSL present ────────────────────────────────────────────────────────

Write-Step "Checking WSL installation"
$wslExe = Get-Command wsl.exe -ErrorAction SilentlyContinue
$wslPresent = $null -ne $wslExe

if ($wslPresent) {
    Write-Ok "wsl.exe found"
} else {
    Write-Warn "WSL not detected — will install"
}

# ── Check: Ubuntu-24.04 already registered ────────────────────────────────────

$ubuntuInstalled = $false
if ($wslPresent) {
    Write-Step "Checking for existing Ubuntu-24.04 distribution"
    # wsl --list output contains non-ASCII chars (BOM/UTF-16); decode safely
    $rawList = & wsl.exe --list 2>&1 | Out-String
    if ($rawList -match 'Ubuntu-24\.04') {
        $ubuntuInstalled = $true
        Write-Ok "Ubuntu-24.04 is already registered"
    } else {
        Write-Warn "Ubuntu-24.04 not found — will install"
    }
}

# ── Set WSL2 as default ───────────────────────────────────────────────────────

Write-Step "Setting WSL default version to 2"
try {
    & wsl.exe --set-default-version 2 2>&1 | Out-Null
    Write-Ok "WSL default version set to 2"
} catch {
    Write-Warn "Could not set WSL default version (may not matter if WSL is not yet installed)"
}

# ── Install Ubuntu-24.04 ──────────────────────────────────────────────────────

if (-not $ubuntuInstalled) {
    Write-Step "Installing Ubuntu-24.04 via 'wsl --install'"
    Write-Host "    This may take several minutes and could prompt for a reboot ..."
    & wsl.exe --install -d Ubuntu-24.04
    $exitCode = $LASTEXITCODE

    if ($exitCode -eq 0) {
        Write-Ok "Ubuntu-24.04 installed successfully"
    } elseif ($exitCode -eq 3010) {
        Write-Warn "A reboot is required to complete WSL installation."
        Write-Warn "Please reboot, then re-run this script (or launch Ubuntu from the Start Menu)."
        exit 3010
    } else {
        Write-Fail "wsl --install exited with code $exitCode.`n  Check Windows Features: 'Virtual Machine Platform' and 'Windows Subsystem for Linux' must be enabled."
    }
} else {
    Write-Step "Verifying Ubuntu-24.04 uses WSL version 2"
    $verboseList = & wsl.exe --list --verbose 2>&1 | Out-String
    if ($verboseList -match 'Ubuntu-24\.04\s+\S+\s+1') {
        Write-Warn "Ubuntu-24.04 is running WSL version 1 — upgrading to version 2 ..."
        & wsl.exe --set-version Ubuntu-24.04 2
        Write-Ok "Upgraded to WSL 2"
    } else {
        Write-Ok "Ubuntu-24.04 is already on WSL 2"
    }
}

# ── Done ──────────────────────────────────────────────────────────────────────

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host " WSL2 + Ubuntu-24.04 setup complete!" -ForegroundColor Green
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host ""
Write-Host " Next steps:"
Write-Host "   1. Launch Ubuntu 24.04 from the Start Menu (first launch creates your Linux user)."
Write-Host "   2. Inside the Ubuntu terminal, run the prerequisites setup script:"
Write-Host ""
Write-Host "        chmod +x doc/scripts/setup-ubuntu.sh" -ForegroundColor Yellow
Write-Host "        ./doc/scripts/setup-ubuntu.sh" -ForegroundColor Yellow
Write-Host ""
Write-Host "   See doc/INSTALL.md for the full installation guide."
Write-Host ""
