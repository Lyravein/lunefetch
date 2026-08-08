package components

import (
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/filecat"
)

// AddURLRequest contains everything needed to start a new download.
type AddURLRequest struct {
	URL         string
	Filename    string // empty = derive from URL
	SaveDir     string // empty = use config default
	Category    string // empty = auto-detect
	SpeedLimit  int64  // bytes/sec, 0 = unlimited
	ScheduledAt string // "HH:MM", empty = start now
}

// ShowAddURLDialog displays the Add Download dialog.
func ShowAddURLDialog(w fyne.Window, cfg *config.Config, addCh chan<- AddURLRequest) {
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://example.com/file.zip")

	filenameEntry := widget.NewEntry()
	filenameEntry.SetPlaceHolder("(auto from URL)")

	categoryLabel := widget.NewLabel("Category: —")

	saveDirEntry := widget.NewEntry()
	saveDirEntry.SetPlaceHolder(cfg.DownloadDir)
	saveDirEntry.Text = cfg.DownloadDir

	browseBtn := widget.NewButton("Browse…", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			saveDirEntry.SetText(uri.Path())
		}, w)
	})

	speedEntry := widget.NewEntry()
	speedEntry.SetPlaceHolder("0 (unlimited)")

	scheduleEntry := widget.NewEntry()
	scheduleEntry.SetPlaceHolder("HH:MM (optional)")

	urlEntry.OnChanged = func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			categoryLabel.SetText("Category: —")
			return
		}
		cat := filecat.FromURL(raw)
		categoryLabel.SetText("Category: " + string(cat))
		if strings.TrimSpace(filenameEntry.Text) == "" {
			if name := BasenameFromURL(raw); name != "" {
				filenameEntry.SetText(name)
			}
		}
	}

	form := container.NewVBox(
		widget.NewLabel("URL:"),
		urlEntry,
		widget.NewLabel("Filename (optional):"),
		filenameEntry,
		widget.NewLabel("Save folder:"),
		container.NewBorder(nil, nil, nil, browseBtn, saveDirEntry),
		categoryLabel,
		widget.NewSeparator(),
		container.NewHBox(
			container.NewVBox(widget.NewLabel("Speed limit (KB/s):"), speedEntry),
			container.NewVBox(widget.NewLabel("Start at (HH:MM):"), scheduleEntry),
		),
	)

	d := dialog.NewCustomConfirm("Add Download", "Download", "Cancel", form,
		func(confirmed bool) {
			if !confirmed {
				return
			}
			rawURL := strings.TrimSpace(urlEntry.Text)
			if rawURL == "" {
				return
			}

			filename := strings.TrimSpace(filenameEntry.Text)
			if filename == "" {
				filename = BasenameFromURL(rawURL)
			}

			saveDir := strings.TrimSpace(saveDirEntry.Text)
			if saveDir == "" {
				saveDir = cfg.DownloadDir
			}

			var speedLimit int64
			if s := strings.TrimSpace(speedEntry.Text); s != "" {
				if kb, err := strconv.ParseInt(s, 10, 64); err == nil && kb > 0 {
					speedLimit = kb * 1024
				}
			}

			scheduledAt := strings.TrimSpace(scheduleEntry.Text)
			if scheduledAt != "" && !IsValidHHMM(scheduledAt) {
				ShowError(w, "Invalid schedule format. Use HH:MM (e.g. 23:30).")
				return
			}

			req := AddURLRequest{
				URL:         rawURL,
				Filename:    filename,
				SaveDir:     saveDir,
				Category:    string(filecat.FromURL(rawURL)),
				SpeedLimit:  speedLimit,
				ScheduledAt: scheduledAt,
			}

			select {
			case addCh <- req:
			default:
				ShowError(w, "Too many pending requests. Please wait.")
			}
		}, w)

	d.Resize(fyne.NewSize(540, 360))
	d.Show()
	w.Canvas().Focus(urlEntry)
}

// IsValidHHMM validates "HH:MM" format.
func IsValidHHMM(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	hh, err1 := strconv.Atoi(s[:2])
	mm, err2 := strconv.Atoi(s[3:])
	return err1 == nil && err2 == nil && hh >= 0 && hh <= 23 && mm >= 0 && mm <= 59
}

// BasenameFromURL extracts the filename from a URL (strips query/fragment).
func BasenameFromURL(rawURL string) string {
	if i := strings.IndexByte(rawURL, '?'); i != -1 {
		rawURL = rawURL[:i]
	}
	if i := strings.IndexByte(rawURL, '#'); i != -1 {
		rawURL = rawURL[:i]
	}
	return filepath.Base(rawURL)
}
