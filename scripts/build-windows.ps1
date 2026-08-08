[CmdletBinding()]
param(
    [string]$OutputDir,
    [string]$ISCC
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Version = (Get-Content (Join-Path $Root "VERSION") -Raw).Trim()

if (-not $OutputDir) {
    $OutputDir = Join-Path $Root "dist"
}
$OutputDir = [System.IO.Path]::GetFullPath($OutputDir)
$StageDir = Join-Path $OutputDir "windows-amd64"
New-Item -ItemType Directory -Force -Path $StageDir | Out-Null

Push-Location $Root
try {
    $env:CGO_ENABLED = "1"
    go build -trimpath -ldflags "-s -w -H=windowsgui -X main.version=$Version" -o (Join-Path $StageDir "lunefetch.exe") ./
    if ($LASTEXITCODE -ne 0) { throw "Windows application build failed" }

    go build -trimpath -ldflags "-s -w -X main.version=$Version" -o (Join-Path $StageDir "lunefetch-native-host.exe") ./cmd/native-host/
    if ($LASTEXITCODE -ne 0) { throw "Windows native host build failed" }
} finally {
    Pop-Location
}

Copy-Item (Join-Path $Root "lunefetch.ico") $StageDir -Force
Copy-Item (Join-Path $Root "LICENSE") $StageDir -Force

if (-not $ISCC) {
    $Candidates = @(
        (Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6\ISCC.exe"),
        (Join-Path $env:ProgramFiles "Inno Setup 6\ISCC.exe")
    )
    $ISCC = $Candidates | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
}
if (-not $ISCC -or -not (Test-Path $ISCC)) {
    throw "Inno Setup 6 compiler not found. Install it or pass -ISCC <path-to-ISCC.exe>."
}

& $ISCC "/DAppVersion=$Version" "/DSourceDir=$StageDir" "/DOutputDir=$OutputDir" (Join-Path $Root "installer\lunefetch.iss")
if ($LASTEXITCODE -ne 0) { throw "Installer compilation failed" }

$Installer = Join-Path $OutputDir "Lunefetch-Setup-$Version-windows-amd64.exe"
if (-not (Test-Path $Installer)) { throw "Installer output missing: $Installer" }
Write-Host $Installer
