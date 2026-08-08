package components

import (
	"errors"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/core"
	"github.com/lyravein/lunefetch/internal/queue"
	"github.com/lyravein/lunefetch/internal/storage"
)

// DownloadsActions is the minimal interface toolbar needs from the downloads page.
type DownloadsActions interface {
	PauseDownload(id int64)
	ResumeDownload(id int64)
}

// Toolbar bundles the toolbar canvas object with references needed by
// keyboard shortcuts (search entry focus, add-dialog trigger).
type Toolbar struct {
	Root   fyne.CanvasObject
	Search *widget.Entry
	pause  *widget.Button
	resume *widget.Button
}

// NewToolbarFull creates the toolbar: Add URL + Pause/Resume (acting on the
// selected download) on the left, search in the middle, Settings on the right.
// selectedFn must return the currently selected record (nil if none).
func NewToolbarFull(
	w fyne.Window,
	cfg *config.Config,
	globalLimiter **core.Limiter,
	qm *queue.Manager,
	selectedFn func() *storage.DownloadRecord,
	dl DownloadsActions,
	addCh chan<- AddURLRequest,
	onSearch func(query string),
) *Toolbar {
	btnAdd := widget.NewButtonWithIcon("Add download", theme.ContentAddIcon(), func() {
		ShowAddURLDialog(w, cfg, addCh)
	})
	btnAdd.Importance = widget.HighImportance

	btnPause := widget.NewButtonWithIcon("Pause", theme.MediaPauseIcon(), func() {
		if sel := selectedFn(); sel != nil && sel.Status == "downloading" {
			dl.PauseDownload(sel.ID)
		}
	})
	btnPause.Importance = widget.LowImportance

	btnResume := widget.NewButtonWithIcon("Resume", theme.MediaPlayIcon(), func() {
		if sel := selectedFn(); sel != nil {
			switch sel.Status {
			case "paused", "failed", "cancelled", "queued":
				dl.ResumeDownload(sel.ID)
			}
		}
	})
	btnResume.Importance = widget.LowImportance

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search downloads")
	searchEntry.OnChanged = func(q string) {
		if onSearch != nil {
			onSearch(q)
		}
	}

	// Keep search compact and aligned with the actions instead of floating in
	// the middle of a wide window.
	searchBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(320, 32)), searchEntry)

	btnSettings := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		ShowSettingsDialog(w, cfg, globalLimiter, qm)
	})
	btnSettings.Importance = widget.LowImportance

	left := container.NewHBox(btnAdd, widget.NewSeparator(), btnPause, btnResume)
	right := container.NewHBox(btnSettings)
	searchArea := container.NewBorder(nil, nil, searchBox, nil, layout.NewSpacer())
	content := container.NewBorder(nil, nil, left, right, searchArea)
	background := canvas.NewRectangle(theme.Color(theme.ColorNameHeaderBackground))
	return &Toolbar{
		Root: container.NewStack(
			background,
			container.New(layout.NewCustomPaddedLayout(8, 8, 12, 12), content),
		),
		Search: searchEntry,
		pause:  btnPause,
		resume: btnResume,
	}
}

// UpdateSelection updates action affordances for the current table selection.
func (tb *Toolbar) UpdateSelection(rec *storage.DownloadRecord) {
	status := ""
	if rec != nil {
		status = rec.Status
	}
	fyne.Do(func() {
		if status == "" {
			tb.pause.Disable()
			tb.resume.Disable()
			return
		}
		tb.pause.Enable()
		tb.resume.Enable()
		if status != "downloading" {
			tb.pause.Disable()
		}
		switch status {
		case "paused", "failed", "cancelled", "queued", "scheduled", "pending":
		default:
			tb.resume.Disable()
		}
	})
}

// SetDownloadsActive disables controls whose effects belong to the downloads
// route while another page is visible.
func (tb *Toolbar) SetDownloadsActive(active bool) {
	fyne.Do(func() {
		if active {
			tb.Search.Enable()
			return
		}
		tb.Search.Disable()
		tb.pause.Disable()
		tb.resume.Disable()
	})
}

// ShowError displays an actionable in-window error dialog.
func ShowError(w fyne.Window, msg string) {
	fyne.Do(func() { dialog.ShowError(errors.New(msg), w) })
}

// FormatSize formats bytes into a human-readable string.
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
