$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Installer = Join-Path $Root "install-windows.ps1"
$InstallDir = Join-Path $env:LOCALAPPDATA "Lunefetch"
$RegistryPaths = @(
    "HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\Chromium\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\Vivaldi\NativeMessagingHosts\com.lyravein.lunefetch",
    "HKCU:\Software\Mozilla\NativeMessagingHosts\com.lyravein.lunefetch"
)

try {
    & $Installer -Browser All
    $Binary = Join-Path $InstallDir "lunefetch-native-host.exe"
    if (-not (Test-Path $Binary)) { throw "native host binary missing" }
    $ExpectedVersion = (Get-Content (Join-Path $Root "VERSION") -Raw).Trim()
    if ((& $Binary --version) -ne "lunefetch-native-host $ExpectedVersion") { throw "native host version mismatch" }
    foreach ($Path in $RegistryPaths) {
        if (-not (Test-Path $Path)) { throw "missing registry entry: $Path" }
        $Manifest = (Get-Item $Path).GetValue("")
        if (-not (Test-Path $Manifest)) { throw "missing manifest: $Manifest" }
    }

    & $Installer -Browser All
    & $Installer -Uninstall
    if (Test-Path $Binary) { throw "native host binary remains after uninstall" }
    foreach ($Path in $RegistryPaths) {
        if (Test-Path $Path) { throw "stale registry entry: $Path" }
    }
} finally {
    & $Installer -Uninstall
}
