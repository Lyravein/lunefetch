# Lunefetch Architecture

## Overview

Lunefetch is a desktop HTTP download manager built with Go + Fyne v2.
The download engine, desktop UI, and browser integration are all implemented as
of 1.1.0. Current focus is correctness and cross-platform parity, not new
features.

---

## Stack

| Layer    | Technology                  |
|----------|-----------------------------|
| Language | Go 1.26.5                   |
| GUI      | Fyne v2.8.0                 |
| Database | SQLite (modernc.org/sqlite) |
| Config   | YAML                        |
| Platform | Linux, Windows              |

---

## Overall Flow

```
Browser Extension
        │
        ▼
HTTP API (:7474)
        │
        ▼
      Store
        │
 ┌──────┼──────────────┐
 ▼      ▼              ▼
Queue  Storage      Downloader
        │
        ▼
     SQLite
```

**Rule: all UI communicates only with Store.**
UI must not know about Downloader, Queue, or Storage directly.

---

## Layers

### Core (`internal/core/`)
Multi-chunk downloader, rate limiter, chunk calculation, destination policy.
Single responsibility: download one file.

Integrity rules the downloader owns:
- Resume progress is reconciled against the real `.part` file, so a deleted or
  truncated file re-downloads instead of being published as complete.
- The final size is verified and the file is `Sync()`ed before the caller
  publishes it.
- `Done()` closes on every return path; cancellation waits for in-flight workers
  and flushes progress so a pause stays resumable.
- Destinations are screened before connecting (loopback, private, link-local,
  CGNAT/Tailscale `100.64/10`, and other reserved ranges), with cloud metadata
  addresses blocked even when local access is allowed.

### Queue (`internal/queue/`)
Controls concurrency. Decides when a download starts.
Does not perform downloads.

### Storage (`internal/storage/`)
SQLite persistence. Downloads + chunks.
Soft-delete model — history is not a separate table, just `deleted_at IS NOT NULL`.
`foreign_keys` and `busy_timeout` are set in the DSN and verified at open, so
cascade deletes cannot silently stop working. Queue moves are confined to live
queued rows and clamped to the compacted 1..N range.

### Config (`internal/config/`)
YAML config. Loaded once at startup, saved on settings change.
`Validate()` bounds every numeric field; `Save()` refuses to write an invalid config.

### User paths (`internal/userpath/`)
Resolves the config, data, and download directories so Linux and Windows both get
absolute, platform-correct paths. Nothing else in the tree should read `$HOME`.

| | Linux | Windows |
|---|---|---|
| Config, API token | `~/.config/lunefetch` | `%AppData%\lunefetch` |
| Database, lock | `$XDG_DATA_HOME` or `~/.local/share/lunefetch` | `%LocalAppData%\lunefetch` |

### API (`internal/api/`)
HTTP server on 127.0.0.1:7474 for the browser extension (Firefox + Chromium).
Bearer-token authenticated, JSON-only, body-capped. It receives a URL and
optional hints and pushes them to the Store.

Destination hints from the extension are untrusted: `main.go` passes them through
`core.SafeSaveDir` so a caller can only redirect a download *within*
`cfg.DownloadDir`, never to an arbitrary absolute path.

### Single instance (`internal/singleinstance/`)
An advisory file lock (flock on unix, LockFileEx on Windows) taken in
`main.go` before the database is opened. The OS releases it on process death, so
a crash never leaves a stale lock.

### Notifications (`internal/notify/`)
Prefers Fyne's `SendNotification`, which is native on Windows and macOS.
`notify-send` is a Linux-only fallback for the case where no Fyne app is running.

### Store (`internal/ui/store/`)
Single source of truth for the UI.
Orchestrates Queue, Storage, and Downloader.
Does NOT contain business logic — that stays in Core/Queue/Storage.

### UI (`internal/ui/`)
Reads state from Store. Calls Store methods for mutations.
No direct access to any backend layer.

Supporting packages:
- `assets/` — `AppIcon`, a PNG embedded with `go:embed`, used for the window,
  taskbar, and tray. The old runtime load of `lunefetch.ico` failed twice over:
  the Linux installer does not ship the .ico beside the binary, and Go cannot
  decode the ICO container. `lunefetch.ico` is still used by the Windows
  installer for shortcuts.
- `fatal/` — startup failures raised before the main window exists (config,
  single instance, database, API token) are shown in a small fixed-size window
  rather than only logged, since a GUI binary has no attached terminal.
- `theme/` — the dark-only moonlit palette and spacing/radius tokens.

---

## Store Interface

```go
type DownloadStatus string

const (
    StatusAll         DownloadStatus = ""
    StatusPending     DownloadStatus = "pending"
    StatusDownloading DownloadStatus = "downloading"
    StatusPaused      DownloadStatus = "paused"
    StatusCompleted   DownloadStatus = "completed"
    StatusFailed      DownloadStatus = "failed"
    StatusCancelled   DownloadStatus = "cancelled"
    StatusQueued      DownloadStatus = "queued"
    StatusScheduled   DownloadStatus = "scheduled"
)

type TableColumn int

const (
    ColName TableColumn = iota
    ColSize
    ColProgress
    ColSpeed
    ColStatus
    ColAdded
)

type Store interface {
    // Lifecycle
    Load() error    // initial load at startup
    Reload() error  // force reload from DB (e.g. after import)

    // Read
    Downloads() []*storage.DownloadRecord
    Selected() *storage.DownloadRecord

    // View state
    SetFilter(status DownloadStatus)
    SetCategory(category string) // "" = the UI-only All category
    SetSearch(query string)
    SetSort(col TableColumn, asc bool)
    Select(id int64)

    // Mutations — Store updates its own state after each call
    Add(req AddRequest)
    Pause(id int64)
    Resume(id int64)
    Cancel(id int64)
    Delete(id int64)
    Retry(id int64)
}
```

Key decisions:
- `DownloadStatus` is a typed constant, not a raw string — compile-time safety, no typos
- `TableColumn` is an enum — same reason
- `Refresh()` is not exposed; mutations auto-update internal state
- `Load()` is for startup; `Reload()` is for explicit force-reload

---

## Folder Structure

```
internal/
    ui/
        store/          -- Store interface + implementation
        components/     -- header.go, sidebar.go, table.go, statusbar.go, dialogs.go, about.go
        pages/          -- downloads.go, history.go
        layout/         -- desktop.go
        theme/          -- navy.go
        assets/         -- embedded icon
        fatal/          -- startup-error window
```

Rule: if a component grows to 4+ files, extract it to its own subfolder (e.g. `components/table/`).

---

## Desktop UI Layout

```
┌────────────────────────────────────────────┐
│ Header: greeting, search, New Download     │
│ Summary cards                               │
├───────────────┬────────────────────────────┤
│ Sidebar 260px │ Virtualized Download List  │
│               │ Name/status/progress/speed │
│ Filters       │ Inline row actions + ETA   │
│ Categories    │                            │
│ Footer routes │                            │
├───────────────┴────────────────────────────┤
│ Status Bar  overall speed │ concurrency    │
└────────────────────────────────────────────┘
```

- **Download list**: `widget.List` with controlled custom rows, not `widget.Table`.
- **Sidebar**: fixed 260px minimum surface; it is not independently resizable.
- **Row details**: filename, size, status, progress, speed, and ETA stay inline;
  the former inspector/detail panel is removed.
- **Status bar**: overall speed plus a persisted concurrency selector.

---

## Refresh Strategy

Polling every 500ms via `fyne.Do`. Not event-driven.

Rationale: multiple chunks emit progress in parallel. Event-driven would spam the UI.
500ms is sufficient for a download manager. Optimize only if a real bottleneck appears (YAGNI).

---

## Decisions Not Taken

| Decision             | Reason skipped                                      |
|----------------------|-----------------------------------------------------|
| Repository layer     | Storage is already clean; pass-through adds no value |
| Event-driven refresh | Fyne has no mature reactive system; polling is fine  |
| Theme system         | Desktop uses the locked dark-only moonlit theme; light/preset modes remain deferred |
| Resizable sidebar    | Overkill for a download manager                     |
| SQLite WAL mode      | The driver creates world-readable `-wal`/`-shm` sidecars, defeating the 0600 database |

---

## Guiding Principles

1. UI only knows Store.
2. Store is a facade — not a place for business logic.
3. Business logic lives in Core, Queue, and Storage.
4. Refactor in phases — project must stay buildable and runnable after each phase.
5. Maintainability over feature count.
6. YAGNI — don't add complexity before there is a real need.
