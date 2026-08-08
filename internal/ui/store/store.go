// Package store defines the Store interface — the single point of contact
// between the UI layer and the backend (Storage, Queue, Downloader).
//
// UI components must not import storage, queue, or core directly.
// All reads and mutations go through Store.
package store

import "github.com/lyravein/lunefetch/internal/storage"

// DownloadStatus is a typed constant for download status values.
// Using a named type instead of raw strings prevents typos and enables
// compile-time checking.
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

// String returns the underlying string value.
func (s DownloadStatus) String() string { return string(s) }

// TableColumn identifies a sortable column in the download table.
type TableColumn int

const (
	ColName     TableColumn = iota // filename
	ColSize                        // total size
	ColProgress                    // downloaded / total
	ColSpeed                       // current speed (active downloads only)
	ColStatus                      // status string
	ColAdded                       // created_at
)

// AddRequest contains everything needed to enqueue a new download.
// Mirrors ui.AddURLRequest — defined here so pages/components can depend on
// store without importing the ui root package.
type AddRequest struct {
	URL         string
	Filename    string // empty = derive from URL
	SaveDir     string // empty = use config default
	Category    string // empty = auto-detect
	SpeedLimit  int64  // bytes/sec, 0 = unlimited
	ScheduledAt string // "HH:MM", empty = start now
}

// Store is the facade between the UI and the backend.
// All UI mutations and reads go through this interface.
//
// Lifecycle:
//   - Load()   — called once at startup to populate internal state from DB.
//   - Reload() — force re-read from DB (e.g. after import/restore).
//
// Mutations (Pause, Resume, Cancel, Delete, Retry, Add) automatically
// refresh the internal state; the UI does not need to call Reload() after them.
type Store interface {
	// Lifecycle
	Load() error
	Reload() error

	// Read — returns the current filtered+sorted+searched view.
	Downloads() []*storage.DownloadRecord
	Selected() *storage.DownloadRecord

	// View state — filters are applied on the in-memory slice, not on DB.
	SetFilter(status DownloadStatus)
	SetCategory(category string)
	SetSearch(query string)
	SetSort(col TableColumn, asc bool)
	Select(id int64)

	// Mutations — each call updates internal state after the backend op.
	Add(req AddRequest)
	Pause(id int64)
	Resume(id int64)
	Cancel(id int64)
	Delete(id int64)
	Retry(id int64)
}
