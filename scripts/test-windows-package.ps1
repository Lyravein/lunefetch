[CmdletBinding()]
param([Parameter(Mandatory = $true)][string]$Installer)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Version = (Get-Content (Join-Path $Root "VERSION") -Raw).Trim()
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\Lunefetch"
$App = Join-Path $InstallDir "lunefetch.exe"
$NativeHost = Join-Path $InstallDir "lunefetch-native-host.exe"
$Uninstaller = Join-Path $InstallDir "unins000.exe"
$RegistryPaths = @(
    "HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\Chromium\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\Vivaldi\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\Mozilla\NativeMessagingHosts\com.lyravein.lunefetch"
)

try {
    $Process = Start-Process -FilePath ([System.IO.Path]::GetFullPath($Installer)) -ArgumentList "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART" -Wait -PassThru
    if ($Process.ExitCode -ne 0) { throw "installer exited with $($Process.ExitCode)" }
    if (-not (Test-Path $App)) { throw "application binary missing" }
    if (-not (Test-Path $NativeHost)) { throw "native host binary missing" }
    if ((& $NativeHost --version) -ne "lunefetch-native-host $Version") { throw "native host version mismatch" }

    foreach ($Path in $RegistryPaths) {
        if (-not (Test-Path $Path)) { throw "missing registry entry: $Path" }
        $Manifest = (Get-Item $Path).GetValue("")
        if (-not (Test-Path $Manifest)) { throw "missing manifest: $Manifest" }
        $JSON = Get-Content $Manifest -Raw | ConvertFrom-Json
        if ($JSON.path -ne $NativeHost) { throw "manifest host path mismatch: $Manifest" }
    }

    $Process = Start-Process -FilePath $Uninstaller -ArgumentList "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART" -Wait -PassThru
    if ($Process.ExitCode -ne 0) { throw "uninstaller exited with $($Process.ExitCode)" }
    if (Test-Path $App) { throw "application remains after uninstall" }
    if (Test-Path $NativeHost) { throw "native host remains after uninstall" }
    foreach ($Path in $RegistryPaths) {
        if (Test-Path $Path) { throw "stale registry entry: $Path" }
    }
} finally {
    if (Test-Path $Uninstaller) {
        Start-Process -FilePath $Uninstaller -ArgumentList "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART" -Wait
    }
}
