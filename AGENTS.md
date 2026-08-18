# Lunefetch Agent Guidelines

This document provides guidance for AI agents working on Lunefetch codebase.

## Quick Reference

### Build & Run
```bash
go build -o lunefetch .          # Build binary
./lunefetch                       # Run GUI application
go test ./... -race -v           # Run tests with race detection
```

### Core Files to Know
- `internal/core/downloader.go` - Download engine with rate limiting
- `internal/core/throttle.go` - Global + per-task limiter logic  
- `internal/ui/components/table.go` - Keyboard-navigable table
- `internal/ui/components/dialogs.go` - AddURL, FreshRestart dialogs
- `internal/ui/pages/downloads.go` - Main downloads page
- `internal/storage/state.go` - DownloadRecord model

## Phase 4 Features (Completed)

### Per-Task Speed Limit
**Location**: `internal/ui/components/table.go:471-500`
- Right-click context menu item "Set Speed Limit..."
- Modal dialog accepts MB/s input
- Persists to DB via `UpdateSpeedLimit()`
- Dynamic runtime update via `SetLimiter()`

**Key method**: `dp.UpdateSpeedLimit(id, speedLimit int64)` in `downloads.go:582-592`

### Global Bandwidth Limiter  
**Location**: `internal/config/config.go:32`, `internal/core/throttle.go`
- Settings page handles KB/s → bytes/sec conversion
- Shared limiter across all downloads
- Updated dynamically using `SetRate()`

### Benchmarks
**Location**: `internal/benchmarks/concurrency_test.go`
- Tested at 1/5/10/20 concurrent downloads
- Linear scaling: 20 concurrent → ~5.24 MB/s aggregate throughput

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
        rec, _ := dp.sm.GetDownload(id)
        // Check status for fresh restart dialog
        if rec.Status == "completed" || rec.Status == "cancelled":
            components.ShowFreshRestartDialog(dp.window, rec, func() {
                dp.DeleteDownload(id)
            })
            return
        dp.openFile(id)
    }
}
```

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

### Benchmark Tests
```bash
go test ./internal/benchmarks -bench=. -benchmem
```

Focus areas:
- Throughput at different concurrency levels
- Memory usage under load
- Rate limiter overhead

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

### 4. Don't Bypass Validator
Always call `core.ValidateURL()` before adding download to ensure safety.

### 5. Don't Ignore Race Detection
Run `go test -race ./...` regularly. Any race condition should be fixed immediately.

## Performance Tips

### For Large Datasets (>1k rows)
Current limitation: Table renders all rows. For virtual DOM optimization:
- Track scroll position
- Only render visible rows + buffer (±20 rows)
- Fyne's widget.Table lacks native OnScroll callback (requires custom renderer)

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
    Category:    "Videos",           // optional, auto-detected
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

## Future Enhancements (Phase 6)

### Priority Order
1. E2E GUI tests (Fyne driver)
2. Integration test server suite
3. Missing unit tests coverage
4. README shortcuts table
5. Accessibility audit

### Deferred Items
- Progress bar color coding
- Drag-and-drop reordering
- Multi-select bulk operations
- Virtual DOM optimization

---

Last updated: 2026-08-18 (Phase 5 complete)
