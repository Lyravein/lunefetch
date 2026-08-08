# Lunefetch

Lunefetch is a desktop HTTP/HTTPS download manager built with Go and Fyne. It
supports parallel ranged downloads, pause and resume, persistent queues,
scheduling, speed limits, and browser handoff through native messaging.

> Lunefetch v1.0.0 is the first public release. Review [SECURITY.md](SECURITY.md)
> before enabling browser integration.

## Features

- Fyne desktop interface with status and category filters, search, history,
  sortable downloads, and per-download actions
- Multi-part downloads when the server advertises byte-range support
- Byte-range response validation and resumable chunk progress in SQLite
- Configurable concurrency, retries, proxy, global speed, and per-download speed
- One-shot daily scheduling using `HH:MM` local time
- Firefox, Zen, Chromium, Chrome, Brave, Edge, and Vivaldi browser integration
- Authenticated loopback API between the native host and desktop application
- Safe destination handling: validated basenames, per-download temporary files,
  and no silent replacement of existing files

Browser handoff supports replayable, unauthenticated HTTP/HTTPS GET downloads,
including a reviewed page-link batch flow and optional user-entered filename and
destination hints. Browser cookies, authorization headers, request bodies, page
content, and referrers are intentionally not transferred. Recent failures can be
retried from the extension popup. Authenticated transfer remains disabled; see
[`docs/authenticated-download-threat-model.md`](docs/authenticated-download-threat-model.md).

## Requirements

- Go 1.26.5 or the version declared in `go.mod` for development builds
- Linux desktop dependencies required by Fyne for Linux development builds
- Node.js 22.x and npm for extension tests and source builds
- Bash and `zip` for extension packaging

End users should use the release packages instead of building the desktop
application. Linux binaries and the Windows installer are available from the
[v1.0.0 GitHub release](https://github.com/Lyravein/lunefetch/releases/tag/v1.0.0).

## Build

```bash
go build -o lunefetch .
go build -o lunefetch-native-host ./cmd/native-host
```

Run the desktop application:

```bash
./lunefetch
```

The application stores configuration in `~/.config/lunefetch/` and download
state in `~/.local/share/lunefetch/`.

## Browser Integration

Build the native host, install a browser-specific host manifest, and package
the extensions:

```bash
./install.sh --firefox
./install.sh --chromium
```

Running `./install.sh` without flags installs manifests for detected browser
families. Extension archives are written to `extension/dist/`.

The Firefox extension is being submitted to Firefox Add-ons. Until it is
approved, download the Firefox release package and use the temporary-loading
instructions below. The browser extension requires the Lunefetch desktop
application and native host on the same computer. It transfers only replayable
HTTP/HTTPS URLs and optional filename or destination hints explicitly supplied
by the user. It does not transfer cookies, authorization headers, referrers,
request bodies, or page content.

Windows users can install the application and native browser host with the
`Lunefetch-Setup-<version>-windows-amd64.exe` attached to each GitHub release.
The PowerShell installer remains available for development. Complete browser
paths, package verification, upgrade, and uninstall details are documented in
[`docs/browser-installation.md`](docs/browser-installation.md).

Firefox or Zen:

1. Open `about:debugging` and select **This Firefox**.
2. Select **Load Temporary Add-on**.
3. Select `extension/dist/lunefetch-firefox.zip`.

Chromium, Chrome, or Brave:

1. Extract `extension/dist/lunefetch-chromium.zip` to a persistent directory.
2. Open the browser's extensions page and enable Developer mode.
3. Select **Load unpacked** and choose the extracted directory.

The Chromium package contains a fixed public key, producing extension ID
`iidkhocioaefjlhhigiaphejnlidchke`. The native-host manifest permits only that
ID. Replacing the key requires updating `install.sh` at the same time.

## Configuration

Configuration is stored at `~/.config/lunefetch/config.yaml` with mode `0600`.
The following defaults are representative:

```yaml
download_dir: ~/Downloads
max_retries: 3
timeout: 30
chunk_rules:
  small: 4
  medium: 8
  large: 16
  xlarge: 32
small_size: 1073741824
medium_size: 5368709120
large_size: 107374182400
max_concurrent_downloads: 2
global_speed_limit: 0
proxy_url: ""
notifications: true
history_retention_days: 30
```

Invalid or unsafe values are rejected during startup instead of being silently
normalized. A configured proxy must use `http`, `https`, or `socks5`.

## Development

```bash
go test ./...
go test -race ./...
go vet ./...
```

The production entrypoint is the Fyne desktop UI under `internal/ui/`.
`internal/ui-tui/` is retained as legacy code and is not exposed by the current
binary.

### Build the Extension

The extension source is readable and reproducible from the tagged release:

```bash
git checkout v1.0.0
scripts/check-version.sh
cd extension
npm ci
./build.sh
```

The Firefox package is written to
`extension/dist/lunefetch-firefox.zip`. The source files are not transpiled,
concatenated, bundled, or minified. `build.sh` copies the source files into the
package, creates the ZIP archive, and performs simple release-metadata
substitution. The npm dependencies are development and testing tools only.

For Mozilla source-code review, the archive must include `VERSION`,
`extension/build.sh`, `extension/package.json`, `extension/package-lock.json`,
`extension/src/`, `extension/manifests/`, `extension/icons/`, and
`scripts/check-version.sh`. Do not include `node_modules/` or generated
`extension/dist/` files in that source archive.

See [ARCHITECTURE.md](ARCHITECTURE.md) and [ROADMAP.md](ROADMAP.md) for design
and release work.

## License

Licensed under the [MIT License](LICENSE).
