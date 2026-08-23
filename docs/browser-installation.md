# Browser Integration Installation

Lunefetch uses the native messaging host `com.lyravein.lunefetch`. Browser
extensions can reach only the native host manifests installed for their browser
family. The Chromium extension identity is fixed as
`iidkhocioaefjlhhigiaphejnlidchke`; the Firefox identity is
`lunefetch@lyravein`.

## Linux

For normal installation, download
`Lunefetch-Setup-<version>-linux-amd64.run` from the GitHub release, make it
executable, and run it. The per-user installer installs the desktop application,
application-menu shortcut, native host, and manifests for detected browsers.
Administrator access is not required. Run `~/.local/opt/lunefetch/uninstall` to
remove the application and native-host integration.

Developers using a source checkout can run `./install.sh` to build only the
native host and install manifests for detected browsers. Use `--firefox` or
`--chromium` to limit the browser family and `--uninstall` to remove that
developer installation.

| Browser | Native messaging manifest directory |
|---------|-------------------------------------|
| Firefox | `~/.mozilla/native-messaging-hosts/` |
| Zen | `~/.zen/native-messaging-hosts/` |
| Chromium | `~/.config/chromium/NativeMessagingHosts/` |
| Google Chrome | `~/.config/google-chrome/NativeMessagingHosts/` |
| Brave | `~/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts/` |
| Microsoft Edge | `~/.config/microsoft-edge/NativeMessagingHosts/` |
| Vivaldi | `~/.config/vivaldi/NativeMessagingHosts/` |

The application and native host are installed below `~/.local/opt/lunefetch`,
with stable links in `~/.local/bin`. Re-running the installer upgrades them in
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

## Browser Extensions

Install browser extensions separately from their official stores. Installers do
not bundle or silently install extensions. Firefox releases are distributed
through [Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/lunefetch/),
and Chromium releases are distributed through the Chrome Web Store. Development
archives can still be built from source and loaded temporarily.

The Firefox extension requires Firefox 142 or newer, which is the first version
that supports the declared `data_collection_permissions`. Firefox for Android is
not supported because it does not provide native messaging.

## Application Data Locations

The native host reads the local API token written by the desktop application, so
both must agree on where per-user files live.

| | Linux | Windows |
|---|---|---|
| Config, API token | `~/.config/lunefetch` | `%AppData%\lunefetch` |
| Database, instance lock | `$XDG_DATA_HOME` or `~/.local/share/lunefetch` | `%LocalAppData%\lunefetch` |

## Troubleshooting

If the popup reports that Lunefetch is unavailable:

1. Confirm the desktop application is running. It closes to the system tray by
   default, so check the tray before relaunching.
2. Confirm the native host is registered. On Linux the manifest must exist in the
   directory listed above for your browser; on Windows the `HKCU` entry must
   point at an existing manifest file.
3. Restart the browser after installing or reinstalling the native host. Browsers
   read native messaging manifests at startup.
4. Use **Check connection** in the extension popup for a specific diagnostic.

If a second Lunefetch window refuses to open, that is intended: only one instance
runs per user, because two processes would share one database and the same
temporary files. Use the tray icon to reopen the existing window.
