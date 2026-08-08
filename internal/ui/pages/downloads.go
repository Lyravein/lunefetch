// Package pages contains full-page UI views.
package pages

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/core"
	"github.com/lyravein/lunefetch/internal/notify"
	"github.com/lyravein/lunefetch/internal/queue"
	"github.com/lyravein/lunefetch/internal/storage"
	"github.com/lyravein/lunefetch/internal/ui/components"
	"github.com/lyravein/lunefetch/internal/ui/store"
)

// downloadEntry tracks a running downloader and its cancel function.
type downloadEntry struct {
	id         int64
	downloader *core.Downloader
	cancelFn   context.CancelFunc
	mu         sync.Mutex
}

// DownloadsPage is the main download list view.
type DownloadsPage struct {
	sm            *storage.StateManager
	cfg           *config.Config
	globalLimiter **core.Limiter
	qm            *queue.Manager
	st            *store.DownloadStore
	window        fyne.Window
	notifier      *notify.Notifier

	cnt                fyne.CanvasObject
	table              *components.DownloadTable
	emptyState         fyne.CanvasObject
	stack              *fyne.Container // stack: shows table or emptyState
	details            *fyne.Container
	detailName         *widget.Label
	detailMeta         *widget.Label
	detailURL          *widget.Label
	onSelectionChanged func(*storage.DownloadRecord)

	active       map[int64]*downloadEntry
	pending      map[int64]struct{}
	mu           sync.RWMutex
	workers      sync.WaitGroup
	shuttingDown bool
}

// NewDownloadsPage creates the downloads page.
func NewDownloadsPage(sm *storage.StateManager, cfg *config.Config, globalLimiter **core.Limiter, st *store.DownloadStore) *DownloadsPage {
	dp := &DownloadsPage{
		sm:            sm,
		cfg:           cfg,
		globalLimiter: globalLimiter,
		st:            st,
		active:        make(map[int64]*downloadEntry),
		pending:       make(map[int64]struct{}),
	}

	dp.detailName = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	dp.detailName.Truncation = fyne.TextTruncateEllipsis
	dp.detailMeta = widget.NewLabel("")
	dp.detailMeta.Importance = widget.LowImportance
	dp.detailURL = widget.NewLabel("")
	dp.detailURL.Importance = widget.LowImportance
	dp.detailURL.Truncation = fyne.TextTruncateEllipsis
	detailContent := container.NewVBox(dp.detailName, dp.detailMeta, dp.detailURL)
	detailPanel := container.NewBorder(widget.NewSeparator(), nil, nil, nil, container.NewPadded(detailContent))
	detailSize := canvas.NewRectangle(color.Transparent)
	detailSize.SetMinSize(fyne.NewSize(1, 82))
	dp.details = container.NewStack(detailSize, detailPanel)

	dp.table = components.NewDownloadTable(
		func(col store.TableColumn, asc bool) {
			st.SetSort(col, asc)
			dp.Refresh()
		},
		func(id int64) {
			st.Select(id)
			selected := st.Selected()
			dp.updateDetails(selected)
			if dp.onSelectionChanged != nil {
				dp.onSelectionChanged(selected)
			}
		},
		func(id int64, action string) {
			switch action {
			case "pause":
				dp.PauseDownload(id)
			case "resume":
				dp.ResumeDownload(id)
			case "cancel":
				dialog.ShowConfirm("Cancel Download", "Cancel this download?",
					func(ok bool) {
						if ok {
							dp.CancelDownload(id)
						}
					}, dp.window)
			case "delete":
				dialog.ShowConfirm("Remove Download", "Remove this download from the list?",
					func(ok bool) {
						if ok {
							dp.DeleteDownload(id)
						}
					}, dp.window)
			case "open_folder":
				dp.openFolder(id)
			case "open_file":
				dp.openFile(id)
			}
		},
	)

	// Empty state shown when no downloads exist.
	emptyIcon := container.New(layout.NewGridWrapLayout(fyne.NewSize(42, 42)), widget.NewIcon(theme.DownloadIcon()))
	emptyHint := widget.NewLabel("Add a download URL to get started")
	emptyHint.Importance = widget.LowImportance
	dp.emptyState = container.NewCenter(container.NewVBox(
		container.NewCenter(emptyIcon),
		container.NewCenter(widget.NewLabelWithStyle(
			"No downloads yet",
			fyne.TextAlignCenter,
			fyne.TextStyle{Bold: true},
		)),
		container.NewCenter(emptyHint),
	))

	dp.stack = container.NewStack(dp.emptyState)
	dp.cnt = container.NewBorder(nil, dp.details, nil, nil, dp.stack)
	return dp
}

func (dp *DownloadsPage) updateDetails(rec *storage.DownloadRecord) {
	fyne.Do(func() {
		if rec == nil {
			dp.detailName.SetText("Select a download to see its destination")
			dp.detailMeta.SetText("")
			dp.detailURL.SetText("")
			return
		}

		progress := "Unknown size"
		if rec.TotalSize > 0 {
			pct := float64(rec.DownloadedSize) / float64(rec.TotalSize) * 100
			progress = fmt.Sprintf("%.0f%% of %s", pct, components.FormatSize(rec.TotalSize))
		}
		dir := rec.SaveDir
		if dir == "" {
			dir = dp.cfg.DownloadDir
		}

		dp.detailName.SetText(rec.Filename)
		dp.detailMeta.SetText(fmt.Sprintf("%s  ·  %s", progress, dir))
		dp.detailURL.SetText(rec.URL)
	})
}

// SetQueueManager injects the queue manager after construction.
func (dp *DownloadsPage) SetQueueManager(qm *queue.Manager) {
	dp.qm = qm
}

// SetWindow injects the parent window (needed for dialogs).
func (dp *DownloadsPage) SetWindow(w fyne.Window) {
	dp.window = w
	dp.table.SetWindow(w)
}

// SetOnSelectionChanged registers a listener for toolbar action state.
func (dp *DownloadsPage) SetOnSelectionChanged(fn func(*storage.DownloadRecord)) {
	dp.onSelectionChanged = fn
}

// SetNotifier injects the optional desktop notification sender.
func (dp *DownloadsPage) SetNotifier(n *notify.Notifier) {
	dp.notifier = n
}

// openFolder opens the save directory of a download in the system file manager.
func (dp *DownloadsPage) openFolder(id int64) {
	rec, err := dp.sm.GetDownload(id)
	if err != nil || rec == nil {
		return
	}
	dir := rec.SaveDir
	if dir == "" {
		dir = dp.cfg.DownloadDir
	}
	openSystemPath(dir)
}

// openFile opens a completed download with the default system application.
func (dp *DownloadsPage) openFile(id int64) {
	rec, err := dp.sm.GetDownload(id)
	if err != nil || rec == nil {
		return
	}
	dir := rec.SaveDir
	if dir == "" {
		dir = dp.cfg.DownloadDir
	}
	openSystemPath(filepath.Join(dir, rec.Filename))
}

// openSystemPath opens a path with the OS default handler.
func openSystemPath(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}

// Container returns the root canvas object for this page.
// DownloadsPage owns only the table + empty state; toolbar lives in desktop.go.
func (dp *DownloadsPage) Container() fyne.CanvasObject { return dp.cnt }

// SetTableWidth lets the desktop layout keep the table columns responsive.
func (dp *DownloadsPage) SetTableWidth(width float32) {
	dp.table.SetAvailableWidth(components.ContentWidth(width))
}

// Refresh reads the current view from Store and redraws the table.
func (dp *DownloadsPage) Refresh() {
	records := dp.st.Downloads()

	dp.mu.RLock()
	speeds := make(map[int64]float64, len(dp.active))
	for id, entry := range dp.active {
		speeds[id] = entry.downloader.GetSpeed()
	}
	dp.mu.RUnlock()

	dp.table.SetRecords(records)
	dp.table.SetSpeeds(speeds)
	selected := dp.st.Selected()
	dp.updateDetails(selected)
	if dp.onSelectionChanged != nil {
		dp.onSelectionChanged(selected)
	}

	// Swap between empty state and table.
	fyne.Do(func() {
		if len(records) == 0 {
			dp.stack.Objects = []fyne.CanvasObject{dp.emptyState}
		} else {
			dp.stack.Objects = []fyne.CanvasObject{dp.table.Widget()}
		}
		dp.stack.Refresh()
	})
}

// StartDownload is called by the queue manager to start a download by ID.
func (dp *DownloadsPage) StartDownload(id int64) {
	rec, err := dp.sm.GetDownload(id)
	if err != nil || rec == nil {
		dp.qm.OnDone(id)
		return
	}

	chunks, err := dp.sm.GetChunks(id)
	if err != nil || len(chunks) == 0 {
		dp.sm.UpdateDownloadStatus(id, "failed") //nolint:errcheck
		dp.qm.OnDone(id)
		return
	}

	chunkDefs := make([]core.Chunk, len(chunks))
	for i, c := range chunks {
		chunkDefs[i] = core.Chunk{
			Index:      c.ChunkIndex,
			Start:      c.StartByte,
			End:        c.EndByte,
			Downloaded: c.DownloadedSize,
		}
	}

	saveDir := rec.SaveDir
	if saveDir == "" {
		saveDir = dp.cfg.DownloadDir
	}
	destPath, err := core.SafeDownloadPath(saveDir, rec.Filename)
	if err != nil {
		dp.sm.UpdateDownloadStatus(id, "failed") //nolint:errcheck
		dp.qm.OnDone(id)
		return
	}
	tmpPath := filepath.Join(saveDir, fmt.Sprintf(".lunefetch-%d.part", id))

	d := core.NewDownloader(rec.URL, tmpPath, rec.TotalSize, chunkDefs, rec.NumChunks, dp.cfg.MaxRetries)
	d.SetValidators(rec.ETag.String, rec.LastModified.String)
	d.SetProgressCallback(func(chunkIndex int, downloaded int64, status string) {
		dp.sm.UpdateChunkProgress(id, chunkIndex, downloaded, status) //nolint:errcheck
	})
	if dp.cfg.AllowLocalHosts {
		d.SetAllowLocalHosts(true)
	}
	if dp.cfg.ProxyURL != "" {
		d.SetProxy(dp.cfg.ProxyURL)
	}
	if dp.globalLimiter != nil && *dp.globalLimiter != nil {
		d.SetGlobalLimiter(*dp.globalLimiter)
	}
	if rec.SpeedLimit > 0 {
		d.SetLimiter(core.NewLimiter(rec.SpeedLimit))
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &downloadEntry{id: id, downloader: d, cancelFn: cancel}

	dp.mu.Lock()
	if dp.shuttingDown {
		dp.mu.Unlock()
		cancel()
		dp.qm.OnDone(id)
		return
	}
	if _, exists := dp.active[id]; exists {
		dp.mu.Unlock()
		cancel()
		dp.qm.OnDone(id)
		return
	}
	dp.active[id] = entry
	dp.workers.Add(1)
	dp.mu.Unlock()

	dp.sm.UpdateDownloadStatus(id, "downloading") //nolint:errcheck

	go func() {
		defer dp.workers.Done()
		err := d.Start(ctx)

		dp.mu.Lock()
		if dp.active[id] == entry {
			delete(dp.active, id)
		}
		dp.mu.Unlock()

		if err != nil && ctx.Err() == nil {
			dp.sm.UpdateDownloadStatus(id, "failed") //nolint:errcheck
			dp.qm.OnDone(id)
			return
		}
		if ctx.Err() != nil {
			dp.qm.OnDone(id)
			return
		}

		dp.finalize(id, tmpPath, destPath, rec.Filename)
	}()
}

// finalize moves the finished temp file into place. When the destination is
// already taken the user decides; the download stays "downloading" until then
// so the temp file is never silently discarded.
func (dp *DownloadsPage) finalize(id int64, tmpPath, destPath, filename string) {
	err := core.MoveFile(tmpPath, destPath)
	if err == nil {
		dp.sm.UpdateDownloadStatus(id, "completed") //nolint:errcheck
		if dp.notifier != nil {
			dp.notifier.Send("Download complete", filename)
		}
		dp.qm.OnDone(id)
		return
	}

	if !errors.Is(err, core.ErrDestinationExists) {
		dp.sm.UpdateDownloadStatus(id, "failed") //nolint:errcheck
		dp.qm.OnDone(id)
		return
	}

	// During shutdown there is nobody to answer the prompt. Keep the temp file
	// and leave the record resumable rather than blocking the exit path.
	dp.mu.Lock()
	closing := dp.shuttingDown
	if !closing && dp.window != nil {
		dp.pending[id] = struct{}{}
	}
	dp.mu.Unlock()
	if closing || dp.window == nil {
		dp.sm.UpdateDownloadStatus(id, "paused") //nolint:errcheck
		dp.qm.OnDone(id)
		return
	}
	dp.promptConflict(id, tmpPath, destPath, filename)
}

// promptConflict offers Keep Both / Replace / Cancel. It releases the queue slot
// as soon as the choice is made, not while the dialog is open.
func (dp *DownloadsPage) promptConflict(id int64, tmpPath, destPath, filename string) {
	claim := func() bool {
		dp.mu.Lock()
		defer dp.mu.Unlock()
		if _, ok := dp.pending[id]; !ok {
			return false
		}
		delete(dp.pending, id)
		return true
	}
	resolve := func(finalPath string, replace bool) {
		if !claim() {
			return
		}
		var err error
		switch {
		case replace:
			err = core.ReplaceFile(tmpPath, finalPath)
		default:
			err = core.MoveFile(tmpPath, finalPath)
		}
		if err != nil {
			components.ShowError(dp.window, fmt.Sprintf("Failed to save file:\n%v", err))
			dp.sm.UpdateDownloadStatus(id, "failed") //nolint:errcheck
			dp.qm.OnDone(id)
			return
		}
		if base := filepath.Base(finalPath); base != filename {
			dp.sm.UpdateDownloadFilename(id, base) //nolint:errcheck
		}
		dp.sm.UpdateDownloadStatus(id, "completed") //nolint:errcheck
		if dp.notifier != nil {
			dp.notifier.Send("Download complete", filepath.Base(finalPath))
		}
		dp.qm.OnDone(id)
		dp.Refresh()
	}

	fyne.Do(func() {
		dp.mu.RLock()
		closing := dp.shuttingDown
		dp.mu.RUnlock()
		if closing {
			return
		}
		msg := widget.NewLabel(fmt.Sprintf("%q already exists in this folder.\nWhat would you like to do?", filename))
		msg.Wrapping = fyne.TextWrapWord

		var d dialog.Dialog
		keepBoth := widget.NewButton("Keep Both", func() {
			d.Hide()
			unique, err := core.UniqueDownloadPath(destPath)
			if err != nil {
				if !claim() {
					return
				}
				components.ShowError(dp.window, fmt.Sprintf("Failed to pick a new name:\n%v", err))
				dp.sm.UpdateDownloadStatus(id, "failed") //nolint:errcheck
				dp.qm.OnDone(id)
				return
			}
			resolve(unique, false)
		})
		keepBoth.Importance = widget.HighImportance

		replace := widget.NewButton("Replace", func() {
			d.Hide()
			resolve(destPath, true)
		})
		replace.Importance = widget.DangerImportance

		cancelBtn := widget.NewButton("Cancel", func() {
			d.Hide()
			if !claim() {
				return
			}
			// Discard the downloaded bytes and mark the attempt cancelled.
			core.CleanupFile(tmpPath)                   //nolint:errcheck
			dp.sm.UpdateDownloadStatus(id, "cancelled") //nolint:errcheck
			dp.qm.OnDone(id)
			dp.Refresh()
		})

		content := container.NewVBox(
			msg,
			container.NewHBox(layout.NewSpacer(), cancelBtn, replace, keepBoth),
		)
		d = dialog.NewCustomWithoutButtons("File already exists", content, dp.window)
		d.Resize(fyne.NewSize(420, 180))
		d.Show()
	})
}

// Shutdown stops accepting workers, cancels active downloads, persists them as
// resumable, and waits until every downloader has flushed its progress.
func (dp *DownloadsPage) Shutdown() {
	dp.mu.Lock()
	if dp.shuttingDown {
		dp.mu.Unlock()
		dp.workers.Wait()
		return
	}
	dp.shuttingDown = true
	entries := make([]*downloadEntry, 0, len(dp.active))
	for _, entry := range dp.active {
		entries = append(entries, entry)
	}
	pending := make([]int64, 0, len(dp.pending))
	for id := range dp.pending {
		pending = append(pending, id)
		delete(dp.pending, id)
	}
	dp.mu.Unlock()

	for _, entry := range entries {
		entry.mu.Lock()
		entry.cancelFn()
		entry.mu.Unlock()
		dp.sm.UpdateDownloadStatus(entry.id, "paused") //nolint:errcheck
	}
	for _, id := range pending {
		dp.sm.UpdateDownloadStatus(id, "paused") //nolint:errcheck
		dp.qm.OnDone(id)
	}
	dp.workers.Wait()
}

// PauseDownload stops a running download.
func (dp *DownloadsPage) PauseDownload(id int64) {
	dp.mu.RLock()
	entry, ok := dp.active[id]
	dp.mu.RUnlock()
	if !ok {
		return
	}
	entry.mu.Lock()
	entry.cancelFn()
	entry.mu.Unlock()
	dp.sm.UpdateDownloadStatus(id, "paused") //nolint:errcheck
}

// ResumeDownload re-enqueues a paused download.
func (dp *DownloadsPage) ResumeDownload(id int64) {
	dp.qm.TryStart(id) //nolint:errcheck
}

// CancelDownload cancels a download and sets status to cancelled.
func (dp *DownloadsPage) CancelDownload(id int64) {
	dp.mu.RLock()
	entry, ok := dp.active[id]
	dp.mu.RUnlock()
	if ok {
		entry.mu.Lock()
		entry.cancelFn()
		entry.mu.Unlock()
	}
	dp.sm.UpdateDownloadStatus(id, "cancelled") //nolint:errcheck
}

// DeleteDownload soft-deletes a download to history.
func (dp *DownloadsPage) DeleteDownload(id int64) {
	dp.CancelDownload(id)
	dp.qm.Remove(id)
	dp.sm.DeleteDownload(id) //nolint:errcheck
	dp.Refresh()
}

// SpeedTotal returns the combined download speed across all active downloaders.
func (dp *DownloadsPage) SpeedTotal() float64 {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	var total float64
	for _, entry := range dp.active {
		total += entry.downloader.GetSpeed()
	}
	return total
}

// ActiveCount returns the number of currently active (downloading) downloads.
func (dp *DownloadsPage) ActiveCount() int {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	return len(dp.active)
}

// FormatSize formats bytes into a human-readable string.
// Re-exported from components to avoid import cycles in pages that need it.
func FormatSize(bytes int64) string {
	return components.FormatSize(bytes)
}

// StartTicker starts a background ticker that calls fn at the given interval.
func StartTicker(interval time.Duration, fn func()) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			fn()
		}
	}()
}
