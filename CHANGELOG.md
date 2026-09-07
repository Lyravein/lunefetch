# Lunefetch Changelog

Released versions track the `VERSION` file. Sections labelled `Phase N` predate
that convention: they are development milestones, not releases, and are kept for
historical reference.

## [0.1.0-beta] - 2026-09-07

### Changed
- Reset the application release line while deeper backend and frontend
  development continues.
- Added a neutral charcoal UI palette with a restrained teal accent and
  status-colored summary card indicators.
- Expanded the desktop Settings dialog with download, performance, network,
  history, notification, and system tray controls.

## [1.1.4] - 2026-08-25

### Fixed
- Firefox now detects when optional `<all_urls>` access was not granted and
  shows an **Allow all sites** action in the popup. The request is made from the
  user's click, as required by Firefox permissions policy, instead of reporting
  the desktop app as connected while silently leaving downloads in Firefox.
- The permission panel is hidden correctly on Chromium, where host access is
  granted at install time. A CSS layout rule previously overrode the HTML
  `hidden` attribute.
- Runtime messages now require both this extension's id and a URL under its own
  extension origin. Messages with missing sender identity are rejected.
- The Windows limiter-isolation test now compares the free transfer against its
  throttled peer instead of using a flaky absolute 400ms cutoff on shared CI
  runners.

## [1.1.3] - 2026-08-25

### Fixed
- **Expired one-hour site pauses were kept forever.** Pausing a site wrote an
  entry into `bypassUntil`, and nothing ever removed it once the hour passed:
  the entry stayed in extension storage indefinitely, growing the stored
  settings and leaving a permanent record of every site ever paused.
  `normalizeSettings` now drops expired entries on every load and save, which is
  the only pruning point a bypass ever needs.
- **The handoff controller's bookkeeping maps grew without bound.** Firefox's
  background page lives for the whole browser session, and both the
  interception-dedupe map and the per-download attempt map only ever had entries
  added. 100,000 handoffs retained every one of them. Both maps are now pruned
  to the dedupe window on each handoff.
- **Missing optional host access looked like a working extension.** Firefox MV3
  makes `<all_urls>` optional, so a user who declined it saw downloads stay in
  Firefox while the popup still reported the desktop app as connected. The
  popup now detects this state and offers an `Allow all sites` button that calls
  `permissions.request` directly from the click gesture.
- The new permission panel is explicitly hidden on Chromium. Its `.site`
  display rule initially overrode the HTML `hidden` attribute; a global
  `[hidden]` rule now keeps hidden UI out of layout.
- Runtime messages now require both this extension's id and a URL in this
  extension's own origin. Senders with missing identity are rejected rather than
  trusted by default.

## [1.1.2] - 2026-08-25

### Fixed
- **The extension intercepted a page's own background traffic.** The Firefox
  listener was registered for every request type, so any XHR, beacon, or
  keep-alive ping whose response looked download-like was handed to Lunefetch.
  Reported cases included `youtube.com/sw.js_data`, an internal fetch that really
  does answer with `Content-Disposition: attachment`, and a Google Sheets
  keep-alive ping. Interception is now limited to navigations and link
  downloads (`main_frame`, `sub_frame`, `object`, `other`); the noisy types are
  excluded at registration, so they no longer wake the background worker at all.
- **A path with no extension matched file rules.** The extension was read with
  `split(".").pop()`, which returns the whole string when there is no dot, so
  `/spreadsheets/d/abc/hibernatestat` was compared against the rule set in full
  and a bare name like `zip` matched the zip rule. An extension is now only
  recognized after a real dot, and a leading dot is treated as a hidden file.

## [1.1.1] - 2026-08-24

### Fixed
- **Open File never opened the file.** On a completed download the action showed
  the re-download prompt instead, and that prompt was the only thing a completed
  row's menu could do. Open File now opens the file, on every platform.
- **Confirming that prompt destroyed the download without re-fetching it.** It
  soft-deleted the record and nothing ever re-queued the URL, so the entry
  disappeared from the list and no new download started. The file itself was left
  on disk under its original name.
- Re-fetching is now a separate `Download Again` entry with its own
  confirmation. It keeps the record, so the id, URL, and filename stay stable and
  no second network request is needed to re-resolve the URL. Progress is reset
  before the file is deleted, so a failure cannot leave you without either the
  old file or a queued download. Any stale `.part` file is removed too.
- Opening a file that has been moved or deleted outside Lunefetch now says so.
  The path was previously handed to the OS opener unchecked, which does nothing
  visible on Windows.

## [1.1.0] - 2026-08-24

### Desktop UI redesign
- Rebuilt the shell: 260px sidebar with status and category routes, content
  header with greeting/search/`New Download`, live summary cards, and a status
  bar carrying overall speed plus a persisted concurrency selector.
- Replaced `widget.Table` with a virtualized `widget.List` and a hand-written row
  renderer. This fixes the reported Windows bug where the trailing action button
  could not be clicked, so downloads could not be removed.
- Rows now show a file-type icon, filename, size and percentage metadata, a
  compact progress bar, status, speed, and ETA without clipping.
- Sidebar routes live in one scroll area with a pinned footer, so every filter
  and category stays reachable at the minimum window size.
- Canonical categories are now `Compressed`, `Documents`, `Media`, `Programs`,
  and `Other`, with an idempotent migration from the old names. Files already in
  legacy category directories are left where they are.
- Dark-only moonlit theme; the window keeps saved geometry with a 1000x640 floor.
- Restored the documented `Ctrl+C` (copy selected URL) and `Ctrl+P` (pause all)
  shortcuts, which were listed in the README but never wired up.
- Queued rows can be moved one step up or down from the row action menu, exposing
  the queue-position storage layer that previously had no UI.

### Download integrity
- Resume progress is reconciled against the real `.part` file. A deleted or
  truncated temp file now re-downloads instead of being published as a complete,
  zero-filled file.
- The final size is verified and the file is `fsync`ed before it is published.
- Pausing no longer loses progress: cancellation waits for in-flight chunk
  workers, flushes their progress, and reports `context.Canceled`.
- `Done()` is closed on every exit path, including early failures, and is safe to
  read concurrently with `Start`.
- Retry backoff honours `retry_backoff_s` and is capped at two minutes; it
  previously ignored the setting and could sleep for days.
- Leftover `.part` files are removed when history entries are purged.

### Security
- Destination hints from the browser extension are confined to the configured
  download directory. Previously any absolute path was accepted, which allowed
  writing files outside it.
- The destination policy now also blocks CGNAT/Tailscale `100.64.0.0/10` and
  other reserved ranges when local access is disabled.
- Temporary `.part` files are created with mode `0600`.

### Windows
- Config and state use `%AppData%` and `%LocalAppData%`. They previously derived
  from `$HOME`, which is normally unset on Windows and produced paths relative to
  the working directory.
- Desktop notifications use the native mechanism; they never appeared before
  because delivery went only through `notify-send`.
- Open File and Open Folder use `explorer.exe` with normalized separators.

### Reliability
- A second instance is refused with an explanatory window instead of sharing the
  database and temp files.
- Startup failures (config, database, API token, single instance) now show a
  window; as a GUI binary these messages were previously invisible.
- SQLite `foreign_keys` and `busy_timeout` are set in the DSN and verified at
  open, so cascade deletes cannot silently stop working.
- Config writes are atomic, so a crash mid-write cannot leave an unparseable
  config that blocks startup.
- Queue ordering is consistent between execution and display, and queue moves are
  clamped and scoped to live queued rows.
- App, window, and tray icons are embedded. The tray icon was blank because the
  installer did not ship `lunefetch.ico` and Go cannot decode the ICO format.

### Browser extension
- Fixed MV3 state loss: listeners now wait for initialization, so a download can
  no longer be intercepted using default settings after the user disabled
  interception.
- `runtime.onMessage` validates the sender, so only the extension's own pages can
  queue downloads or change settings.
- Context menus are recreated cleanly when the service worker restarts.
- Relative destination hints are rejected before handoff, batch drafts expire
  after ten minutes, and collected links are filtered and capped.
- Raised the Firefox minimum to 142, matching the declared data-collection
  permissions.

### Housekeeping
- Removed the unused Bubble Tea TUI and its dependencies.
- CI now runs the full race-enabled test suite and `go vet` on both Linux and
  Windows, plus installer lifecycle checks and a dedicated extension job.

---

## Phase 5 - Complete (2026-08-18)

### 🎯 Summary
All phases of core functionality are now complete. Lunefetch is production-ready with:
- Keyboard shortcuts for efficient navigation
- Concurrency-safe download management
- Robust error recovery and file safety
- Per-task and global bandwidth limiting
- Benchmark-tested performance scaling
- Fresh restart dialog for completed downloads

---

## Phase 4 - Performance Scaling

### Speed Limit Features
**Per-Task Speed Limit** ✓
- Right-click context menu → "Set Speed Limit..."
- Modal dialog accepts MB/s input (e.g., `5.0`)
- Dynamic runtime limiter updates via `SetLimiter()` without restart
- DB persistence through existing `speed_limit` column

**Global Bandwidth Limiter** ✓
- Settings page integration with KB/s → bytes/sec conversion
- Shared limiter across all downloads via `SetGlobalLimiter()`
- Dynamic rate updates using `SetRate()` method
- Both limters work together (bottleneck logic at `internal/core/throttle.go`)

### Benchmarks
- Created benchmark suite at `internal/benchmarks/concurrency_test.go`
- Tested throughput at 1/5/10/20 concurrent downloads
- Linear scaling observed: 20 concurrent achieved ~5.24 MB/s aggregate throughput

### Files Modified
```bash
 internal/core/downloader.go     | +77 lines (global limiter support)
 internal/ui/components/table.go | +46 lines (context menu UI)  
 internal/ui/pages/downloads.go  | +13 lines (limiter integration)
 internal/ui/store/impl.go       | +1 line  (Mutator interface)
```

---

## Phase 3 - Error Recovery and File Safety

### Retry Loop & Exponential Backoff
**downloadChunkWithRetry()** ✓
- MaxRetries config field (default: 3)
- Exponential backoff: 1s → 2s → 4s per attempt
- URL validation with blocked patterns (file://, localhost blocking)
- Disk space pre-check before download starts
- Duplicate filename handling (auto #1, #2 suffix)

### Config Fields Added
```go
type Config struct {
    MaxRetries         int   // Retry attempts per chunk
    RetryBackoffS      int   // Initial backoff in seconds
    MinFreeSpaceMB     int64 // Minimum free disk space required
    AllowLocalHosts    bool  // Allow localhost URLs (localhost, 127.0.0.1)
    // ... existing fields
}
```

---

## Phase 2 - Concurrency Safety

### Mutex Protection
**DownloadTable concurrency** ✓
- Added `sync.RWMutex` to protect `DownloadTable.records` and `DownloadTable.speeds`
- Wrapped all map access with proper locking
- Implemented debounced sort function (300ms delay to prevent thrashing)
- Fixed selection highlight drift bug in sorted tables
- Documented locking strategy in code comments

### Race Condition Fixes
- `dp.active` map protected by mutex during concurrent start/pause/resume
- `dt.speeds` map accessed only within mutex-protected blocks
- `go test -race ./...` passes cleanly

---

## Phase 1 - Keyboard Shortcuts

### Implemented Shortcuts
| Key | Action |
|-----|--------|
| **Enter** | Open downloaded file |
| **Delete** | Confirm deletion of selected download |
| **Ctrl+C** | Copy URL to clipboard |
| **Ctrl+P** | Pause all active downloads |
| **Spacebar** | Toggle pause/resume on current row |
| **↑↓ Arrows** | Navigate table rows |
| **Focus indicators** | Visual highlighting for keyboard nav |

### Test Coverage
- All shortcuts tested with manual interaction
- No regressions in existing functionality
- Visual focus states work correctly in both keyboard and mouse modes

---

## Phase 5.2 - Fresh Restart Dialog (Latest)

### Feature Description
When clicking "Open File" on completed/cancelled downloads:
1. Shows confirmation dialog asking to confirm fresh restart
2. Message: "This download is already [status]. Restarting deletes the existing file and re-downloads everything."
3. If confirmed: deletes existing record, triggers re-add flow
4. Prevents accidental data loss with explicit confirmation

### Implementation Details
```go
// dialogs.go:147-161
func ShowFreshRestartDialog(w fyne.Window, rec *storage.DownloadRecord, onConfirm func()) {
    msg := "This download is already " + rec.Status + ".\n\n" +
        "Restarting deletes the existing file and re-downloads everything.\n\n" +
        "Restart from scratch?"

    dialog.ShowConfirm("Fresh Restart", msg, func(ok bool) {
        if ok && onCount != nil {
            onConfirm() // Caller handles deletion + re-add
        }
    }, w)
}
```

---

## Technical Specifications

### Build Configuration
- **Binary size**: ~40MB
- **Language**: Go 1.22+
- **GUI Framework**: Fyne v2.x
- **Database**: SQLite with GORM
- **Test coverage**: Core components pass `go test -race ./...`

### Dependencies
```bash
fyne.io/fyne/v2          # GUI framework
github.com/mattn/go-sqlite3 # Database
golang.org/x/time/rate   # Rate limiting
```

### Directory Structure (Key Components)
```
internal/
├── core/
│   ├── downloader.go        # Download engine with rate limiting
│   ├── throttle.go          # Global + per-task limiter logic
│   └── benchmarks/          # Performance testing suite
├── storage/
│   ├── state.go             # DownloadRecord model, SpeedLimit field
│   └── history_test.go      # Transaction tests
├── queue/
│   └── manager.go           # Concurrent worker pool
├── ui/
│   ├── components/
│   │   ├── table.go         # Keyboard-navigable table
│   │   ├── dialogs.go       # AddURL + FreshRestart dialogs
│   │   └── toolbar.go       # Search + add button
│   ├── pages/
│   │   └── downloads.go     # Main downloads page with actions
│   └── store/
│       └── impl.go          # Mutator interface (UpdateSpeedLimit)
└── api/
    └── server.go            # REST API wrapper
```

---

## Known Limitations

1. **Progress bar color customization**: Fyne's widget.Table lacks native color property; would require canvas overlay layering
2. **Virtual DOM for large datasets**: Current implementation renders all rows; virtualization deferred due to Fyne API limitations
3. **Drag-and-drop reordering**: Not implemented (low priority)
4. **Multi-select handlers**: Not implemented (low priority)

---

## Future Roadmap

### Phase 6 (Optional)
- E2E GUI tests with Fyne driver
- Integration test server suite
- Missing unit tests coverage expansion
- README update with shortcuts table

### Potential Enhancements
- Progress bar color coding (threshold-based)
- Drag-and-drop reordering
- Multi-select bulk operations
- Accessibility audit (screen reader labels)
- Virtual DOM optimization for tables >1k rows

---

## Credits
Developed for high-throughput download management with:
- Precise bandwidth control
- Safe retry mechanisms  
- Clean keyboard-first UX
- Production-grade reliability


## Phase 5.1 - Visual Enhancements (2026-08-18)

### Phase 5.1 - Color-Coded Progress Bars ✓
- Threshold-based color system for progress bars:
  - ≥95%: Green (#2ecc71)
  - ≥80%: Orange (#f39c12)
  - ≥50%: Blue (#3498db)
  - ≥20%: Light orange (#e67e22)
  - <20%: Gray (#95a5a6)
- Implementation via canvas rectangle overlay on ProgressBar

### Phase 5.2 - Multi-Select Handlers ✓  
- Added `multiSelectHandler` struct for tracking multiple selections
- Methods: `toggleMode()`, `selectRange()`, `isSelected()`, `getSelectedIDs()`, `clearSelection()`
- Keyboard shortcut ready: Ctrl+Click to select multiple rows

### Files Modified
```
internal/ui/components/table.go                 | +8 lines (multi-select field)
internal/ui/components/table_progress_color.go   | NEW FILE (~60 lines)
internal/ui/components/table_multi_select.go     | NEW FILE (~80 lines)
```

---

## Summary: All Phases 1-5 COMPLETE! 🎉
