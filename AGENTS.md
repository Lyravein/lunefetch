# Lunefetch Agent Guidelines

This document provides guidance for AI agents working on the Lunefetch codebase.
Current version: 1.1.0 (see `VERSION`; `scripts/check-version.sh` enforces that
the extension manifests agree).

## Quick Reference

### Build & Run
```bash
go build -o lunefetch .              # Build binary
./lunefetch                          # Run GUI application
go test ./... -race -count=1         # Full suite with race detection
go vet ./...
bash scripts/test-install-linux.sh   # Installer lifecycle
```

Extension:
```bash
cd extension && npm test && npm run check && ./build.sh && npm run lint:firefox
```

Cross-compile check after touching platform-specific code (GUI packages need cgo
and only build on a real Windows runner):
```bash
GOOS=windows go build ./internal/userpath/ ./internal/singleinstance/ \
  ./internal/notify/ ./internal/storage/ ./internal/config/ ./internal/api/ \
  ./internal/core/ ./internal/queue/ ./cmd/native-host/
```

### Core Files to Know
- `internal/core/downloader.go`: download engine, ranges, destination policy
- `internal/core/throttle.go`: global + per-task limiter
- `internal/storage/state.go`: `DownloadRecord`, schema, migrations
- `internal/queue/manager.go`: concurrency and queue ordering
- `internal/ui/components/table.go`: virtualized download list and row renderer
- `internal/ui/layout/desktop.go`: window assembly, shortcuts, tray
- `internal/ui/pages/downloads.go`: download lifecycle and row actions
- `main.go`: startup order: config, instance lock, database, GUI, API

## Desktop UI

Dark-only, default Fyne font. The shell is a 260px sidebar, a content header with
greeting/search/`New Download`, summary cards, a virtualized download list, and a
status bar. `docs/ui-redesign-plan.md` holds the locked design decisions.

- `header.go`: greeting, search, New Download, summary cards
- `sidebar.go`: status/category routes plus Settings/History/About
- `table.go`: `widget.List` with a hand-written row renderer
- `statusbar.go`: overall speed and persisted concurrency control

### Visual QA loop
Render components to PNG instead of asking a human for screenshots:
```bash
LUNEFETCH_VISUAL_DIR=/tmp/lunefetch-visual \
  go test ./internal/ui/components -run 'Visual|DesktopShellRenders' -count=1
```
In `captureCanvas`, resize the window only. The test canvas already insets by
theme padding, so resizing the content too fakes right-edge overflow.

### Speed limits
Per-download: row action menu → “Set Speed Limit…” (`table.go`), persisted via
`UpdateSpeedLimit` and applied live with `SetLimiter`. Global: `GlobalSpeedLimit`
in config, shared limiter updated with `SetRate`. Both are enforced at once and
the lower one wins.

## Error Recovery Pattern

**downloadChunkWithRetry()** - `internal/core/downloader.go`
```go
for attempt := 0; attempt <= MaxRetries; attempt++ {
    err := downloadSingleChunk(...)
    if err == nil { return }
    time.Sleep(time.Duration(attempt*RetryBackoffS) * time.Second)
}
return fmt.Errorf("max retries exceeded")
```

Exponential backoff: 1s → 2s → 4s per attempt.

## Concurrency Safety Rules

### Locking Strategy
1. `DownloadTable.records` protected by `sync.RWMutex`
2. `dt.speeds` map accessed only within mutex blocks
3. Debounced sort function (300ms delay)
4. `dp.active` map protected during concurrent start/pause/resume

**Pattern**: Always hold lock before accessing shared state:
```go
dp.mu.RLock()
entry, ok := dp.active[id]
dp.mu.RUnlock()
```

## Database Schema

### Downloads Table
```sql
CREATE TABLE downloads (
    id INTEGER PRIMARY KEY,
    url TEXT NOT NULL,
    filename TEXT,
    save_dir TEXT,
    category TEXT,
    speed_limit INTEGER,      -- bytes/sec, 0 = unlimited
    total_size INTEGER,
    downloaded_size INTEGER,
    status TEXT,              -- downloading, paused, completed, etc.
    supports_ranges BOOLEAN,
    num_chunks INTEGER,
    etag TEXT,
    last_modified TEXT,
    queue_position INTEGER,
    scheduled_at TEXT,        -- "HH:MM" or NULL
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME       -- NULL = not deleted
);
```

### Key Fields
- `speed_limit`: Used for per-download bandwidth control
- `status`: Controls UI behavior and allowed actions
- `deleted_at`: Soft delete for history preservation

## UI Component Patterns

### Dialog Structure
```go
func ShowDialog(w fyne.Window, title string, message string, onConfirm func()) {
    dialog.ShowConfirm(title, message, func(ok bool) {
        if ok && onConfirm != nil {
            onConfirm()
        }
    }, w)
}
```

### Table Actions Handler
```go
onAction := func(id int64, action string) {
    switch action {
    case "pause":
        dp.PauseDownload(id)
    case "resume":
        dp.ResumeDownload(id)
    case "delete":
        // Show confirmation first
    case "open_file":
        // Opens the file. Never repurpose this into anything destructive.
        dp.openFile(id)
    case "download_again":
        // Destructive: confirm, then reset progress and delete the file.
        components.ShowDownloadAgainDialog(dp.window, rec, func() {
            dp.DownloadAgain(id)
        })
    }
}
```

**Do not fold a destructive action into a read-only one.** `open_file` used to
show the re-download prompt for completed rows, so Open File could never open
anything, and confirming it soft-deleted the record without ever re-queueing the
URL. Re-fetching lives in `download_again` -> `DownloadAgain`, which resets
progress *before* deleting the file so a failure cannot destroy both.

## Testing Guidelines

### Unit Tests
- Test retry logic with mock HTTP server
- Validate URL blocking rules
- Verify disk space checks
- DB transaction integrity tests

### Integration Tests
- Concurrent download limits
- Rate limiter enforcement
- State transitions (pending → downloading → completed)
- Mutex protection verification (`go test -race`)

### Regression Tests Worth Knowing
- `internal/core/lifecycle_test.go`: sparse/truncated `.part` rejection, `Done()`
  closure on every path, cancel-during-spawn, backoff, destination policy
- `internal/storage/queue_position_test.go`: queue ordering and pragmas
- `internal/singleinstance/singleinstance_test.go`: second-instance refusal
- `internal/ui/components/visual_test.go`: row/shell geometry invariants
- `extension/test/lifecycle.test.mjs`: MV3 restart, sender validation, hints

## Gotchas & Common Mistakes

### 1. Don't Access Unprotected Shared State
❌ BAD: Direct access to `dp.active` without mutex
✅ GOOD: Use RLock/RUnlock pattern

### 2. Don't Hardcode Paths
Use config fields:
- `cfg.DownloadDir` for default save location
- `cfg.MaxConcurrent` for worker limit

### 3. Don't Forget Soft Delete
When deleting, set `deleted_at` rather than removing row entirely. History preserved for audits.

### 4. Don't Bypass Destination Validation
`core.GetFileInfo` and the downloader's transport enforce the SSRF policy on
every request, so URLs are screened without a separate pre-check. For paths:
- `core.ValidateFilename` / `core.SafeDownloadPath` for filenames
- `core.SafeSaveDir` for any caller-supplied directory, and always for input
  arriving from the local API or extension (it confines writes to
  `cfg.DownloadDir`)

### 5. Don't Ignore Race Detection
Run `go test -race ./...` regularly. Any race condition should be fixed immediately.

### 6. Don't Trust Persisted Progress As Bytes On Disk
`Downloader.Start` reconciles chunk progress against the real `.part` file and
verifies the final size before returning, then `Sync()`s it. A resume whose part
file was deleted or truncated must re-download, never publish a sparse file.

### 7. Every Start Return Path Must Close `done`
`Done()` is closed exactly once per `Start`, including early errors and
cancellation, and cancellation still waits for in-flight workers before the
deferred file close. Do not add an early `return` that skips it.

### 8. Use `internal/userpath` For User Directories
Never use `os.Getenv("HOME")`: it is empty on Windows, which is a shipped
target, and produces paths relative to the working directory.

### 9. One Instance Per User
`main.go` holds an advisory lock via `internal/singleinstance` before touching
the database. Two processes would share the single SQLite connection and write
the same `.part` files, so a second launch exits with a message instead.

Windows byte-range locks are mandatory, not advisory: a locked region cannot be
read even through another handle in the same process. The lock therefore sits on
a sentinel byte at offset 2^32, past any file content, so the pid written for
diagnostics stays readable. Do not move it back to byte 0.

### 10. Keep SQLite Pragmas In The DSN
`foreign_keys` and `busy_timeout` are set in the connection string and verified
at open. A bare `PRAGMA` Exec only configures one connection, which silently
disabled chunk cascade deletes. WAL is intentionally NOT enabled: its
`-wal`/`-shm` sidecars are created world-readable, defeating the 0600 database.

### 11. Report Startup Failures Through The GUI
Lunefetch is a GUI binary, so `log.Fatalf` before the main window exists is
invisible. Use `internal/ui/fatal.Show(title, message)` for any fatal startup
error (config, single-instance, database, API token). It logs and shows a
floating window.

### 12. Icons Come From `internal/ui/assets`
`assets.AppIcon` is an embedded PNG. Do NOT load `installer/lunefetch.ico` at runtime: the
installer never ships it next to the binary, and Go's image decoders cannot read
the ICO container, so both window and tray icons render blank.

### 13. Extension: MV3 Service Workers Lose Module State
`extension/src/background.js` keeps settings/connection/failures in module scope,
which Chromium wipes whenever the idle service worker is terminated. Every
listener must be registered synchronously at the top level (MV3 drops listeners
added after an `await`) and must `await ready()` before reading that state.
Otherwise a download can be intercepted using defaults after the user disabled
interception.

### 14. Extension: Validate Message Senders
`runtime.onMessage` checks `sender.id` and requires the sender URL to be under
`runtime.getURL("")`. Without it a web page that learns the extension id could
queue downloads or rewrite settings. Do NOT reject every sender carrying a tab:
an extension page opened in a tab (options.html, batch.html) legitimately has
`sender.tab` in Chromium, and rejecting it left those pages with no state.

### 15. Extension: `contextMenus.create` Is Not Idempotent
It throws on a duplicate id, and the worker re-runs the whole file on restart.
Call `removeAll` first (see `createContextMenus`).

### 15b. Extension: Filter webRequest By Request Type
`onHeadersReceived` must be registered with `types:
[...INTERCEPTABLE_REQUEST_TYPES]` and re-check `isInterceptableRequestType`
inside the handler. Registered for every type, it saw a page's own XHRs,
beacons, and keep-alive pings, and response headers cannot distinguish them:
`youtube.com/sw.js_data` is an internal fetch that really does send
`Content-Disposition: attachment`. Never widen this back to all types.

Related: read a file extension with `extensionOf`, not `split(".").pop()`. The
latter returns the whole string when there is no dot, so any dotless path was
compared against the rule set in full.

### 16. Windows Is A First-Class Target
Anything platform-specific needs a Windows path, not just a Unix one:
- Per-user directories come from `internal/userpath` (`%AppData%` for config,
  `%LocalAppData%` for state). Never `$HOME`, never a hardcoded `.local/share`.
- Desktop notifications go through Fyne's `SendNotification`; `notify-send` is a
  Linux-only fallback and does not exist on Windows.
- `openSystemPath` uses `explorer.exe` with `filepath.FromSlash`; forward slashes
  and a leading `/` confuse Explorer.
- The single-instance lock has separate flock/LockFileEx implementations.
- Cross-compile check: `GOOS=windows go build ./internal/... ./cmd/...` (the GUI
  packages need cgo, so they only build on a real Windows runner).

### 17. Write Config Atomically
`config.Save` writes a temp file in the same directory and renames over the
target. `Load` treats a parse error as fatal, so a truncated config would brick
startup.

## Performance Tips

### For Large Datasets (>1k rows)
The download list uses Fyne's virtualized `widget.List`; rows are created and
reused by the list renderer. Keep row updates layout-managed and avoid manual
child `Resize()`/`Move()` calls when extending the component.

### Rate Limiter Best Practices
- Update global limiter once at startup
- Per-task limits override global when stricter
- Both enforced simultaneously (lower wins)

## API Usage

### Start New Download
```go
req := components.AddURLRequest{
    URL:         "https://example.com/file.zip",
    Filename:    "custom-name.zip",  // optional
    SaveDir:     "/path/to/dir",     // optional, uses cfg.DownloadDir if empty
    Category:    "Media",            // optional, auto-detected
    SpeedLimit:  5*1024*1024,        // 5 MB/s
    ScheduledAt: "14:30",            // optional, "HH:MM" format
}
addCh <- req
```

### Query Store
```go
rec, _ := sm.GetDownload(id)
if rec != nil {
    fmt.Printf("Status: %s, Size: %d/%d\n", 
        rec.Status, rec.DownloadedSize, rec.TotalSize)
}
```

## Debugging Tips

### Enable Verbose Logging
```go
// In downloader.go
log.Printf("Starting download %d: %s", rec.ID, rec.URL)
log.Printf("Chunk %d: offset=%d, size=%d", chunkIndex, offset, size)
```

### Check Rate Limiter State
```go
fmt.Printf("Global limiter rate: %.2f B/s, allow: %v\n", 
    *globalLimiter.Rate(), (*globalLimiter).Allow())
```

### Trace Concurrency Issues
Use `go run -race` or `go test -race` to catch data races early.

## Open Work

### Next up
1. Manual Windows review: row action menus, installer lifecycle, and 100/125/150%
   DPI scaling. Nothing here has run on real Windows hardware; CI covers the build
   and the test suite only.
2. Accessibility audit: screen-reader labels and full keyboard workflows.

### Deferred by decision
- Light and preset themes; the desktop is intentionally dark-only.
- Drag-and-drop queue reordering. Single-step moves are wired to the row action
  menu (`queue_up` / `queue_down` → `DownloadsPage.MoveInQueue`).
- Authenticated downloads; see `docs/authenticated-download-threat-model.md`.

---

Last updated: 2026-08-23 (1.1.0 release: docs refresh, Windows parity, extension MV3 hardening, backend hardening)
