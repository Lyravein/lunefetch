# Lunefetch

A high-performance, keyboard-first download manager built with Go and Fyne.

## Features

### Core Capabilities
- **Keyboard shortcuts** for efficient navigation (Enter, Delete, Ctrl+C, Ctrl+P, Spacebar, Arrows)
- **Concurrency-safe** download management with mutex protection
- **Error recovery** with exponential backoff (1s → 2s → 4s retries)
- **File safety** with disk space pre-checks and duplicate filename handling
- **Per-task speed limits** via right-click context menu (MB/s input)
- **Global bandwidth limiter** from settings page (KB/s conversion)
- **Fresh restart dialog** for completed/cancelled downloads

### Performance
- Benchmark-tested scaling: 20 concurrent downloads → ~5.24 MB/s aggregate throughput
- Dynamic rate limiting without application restart
- SQLite database backend for reliable persistence

## Installation

### Installers

Download the installer for your platform from the latest GitHub release. The
Linux `.run` installer and Windows `.exe` installer install the desktop app,
native messaging host, application shortcut, and supported browser manifests
automatically. They do not bundle or silently install browser extensions.

After running an installer, install the extension manually from the official
browser store. Firefox users can use [Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/lunefetch/).

The Linux installer is per-user and does not require `sudo`. It can be removed
with:

```bash
~/.local/opt/lunefetch/uninstall
```

### Source Build Prerequisites

- Go 1.26 or later
- Fyne v2.x framework

For source builds:
```bash
git clone https://github.com/Lyravein/lunefetch.git
cd lunefetch
go build -o lunefetch .
./lunefetch
```

Binary size: ~40MB

## Firefox Extension

Install the official extension from [Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/lunefetch/).
It intercepts supported HTTP/HTTPS downloads and sends their URLs to the local
Lunefetch application through Firefox Native Messaging.

The extension requires both the desktop application and its native messaging
host. The platform installer registers the host automatically. For a source
checkout, run:

```bash
./install.sh --firefox
```

The Windows installer registers the native host automatically. Detailed browser
and platform paths are documented in
[`docs/browser-installation.md`](docs/browser-installation.md).

Extension features include automatic interception, context-menu handoff, batch
link selection, per-site and file-type rules, connection diagnostics, and safe
fallback to Firefox when Lunefetch is unavailable. It does not transmit browser
cookies, authorization headers, referrers, request bodies, or passwords to the
desktop application.

To build and test the extension from source:

```bash
cd extension
npm ci
npm test
./build.sh
npm run lint:firefox
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| **Enter** | Open downloaded file |
| **Delete** | Confirm deletion of selected download |
| **Ctrl+C** | Copy URL to clipboard |
| **Ctrl+P** | Pause all active downloads |
| **Spacebar** | Toggle pause/resume on current row |
| **↑↓ Arrows** | Navigate table rows |
| **Right-click** | Context menu with actions |

### Background Mode

On desktop systems with a system tray, closing the window hides Lunefetch and
keeps active downloads running. Use the tray icon to reopen the window, add a
download, or quit the application. Set `close_to_tray: false` in
`~/.config/lunefetch/config.yaml` to restore normal close behavior.

## Configuration

### Config Fields (`internal/config/config.go`)
```go
type Config struct {
    DownloadDir      string // Default download directory
    MaxConcurrent    int    // Concurrent download limit
    MaxRetries       int    // Retry attempts per chunk (default: 3)
    RetryBackoffS    int    // Initial backoff seconds (default: 1)
    MinFreeSpaceMB   int64  // Minimum free disk space (MB)
    AllowLocalHosts  bool   // Allow localhost/file:// URLs
    CloseToTray      bool   // Hide window on close and keep running (default: true)
    // ... additional fields
}
```

### Database Schema
- `downloads` table: ID, URL, Filename, SaveDir, SpeedLimit, TotalSize, DownloadedSize, Status, SupportsRanges, NumChunks, ETag, LastModified, QueuePosition, ScheduledAt, CreatedAt, UpdatedAt, DeletedAt
- `chunks` table: ID, DownloadID, ChunkIndex, Offset, Size, MD5, FilePath

## Architecture

### Components
```
internal/
├── core/              # Download engine
│   ├── downloader.go  # Rate-limited download loop
│   └── throttle.go    # Global + per-task limiter logic
├── storage/           # Data layer
│   ├── state.go       # DownloadRecord model, SpeedLimit field
│   └── history_test.go
├── queue/             # Concurrent worker pool
│   └── manager.go     # Enqueue/dequeue with concurrency control
├── ui/
│   ├── components/    # Reusable widgets
│   │   ├── table.go   # Keyboard-navigable download table
│   │   ├── dialogs.go # AddURL + FreshRestart dialogs
│   │   └── toolbar.go # Search + add button
│   ├── pages/         # Application pages
│   │   └── downloads.go # Main downloads page with actions
│   └── store/         # UI-state interface
│       └── impl.go    # Mutator interface (UpdateSpeedLimit)
└── api/               # REST API wrapper
```

### Concurrency Model
- **QueueManager**: Manages max concurrent downloads with semaphore pattern
- **Mutex protection**: All shared state protected by RWMutex
- **Debounced sorting**: 300ms delay to prevent thrashing
- **Workers sync.WaitGroup**: Tracks in-flight downloads

### Rate Limiting
- Per-task limiter: `download.SetLimiter(core.NewLimiter(speedLimit))`
- Global limiter: Shared across all downloads, updated dynamically
- Bottleneck logic: Both limits enforced simultaneously (lower wins)

## Testing

### Run Tests
```bash
go test ./... -race -v
go test ./internal/core/... ./internal/storage/... ./internal/ui/components/...
```

### Benchmark Suite
```bash
cd internal/benchmarks
go test -bench=. -benchmem ./concurrency_test.go
```

Tests verify:
- Race condition detection (`-race`)
- Retry logic with exponential backoff
- Disk space validation
- URL blocking rules
- DB transaction integrity

## Known Limitations

1. **Virtual DOM for large datasets**: table virtualization is deferred due to Fyne API limitations.
2. **Drag-and-drop reordering**: not implemented.
3. **Firefox Android**: the extension supports Firefox desktop only because Native Messaging is required.

## Future Enhancements

- [ ] Drag-and-drop reordering
- [ ] Complete accessibility audit (screen reader labels and keyboard workflows)
- [ ] Virtual DOM optimization for tables >1k rows
- [ ] E2E GUI tests

## License

MIT License - see LICENSE file

## Contributing

Contributions welcome! Please submit PRs with:
- Clear commit messages
- Test coverage for new features
- No breaking changes without discussion

---

Built with Go, Fyne, and SQLite.
