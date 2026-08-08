# Lunefetch Architecture

## Overview

Lunefetch is a desktop HTTP download manager built with Go + Fyne v2.
The core download engine is stable. Current focus is **UI architecture and maintainability**, not new features.

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
Multi-chunk downloader, rate limiter, chunk calculation.
Single responsibility: download one file.

### Queue (`internal/queue/`)
Controls concurrency. Decides when a download starts.
Does not perform downloads.

### Storage (`internal/storage/`)
SQLite persistence. Downloads + chunks.
Soft-delete model — history is not a separate table, just `deleted_at IS NOT NULL`.

### Config (`internal/config/`)
YAML config. Loaded once at startup, saved on settings change.

### API (`internal/api/`)
HTTP server on port 7474 for browser extension (Firefox + Chromium).
Receives URL from extension, pushes to Store.

### Store (`internal/ui/store/`)
Single source of truth for the UI.
Orchestrates Queue, Storage, and Downloader.
Does NOT contain business logic — that stays in Core/Queue/Storage.

### UI (`internal/ui/`)
Reads state from Store. Calls Store methods for mutations.
No direct access to any backend layer.

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
    SetSearch(query string)
    SetSort(col TableColumn, asc bool)
    Select(id int64)

    // Mutations — Store updates its own state after each call
    Add(req AddURLRequest)
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
        components/     -- toolbar.go, sidebar.go, table.go, inspector.go, statusbar.go
        pages/          -- downloads.go, history.go
        layout/         -- desktop.go
```

Rule: if a component grows to 4+ files, extract it to its own subfolder (e.g. `components/table/`).

---

## UI Layout

```
┌────────────────────────────────────────────┐
│ Toolbar                                    │
├───────────────┬────────────────────────────┤
│ Sidebar 220px │ Download Table             │
│               │                            │
│ Downloads     │ Name│Size│Progress│Speed.. │
│ Active        │                            │
│ Paused        │                            │
│ Completed     │                            │
│ Failed        │                            │
│ History       │                            │
├───────────────┴────────────────────────────┤
│ Inspector (widget.Accordion, collapsible)  │
├────────────────────────────────────────────┤
│ Status Bar  ↓ speed │ N active │ N total   │
└────────────────────────────────────────────┘
```

- **Table**: `widget.Table`, not `widget.List` — data is tabular
- **Sidebar**: fixed 220px, not resizable — no need for drag in a download manager
- **Inspector**: `widget.Accordion` — collapsible detail panel
- **Status bar**: global speed + active count + total count

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
| Theme system         | Cosmetic; deferred to v1.2                          |
| Resizable sidebar    | Overkill for a download manager                     |

---

## Guiding Principles

1. UI only knows Store.
2. Store is a facade — not a place for business logic.
3. Business logic lives in Core, Queue, and Storage.
4. Refactor in phases — project must stay buildable and runnable after each phase.
5. Maintainability over feature count.
6. YAGNI — don't add complexity before there is a real need.
