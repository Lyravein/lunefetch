package pages

import (
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/storage"
)

// HistoryPage displays soft-deleted (completed/removed) downloads.
type HistoryPage struct {
	sm     *storage.StateManager
	window fyne.Window

	cnt     fyne.CanvasObject
	list    *widget.List
	records []storage.DownloadRecord
	mu      sync.RWMutex
}

// NewHistoryPage creates the history page.
func NewHistoryPage(sm *storage.StateManager, w fyne.Window) *HistoryPage {
	hp := &HistoryPage{sm: sm, window: w}

	hp.list = widget.NewList(
		func() int { return len(hp.records) },
		func() fyne.CanvasObject { return newHistoryRow() },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			hp.mu.RLock()
			if i >= len(hp.records) {
				hp.mu.RUnlock()
				return
			}
			rec := hp.records[i]
			hp.mu.RUnlock()
			hp.updateHistoryRow(o, rec)
		},
	)

	btnClearAll := widget.NewButton("Clear All History", func() {
		dialog.ShowConfirm("Clear History",
			"Permanently delete all history entries?",
			func(ok bool) {
				if ok {
					sm.PurgeAllDeleted() //nolint:errcheck
					hp.Refresh()
				}
			}, w)
	})

	// Auto-refresh every 2 seconds.
	StartTicker(2*time.Second, hp.Refresh)

	hp.cnt = container.NewBorder(nil, btnClearAll, nil, nil, hp.list)
	return hp
}

// Container returns the root canvas object for this page.
func (hp *HistoryPage) Container() fyne.CanvasObject { return hp.cnt }

// Refresh reloads deleted records from storage and redraws the list.
func (hp *HistoryPage) Refresh() {
	records, err := hp.sm.ListDeleted()
	if err != nil {
		return
	}
	hp.mu.Lock()
	hp.records = records
	hp.mu.Unlock()
	fyne.Do(func() { hp.list.Refresh() })
}

func newHistoryRow() fyne.CanvasObject {
	filename := widget.NewLabel("filename")
	filename.Truncation = fyne.TextTruncateEllipsis

	status := widget.NewLabel("status")
	size := widget.NewLabel("size")
	date := widget.NewLabel("date")
	btnRestore := widget.NewButton("Restore", nil)
	btnDelete := widget.NewButton("Delete", nil)

	right := container.NewHBox(size, date, btnRestore, btnDelete)
	return container.NewBorder(nil, nil, nil, right,
		container.NewVBox(filename, status),
	)
}

func (hp *HistoryPage) updateHistoryRow(o fyne.CanvasObject, rec storage.DownloadRecord) {
	border := o.(*fyne.Container)
	left := border.Objects[0].(*fyne.Container)
	right := border.Objects[1].(*fyne.Container)

	filenameLabel := left.Objects[0].(*widget.Label)
	statusLabel := left.Objects[1].(*widget.Label)
	sizeLabel := right.Objects[0].(*widget.Label)
	dateLabel := right.Objects[1].(*widget.Label)
	btnRestore := right.Objects[2].(*widget.Button)
	btnDelete := right.Objects[3].(*widget.Button)

	filenameLabel.SetText(rec.Filename)
	statusLabel.SetText(rec.Status)
	sizeLabel.SetText(FormatSize(rec.TotalSize))

	if rec.DeletedAt.Valid {
		dateLabel.SetText(rec.DeletedAt.Time.Format("02 Jan 2006"))
	} else {
		dateLabel.SetText(rec.CreatedAt.Format("02 Jan 2006"))
	}

	id := rec.ID
	btnRestore.OnTapped = func() {
		if err := hp.sm.RestoreDownload(id); err == nil {
			hp.Refresh()
		}
	}
	btnDelete.OnTapped = func() {
		dialog.ShowConfirm("Delete Entry",
			fmt.Sprintf("Permanently delete %q from history?", rec.Filename),
			func(ok bool) {
				if ok {
					hp.sm.PurgeDownload(id) //nolint:errcheck
					hp.Refresh()
				}
			}, hp.window)
	}
}
