# Lunefetch

A keyboard-first HTTP download manager for Linux and Windows, built with Go and
Fyne. Chunked parallel downloads, a queue you can actually see, and a browser
extension that hands downloads over without leaking your session.

## Features

- **Multi-chunk parallel downloads**, with the chunk count chosen by file size
- **Pause, resume, cancel**, with resume validated against `ETag` and
  `Last-Modified` so a changed file is never stitched together
- **Queue** with a configurable concurrency limit and per-row reordering
- **Scheduler** to start a download at a given time
- **Speed limits** per download and globally; the stricter one wins
- **Browser extension** for Firefox that intercepts downloads and hands off only
  the URL, never cookies, headers, or page content
- **Automatic retries** with exponential backoff
- **Proxy support** for HTTP, HTTPS, and SOCKS5
- **System tray** with close-to-tray background operation
- Dark moonlit interface with a virtualized list, live speed, and ETA

Resume is reconciled against the real temp file on disk, so a deleted or
truncated one is re-downloaded rather than published as if it were complete.
Filenames are validated, destinations are confined to your download directory,
and only one instance runs per user.

## Install

Grab the installer for your platform from the
[latest release](https://github.com/Lyravein/lunefetch/releases/latest), then
install the browser extension from
[Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/lunefetch/).

The Linux installer is per-user and needs no `sudo`. Full instructions, including
building from source, are in [docs/installation.md](docs/installation.md).

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| **Ctrl+N** | Open the Add Download dialog |
| **Ctrl+F** | Focus the search field |
| **Ctrl+C** | Copy the selected download's URL |
| **Ctrl+P** | Pause all active downloads |
| **Delete** | Remove the selected download, with confirmation |
| **Spacebar** | Toggle pause/resume on the selected row |
| **↑↓ Arrows** | Move between rows |

Shortcuts that also mean something while typing are ignored when a text field has
focus. Per-row actions live in the row's action menu: open file, open folder, copy
URL, set a speed limit, move within the queue, remove, and **Download Again** on a
finished row.

## Documentation

- [Installation](docs/installation.md) — installers, extension, source build
- [Configuration](docs/configuration.md) — config fields, file locations, schema
- [Development](docs/development.md) — project layout, tests, CI
- [Architecture](ARCHITECTURE.md) — layering rules and design decisions
- [Changelog](CHANGELOG.md) · [Roadmap](ROADMAP.md) · [Security](SECURITY.md)

## Known limitations

- Windows behaviour is verified by CI but has not had a visual review on real
  hardware: DPI scaling and tray rendering are unconfirmed.
- Dark theme only.
- Queue reordering is one step at a time from the action menu; no drag-and-drop.
- The extension is Firefox desktop only. It depends on native messaging, which
  Firefox for Android does not provide.
- Authenticated downloads are out of scope; see the
  [threat model](docs/authenticated-download-threat-model.md).

## Contributing

Pull requests are welcome. Please include tests for new behaviour and a clear
commit message, and open an issue first for anything that changes existing
behaviour. See [docs/development.md](docs/development.md) to get set up.

## License

MIT. See [LICENSE](LICENSE).
