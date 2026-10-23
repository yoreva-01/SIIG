# SIIG Uninstall Script (Windows PowerShell)
# Usage: irm https://get.siig.dev/uninstall.ps1 | iex

$ErrorActionPreference = "Stop"

$siigHome = Join-Path $env:USERPROFILE ".siig"
$installDir = Join-Path $siigHome "bin"
$binaryPath = Join-Path $installDir "siig.exe"

if (Test-Path $binaryPath) {
    Write-Host "-> Removing SIIG..." -ForegroundColor Cyan
    Remove-Item $binaryPath -Force
} else {
    Write-Host "SIIG binary not found at $binaryPath" -ForegroundColor Yellow
}

# Remove from PATH
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -like "*$installDir*") {
    Write-Host "-> Removing $installDir from PATH..." -ForegroundColor Cyan
    $newPath = ($userPath -split ";" | Where-Object { $_ -ne $installDir }) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
}

# ~/.siig also holds the fetched rule cache, so it goes as well
if (Test-Path $siigHome) {
    Write-Host "-> Removing cached rules from $siigHome..." -ForegroundColor Cyan
    Remove-Item $siigHome -Recurse -Force
}

Write-Host ""
Write-Host "SIIG uninstalled successfully." -ForegroundColor Green
