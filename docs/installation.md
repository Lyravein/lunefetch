# Installation

## Installers

Download the installer for your platform from the
[latest release](https://github.com/Lyravein/lunefetch/releases/latest).

Both installers set up the desktop application, the native messaging host, an
application shortcut, and the supported browser manifests. Neither bundles nor
silently installs a browser extension: install that from the official store
afterwards.

### Linux

The `.run` installer is per-user and does not require `sudo`.

```bash
chmod +x Lunefetch-Setup-*-linux-amd64.run
./Lunefetch-Setup-*-linux-amd64.run
```

To remove it:

```bash
~/.local/opt/lunefetch/uninstall
```

### Windows

The `.exe` installer writes to `%LocalAppData%\Programs\Lunefetch` and registers
an uninstaller in the usual Apps list.

## Browser extension

Install from
[Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/lunefetch/).
Firefox 142 or newer is required. A Chromium build is produced from the same
source but is not published to the Chrome Web Store yet.

The extension needs both the desktop application and its native messaging host.
The platform installers register the host for you. From a source checkout:

```bash
./scripts/install-native-host.sh --firefox
```

Firefox treats this extension's `<all_urls>` access as optional. If you decline
it, automatic interception cannot run and downloads stay in Firefox; the popup
then offers an **Allow all sites** button.

Per-browser manifest paths and troubleshooting live in
[browser-installation.md](browser-installation.md).

## Building from source

Requires Go 1.26 or later and the Fyne build dependencies for your platform.

```bash
git clone https://github.com/Lyravein/lunefetch.git
cd lunefetch
go build -o lunefetch .
./lunefetch
```

The binary is roughly 30 MB because Fyne and SQLite are linked statically.
