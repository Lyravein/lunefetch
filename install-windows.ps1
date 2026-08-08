[CmdletBinding()]
param(
    [ValidateSet("All", "Firefox", "Chromium")]
    [string]$Browser = "All",
    [switch]$Uninstall,
    [string]$NativeHostBinary
)

$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot
$Version = (Get-Content (Join-Path $Root "VERSION") -Raw).Trim()
$InstallDir = Join-Path $env:LOCALAPPDATA "Lunefetch"
$BinaryPath = Join-Path $InstallDir "lunefetch-native-host.exe"
$ManifestPath = Join-Path $InstallDir "com.lyravein.lunefetch.json"
$FirefoxManifestPath = Join-Path $InstallDir "com.lyravein.lunefetch-firefox.json"

$ChromiumRegistryPaths = @(
    "HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\Chromium\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\Vivaldi\NativeMessagingHosts\com.lyravein.lunefetch"
)
$FirefoxRegistryPath = "HKCU:\Software\Mozilla\NativeMessagingHosts\com.lyravein.lunefetch"

function Remove-Integration {
    foreach ($Path in $ChromiumRegistryPaths + @($FirefoxRegistryPath)) {
        if (Test-Path $Path) { Remove-Item $Path -Recurse -Force }
    }
    Remove-Item $ManifestPath, $FirefoxManifestPath, $BinaryPath -Force -ErrorAction SilentlyContinue
    if ((Test-Path $InstallDir) -and -not (Get-ChildItem $InstallDir -Force)) {
        Remove-Item $InstallDir -Force
    }
}

function Write-UTF8NoBOM([string]$Path, [string]$Content) {
    $Encoding = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($Path, $Content, $Encoding)
}

if ($Uninstall) {
    Remove-Integration
    Write-Host "Lunefetch native messaging integration removed."
    exit 0
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
if ($NativeHostBinary) {
    Copy-Item -Force $NativeHostBinary $BinaryPath
} else {
    Push-Location $Root
    try {
        go build -ldflags "-X main.version=$Version" -o $BinaryPath ./cmd/native-host/
        if ($LASTEXITCODE -ne 0) { throw "Native host build failed" }
    } finally {
        Pop-Location
    }
}

$ChromiumManifest = @{
    name = "com.lyravein.lunefetch"
    description = "Lunefetch native messaging host"
    path = $BinaryPath
    type = "stdio"
    allowed_origins = @("chrome-extension://iidkhocioaefjlhhigiaphejnlidchke/")
} | ConvertTo-Json -Depth 3

$FirefoxManifest = @{
    name = "com.lyravein.lunefetch"
    description = "Lunefetch native messaging host"
    path = $BinaryPath
    type = "stdio"
    allowed_extensions = @("lunefetch@lyravein")
} | ConvertTo-Json -Depth 3

if ($Browser -in @("All", "Chromium")) {
    Write-UTF8NoBOM $ManifestPath $ChromiumManifest
    foreach ($Path in $ChromiumRegistryPaths) {
        New-Item -Force -Path $Path | Out-Null
        Set-Item -Path $Path -Value $ManifestPath
    }
}
if ($Browser -in @("All", "Firefox")) {
    Write-UTF8NoBOM $FirefoxManifestPath $FirefoxManifest
    New-Item -Force -Path $FirefoxRegistryPath | Out-Null
    Set-Item -Path $FirefoxRegistryPath -Value $FirefoxManifestPath
}

Write-Host "Installed Lunefetch native host $Version to $BinaryPath"
Write-Host "Browser detection and extension installation guidance: docs/browser-installation.md"
