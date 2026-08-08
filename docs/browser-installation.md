# Browser Integration Installation

Lunefetch uses the native messaging host `com.lyravein.lunefetch`. Browser
extensions can reach only the native host manifests installed for their browser
family. The Chromium extension identity is fixed as
`iidkhocioaefjlhhigiaphejnlidchke`; the Firefox identity is
`lunefetch@lyravein`.

## Linux

Run `./install.sh` to build the native host and install manifests for detected
browsers. Use `--firefox` or `--chromium` to limit the browser family and
`--uninstall` to remove the native host and every supported manifest.

| Browser | Native messaging manifest directory |
|---------|-------------------------------------|
| Firefox | `~/.mozilla/native-messaging-hosts/` |
| Zen | `~/.zen/native-messaging-hosts/` |
| Chromium | `~/.config/chromium/NativeMessagingHosts/` |
| Google Chrome | `~/.config/google-chrome/NativeMessagingHosts/` |
| Brave | `~/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts/` |
| Microsoft Edge | `~/.config/microsoft-edge/NativeMessagingHosts/` |
| Vivaldi | `~/.config/vivaldi/NativeMessagingHosts/` |

The native host binary is installed at
`~/.local/bin/lunefetch-native-host`. Re-running the installer upgrades it in
place without changing either extension identity.

## Windows

For normal installation, download
`Lunefetch-Setup-<version>-windows-amd64.exe` from the GitHub release and run
it. The per-user installer includes the desktop application and native host,
creates a Start Menu shortcut, optionally creates a desktop shortcut, and
registers native messaging for every supported browser. Administrator access
is not required. Lunefetch can be removed from Windows **Installed apps**.

Developers can instead run
`powershell -ExecutionPolicy Bypass -File .\install-windows.ps1`. Use
`-Browser Firefox` or `-Browser Chromium` to limit native-host registration,
and `-Uninstall` to remove the developer-installed binary, manifests, and
registry entries.

The graphical installer writes into `%LOCALAPPDATA%\Programs\Lunefetch`.
The developer script writes into `%LOCALAPPDATA%\Lunefetch`. Both register
per-user `HKCU` native messaging entries for Firefox, Chrome, Chromium, Brave,
Edge, and Vivaldi.

## Extension Packages

Release archives are in `extension/dist/`. Verify them before installation:

```sh
cd extension/dist
sha256sum -c SHA256SUMS
```

Firefox release packages are distributed through Firefox Add-ons. Chromium
release packages are distributed through the Chrome Web Store. Development
archives can be loaded temporarily from the corresponding unpacked directory.
