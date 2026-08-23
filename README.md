# Lunefetch

A keyboard-first HTTP download manager for Linux and Windows, built with Go and Fyne.

## Features

### Downloads
- **Multi-chunk parallel downloads** with per-file chunk counts chosen by size
- **Pause, resume, cancel** with resume bound to `ETag`/`Last-Modified` via `If-Range`
- **Queue** with a configurable concurrency limit and per-row reordering
- **Scheduler** to start a download at a given `HH:MM`
- **Automatic retries** with exponential backoff, capped at two minutes
- **Per-download speed limits** from the row action menu, plus a global limiter
- **Proxy support** for HTTP, HTTPS, and SOCKS5

### Integrity and safety
- Resume progress is reconciled against the real `.part` file, so a deleted or
  truncated temp file is re-downloaded instead of published as complete
- The final size is verified and the file is flushed to disk before publishing
- Range responses are strictly validated: `Content-Range`, exact chunk lengths,
  and refusal of a `200` reply to a partial request
- Filenames are validated and destinations are confined to the download directory
- Destination policy blocks loopback, private, link-local, CGNAT, and other
  reserved ranges unless `allow_local_hosts` is enabled; cloud metadata
  endpoints stay blocked either way
- Only one instance runs per user, so two processes cannot share the database

### Interface
- Dark moonlit theme with a 260px navigation rail
- Virtualized download list with file-type icons, progress, speed, and ETA
- Status filters, file categories, and live summary counts
- Selection mode for bulk actions
- System tray with close-to-tray background operation
- Desktop notifications on completion (native on Windows, `notify-send` on Linux)

## Installation

### Installers

Download the installer for your platform from the latest GitHub release. The
Linux `.run` and Windows `.exe` installers install the desktop app, native
messaging host, application shortcut, and supported browser manifests. They do
not bundle or silently install browser extensions.

After running an installer, install the extension from the official browser
store. Firefox users can use
[Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/lunefetch/).

The Linux installer is per-user and does not require `sudo`. Remove it with:

```bash
~/.local/opt/lunefetch/uninstall
```

On Windows the installer writes to `%LocalAppData%\Programs\Lunefetch` and
registers an uninstaller in the usual Apps list.

### Source build

Requires Go 1.26 or later and the Fyne build dependencies for your platform.

```bash
git clone https://github.com/Lyravein/lunefetch.git
cd lunefetch
go build -o lunefetch .
./lunefetch
```

The resulting binary is roughly 30 MB because Fyne and SQLite are linked
statically.

## Browser Extension

Install the official extension from
[Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/lunefetch/).
Firefox 142 or newer is required. A Chromium build is produced from the same
source.

The extension intercepts eligible HTTP and HTTPS downloads and hands their URLs
to the local Lunefetch application through browser native messaging. It requires
both the desktop application and its native messaging host; platform installers
register the host automatically. For a source checkout:

```bash
./install.sh --firefox
```

Detailed browser and platform paths are documented in
[`docs/browser-installation.md`](docs/browser-installation.md).

Features include automatic interception, context-menu handoff, batch link
selection with a confirmation step, per-site and file-type rules, connection
diagnostics, and preservation of the browser download whenever a handoff fails.

The extension sends only a URL plus optional user-entered filename and
destination hints. It does not transmit cookies, authorization headers,
referrers, request bodies, or page content. Destination hints are confined to
the configured download directory by the desktop application.

To build and test the extension from source:

```bash
cd extension
npm ci
npm test
npm run check
./build.sh
npm run lint:firefox
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| **Ctrl+N** | Open the Add Download dialog |
| **Ctrl+F** | Focus the search field |
| **Ctrl+C** | Copy the selected download's URL |
| **Ctrl+P** | Pause all active downloads |
| **Delete** | Remove the selected download, with confirmation |
| **Spacebar** | Toggle pause/resume on the selected row |
| **↑↓ Arrows** | Move between rows |

Shortcuts that are also meaningful while typing are ignored when a text field has
focus. Per-row actions — open file, open folder, copy URL, set speed limit,
remove — live in the row's action menu. A queued row additionally offers
**Move Up in Queue** and **Move Down in Queue**.

### Background Mode

On desktop systems with a system tray, closing the window hides Lunefetch and
keeps active downloads running. Use the tray icon to reopen the window, add a
download, or quit. Set `close_to_tray: false` in the config file to restore
normal close behaviour.

## Configuration

### File locations

| | Linux | Windows |
|---|---|---|
| Config, API token | `~/.config/lunefetch` | `%AppData%\lunefetch` |
| Database, instance lock | `$XDG_DATA_HOME` or `~/.local/share/lunefetch` | `%LocalAppData%\lunefetch` |

### Config fields (`internal/config/config.go`)

```go
type Config struct {
    DownloadDir          string     // Default download directory
    MaxConcurrent        int        // Concurrent download limit
    MaxRetries           int        // Retry attempts per chunk (default 3)
    RetryBackoffS        int        // Base backoff seconds, doubling per attempt (default 1)
    MinFreeSpaceMB       int        // Minimum free disk space before starting
    Timeout              int        // Request timeout in seconds
    ChunkRules           ChunkRules // Chunk counts per size bucket
    SmallSize            int64      // Size thresholds selecting a chunk rule
    MediumSize           int64
    LargeSize            int64
    GlobalSpeedLimit     int64      // Bytes/sec across all downloads, 0 = unlimited
    ProxyURL             string     // http, https, or socks5 URL; empty = direct
    Notifications        bool       // Desktop notification on completion
    CloseToTray          bool       // Hide on close and keep running (default true)
    HistoryRetentionDays int        // 0 = keep history forever
    AllowLocalHosts      bool       // Permit LAN, loopback, and link-local targets
}
```

Every field is range-checked on load and on save; an invalid config is rejected
rather than silently corrected.

### Database schema

- `downloads`: `id`, `url`, `filename`, `save_dir`, `category`, `speed_limit`,
  `total_size`, `downloaded_size`, `status`, `supports_ranges`, `num_chunks`,
  `etag`, `last_modified`, `queue_position`, `scheduled_at`, `created_at`,
  `updated_at`, `deleted_at`
- `chunks`: `id`, `download_id`, `chunk_index`, `start_byte`, `end_byte`,
  `downloaded_size`, `status`, `error`, `retry_count`, `created_at`, `updated_at`

History is a soft-delete view of `downloads` where `deleted_at IS NOT NULL`.

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for layering rules and design decisions.

```
internal/
├── core/            # Download engine: chunking, ranges, limiter, destination policy
├── queue/           # Concurrency manager; decides when a download starts
├── storage/         # SQLite persistence for downloads and chunks
├── config/          # YAML config with validation and atomic writes
├── api/             # Authenticated loopback API for the browser extension
├── notify/          # Desktop notifications
├── filecat/         # Filename to category mapping
├── userpath/        # Per-user directories for Linux and Windows
├── singleinstance/  # Advisory lock preventing a second instance
└── ui/
    ├── components/  # Sidebar, header, download list, status bar, dialogs
    ├── pages/       # Downloads and history pages
    ├── layout/      # Window assembly, shortcuts, tray
    ├── store/       # The only bridge between UI and backend
    ├── theme/       # Moonlit dark theme tokens
    ├── assets/      # Embedded icon
    └── fatal/       # Startup-error window for GUI builds
```

## Testing

```bash
go test ./... -race -count=1     # Full suite with race detection
go vet ./...
bash scripts/test-install-linux.sh   # Install, upgrade, and uninstall lifecycle
```

Extension:

```bash
cd extension && npm test
```

Visual snapshots of UI components can be rendered to PNG for inspection:

```bash
LUNEFETCH_VISUAL_DIR=/tmp/lunefetch-visual \
  go test ./internal/ui/components -run 'Visual|DesktopShellRenders' -count=1
```

CI runs the full race-enabled suite and `go vet` on both Linux and Windows, plus
installer lifecycle checks and a dedicated extension job.

## Known Limitations

1. **Windows visual review is pending.** The code builds and its tests pass on
   Windows in CI, but the installer lifecycle, DPI scaling, tray rendering, and
   row action menus have not been reviewed on real Windows hardware.
2. **Dark theme only.** Light and preset themes are deferred.
3. **Queue reordering is menu-only.** A queued row can be moved one step at a
   time from its action menu; there is no drag-and-drop.
4. **Firefox desktop only.** The extension depends on native messaging, which
   Firefox for Android does not provide.
5. **Authenticated downloads are out of scope.** See
   [`docs/authenticated-download-threat-model.md`](docs/authenticated-download-threat-model.md).

## Roadmap

See [ROADMAP.md](ROADMAP.md).

## Security

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## License

MIT License; see the LICENSE file.

## Contributing

Contributions are welcome. Please submit pull requests with clear commit
messages, tests for new behaviour, and no breaking changes without discussion.

---

Built with Go, Fyne, and SQLite.
