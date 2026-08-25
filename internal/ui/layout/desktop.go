// Package layout wires all UI components into the main application window.
package layout

import (
	"fmt"
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/core"
	"github.com/lyravein/lunefetch/internal/filecat"
	"github.com/lyravein/lunefetch/internal/notify"
	"github.com/lyravein/lunefetch/internal/queue"
	"github.com/lyravein/lunefetch/internal/storage"
	"github.com/lyravein/lunefetch/internal/ui/assets"
	"github.com/lyravein/lunefetch/internal/ui/components"
	"github.com/lyravein/lunefetch/internal/ui/pages"
	"github.com/lyravein/lunefetch/internal/ui/store"
	uitheme "github.com/lyravein/lunefetch/internal/ui/theme"
)

// App holds all GUI state and dependencies.
type App struct {
	fyneApp fyne.App
	window  fyne.Window

	sm  *storage.StateManager
	cfg *config.Config
	qm  *queue.Manager
	st  *store.DownloadStore

	// globalLimiter is shared across all active downloaders.
	globalLimiter *core.Limiter

	// AddURLCh receives download requests from the API server, dialog, and scheduler.
	AddURLCh chan components.AddURLRequest

	downloads *pages.DownloadsPage
	history   *pages.HistoryPage
	statusBar *components.StatusBar
	sidebar   *components.Sidebar
	header    *components.ContentHeader
	summaries *components.SummaryCards

	// lastSize tracks the window size so we only persist on change.
	lastSize         fyne.Size
	onDownloadsRoute bool
	stopCh           chan struct{}
	shutdownOnce     sync.Once
}

// Preference keys for persisted window geometry.
const (
	prefWindowWidth  = "window.width"
	prefWindowHeight = "window.height"
	minWindowWidth   = 1000
	minWindowHeight  = 640
)

// New creates the Fyne app and wires everything together.
func New(sm *storage.StateManager, cfg *config.Config) *App {
	a := app.NewWithID("io.github.lyravein.lunefetch")
	a.Settings().SetTheme(uitheme.NewNavy())
	a.SetIcon(assets.AppIcon)
	w := a.NewWindow("Lunefetch")
	w.SetIcon(assets.AppIcon)

	// Restore persisted window size, falling back to a sane default.
	// Upper clamps guard against previously corrupted (runaway) values.
	prefs := a.Preferences()
	ww := float32(prefs.Float(prefWindowWidth))
	wh := float32(prefs.Float(prefWindowHeight))
	if ww < 760 || wh < 480 || ww > 2400 || wh > 1600 {
		ww, wh = minWindowWidth, minWindowHeight
	}
	w.Resize(fyne.NewSize(ww, wh))

	guiApp := &App{
		fyneApp:          a,
		window:           w,
		sm:               sm,
		cfg:              cfg,
		AddURLCh:         make(chan components.AddURLRequest, 16),
		lastSize:         fyne.NewSize(ww, wh),
		onDownloadsRoute: true,
		stopCh:           make(chan struct{}),
	}
	guiApp.configureSystemTray()

	// Initialise global limiter from config.
	if cfg.GlobalSpeedLimit > 0 {
		guiApp.globalLimiter = core.NewLimiter(cfg.GlobalSpeedLimit)
	}

	// Create Store.
	guiApp.st = store.NewDownloadStore(sm)
	guiApp.st.SetAdder(guiApp)

	// Build pages.
	guiApp.downloads = pages.NewDownloadsPage(sm, cfg, &guiApp.globalLimiter, guiApp.st)
	notifier := notify.New(cfg.Notifications)
	// Fyne's SendNotification is the only mechanism that works on Windows;
	// notify-send is a Linux-only fallback.
	notifier.SetApp(a)
	guiApp.downloads.SetNotifier(notifier)
	guiApp.history = pages.NewHistoryPage(sm, w)
	guiApp.history.SetPurgeCleaner(guiApp.downloads.RemovePartFile)

	// Queue manager: StartFunc is downloads.StartDownload.
	guiApp.qm = queue.NewManager(sm, cfg.MaxConcurrent, guiApp.downloads.StartDownload)
	guiApp.downloads.SetQueueManager(guiApp.qm)
	guiApp.downloads.SetWindow(w)

	// Inject mutator into store now that downloads page is ready.
	guiApp.st.SetMutator(guiApp.downloads)

	// Load initial state into store.
	guiApp.st.Load() //nolint:errcheck

	guiApp.header = components.NewContentHeader(w, cfg, guiApp.AddURLCh, func(query string) {
		guiApp.st.SetSearch(query)
		guiApp.downloads.Refresh()
	})
	guiApp.summaries = components.NewSummaryCards()

	// Build status bar.
	guiApp.statusBar = components.NewStatusBar(cfg, guiApp.qm)

	// Sidebar: filters the download list or switches to history.
	contentStack := container.NewStack(guiApp.downloads.Container())

	guiApp.sidebar = components.NewSidebar(
		func(f store.DownloadStatus) {
			guiApp.onDownloadsRoute = true
			contentStack.Objects = []fyne.CanvasObject{guiApp.downloads.Container()}
			contentStack.Refresh()
			guiApp.st.SetFilter(f)
			guiApp.downloads.Refresh()
		},
		func(category string) {
			guiApp.onDownloadsRoute = true
			contentStack.Objects = []fyne.CanvasObject{guiApp.downloads.Container()}
			contentStack.Refresh()
			guiApp.st.SetCategory(category)
			guiApp.downloads.Refresh()
		},
		func() {
			guiApp.onDownloadsRoute = false
			guiApp.st.Select(0)
			contentStack.Objects = []fyne.CanvasObject{guiApp.history.Container()}
			contentStack.Refresh()
			guiApp.history.Refresh()
		},
	)
	guiApp.sidebar.SetFooterActions(
		func() { components.ShowSettingsDialog(w, cfg, &guiApp.globalLimiter, guiApp.qm) },
		func() { components.ShowAboutDialog(w) },
	)

	sidebar := container.NewBorder(nil, nil, nil, widget.NewSeparator(), guiApp.sidebar.Container())
	mainContent := container.NewBorder(guiApp.summaries.Root, nil, nil, nil, contentStack)
	main := container.NewBorder(guiApp.header.Root, nil, nil, nil, mainContent)
	body := container.NewBorder(nil, nil, sidebar, nil, main)
	root := container.NewBorder(nil, guiApp.statusBar.Container(), nil, nil, body)
	minimum := canvas.NewRectangle(color.Transparent)
	minimum.SetMinSize(fyne.NewSize(minWindowWidth, minWindowHeight))
	w.SetContent(container.NewStack(minimum, root))

	guiApp.registerShortcuts(guiApp.header)
	w.SetCloseIntercept(func() {
		if cfg.CloseToTray {
			w.Hide()
			return
		}
		guiApp.Shutdown()
		w.Close()
	})

	return guiApp
}

func (a *App) configureSystemTray() {
	desk, ok := a.fyneApp.(desktop.App)
	if !ok {
		return
	}
	show := func() {
		a.window.Show()
		a.window.RequestFocus()
	}
	quit := func() {
		a.Shutdown()
		a.fyneApp.Quit()
	}
	desk.SetSystemTrayMenu(fyne.NewMenu("Lunefetch",
		fyne.NewMenuItem("Open Lunefetch", show),
		fyne.NewMenuItem("Add download", func() {
			show()
			components.ShowAddURLDialog(a.window, a.cfg, a.AddURLCh)
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", quit),
	))
	desk.SetSystemTrayWindow(a.window)
	desk.SetSystemTrayIcon(assets.AppIcon)
}

// registerShortcuts wires global keyboard shortcuts onto the window canvas.
//
//	Ctrl+N:  open Add URL dialog
//	Ctrl+F:  focus the search entry
//	Ctrl+C:  copy the selected download's URL
//	Ctrl+P:  pause all active downloads
//	Delete:  remove the selected download (with confirm)
//	Space:   pause/resume the selected download
//
// Every shortcut that is also meaningful inside a text field checks
// entryFocused() first, so typing in search is never hijacked.
func (a *App) registerShortcuts(header *components.ContentHeader) {
	canvas := a.window.Canvas()

	// entryFocused reports whether keyboard focus is inside a text entry,
	// in which case Delete/Space must not be hijacked.
	entryFocused := func() bool {
		_, ok := canvas.Focused().(*widget.Entry)
		return ok
	}

	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierControl},
		func(fyne.Shortcut) {
			components.ShowAddURLDialog(a.window, a.cfg, a.AddURLCh)
		})

	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierControl},
		func(fyne.Shortcut) {
			if !a.onDownloadsRoute {
				return
			}
			canvas.Focus(header.Search)
		})

	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyC, Modifier: fyne.KeyModifierControl},
		func(fyne.Shortcut) {
			// Inside a text entry Ctrl+C must keep copying the selection.
			if !a.onDownloadsRoute || entryFocused() {
				return
			}
			sel := a.st.Selected()
			if sel == nil {
				return
			}
			a.window.Clipboard().SetContent(sel.URL)
		})

	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyP, Modifier: fyne.KeyModifierControl},
		func(fyne.Shortcut) {
			if entryFocused() {
				return
			}
			a.downloads.PauseAll()
		})

	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyDelete},
		func(fyne.Shortcut) {
			if !a.onDownloadsRoute || entryFocused() {
				return
			}
			sel := a.st.Selected()
			if sel == nil {
				return
			}
			dialog.ShowConfirm("Remove Download", "Remove this download from the list?",
				func(ok bool) {
					if ok {
						a.downloads.DeleteDownload(sel.ID)
					}
				}, a.window)
		})

	canvas.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySpace},
		func(fyne.Shortcut) {
			if !a.onDownloadsRoute || entryFocused() {
				return
			}
			sel := a.st.Selected()
			if sel == nil {
				return
			}
			switch sel.Status {
			case "downloading":
				a.downloads.PauseDownload(sel.ID)
			case "paused", "failed", "cancelled", "queued":
				a.downloads.ResumeDownload(sel.ID)
			}
		})
}

// Run starts the refresh loop and shows the window.
func (a *App) Run() {
	go a.refreshLoop()
	go a.addURLLoop()
	a.window.ShowAndRun()
	a.Shutdown()
}

// refreshLoop polls storage and updates the UI every 500ms.
func (a *App) refreshLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
		}
		a.st.Load() //nolint:errcheck
		a.downloads.Refresh()

		// Update status summaries from all records (unfiltered).
		allRecords, _ := a.sm.ListDownloads()
		counts := make(map[string]int)
		for i := range allRecords {
			counts[allRecords[i].Status]++
		}
		a.summaries.Update(counts)

		// Update status bar.
		totalSpeed := a.downloads.SpeedTotal()
		activeCount := a.downloads.ActiveCount()
		totalCount := len(allRecords)
		a.statusBar.Update(totalSpeed, activeCount, totalCount)

		// Persist window size when it changes.
		a.persistWindowSize()
	}
}

// persistWindowSize saves the window size to preferences when it has changed.
// Runs on the refresh tick, cheap because it only writes on change.
func (a *App) persistWindowSize() {
	size := a.window.Canvas().Size()
	if size == a.lastSize || size.Width < 760 || size.Height < 480 {
		return
	}
	a.lastSize = size
	prefs := a.fyneApp.Preferences()
	prefs.SetFloat(prefWindowWidth, float64(size.Width))
	prefs.SetFloat(prefWindowHeight, float64(size.Height))
}

// addURLLoop listens for new download requests from the dialog / API server.
func (a *App) addURLLoop() {
	for {
		var req components.AddURLRequest
		select {
		case <-a.stopCh:
			return
		case req = <-a.AddURLCh:
		}
		if req.URL == "" {
			a.downloads.Refresh()
			continue
		}
		a.HandleAdd(store.AddRequest{
			URL:         req.URL,
			Filename:    req.Filename,
			SaveDir:     req.SaveDir,
			Category:    req.Category,
			SpeedLimit:  req.SpeedLimit,
			ScheduledAt: req.ScheduledAt,
		})
	}
}

// Done is closed when application shutdown begins.
func (a *App) Done() <-chan struct{} { return a.stopCh }

// Shutdown stops background UI work and waits for active downloads to become
// safely resumable. It is idempotent so both the close intercept and Run can
// invoke it.
func (a *App) Shutdown() {
	a.shutdownOnce.Do(func() {
		close(a.stopCh)
		a.downloads.Shutdown()
	})
}

// HandleAdd implements store.Adder: processes a new download request.
func (a *App) HandleAdd(req store.AddRequest) {
	if existing, err := a.sm.FindByURL(req.URL); err == nil && existing != nil {
		components.ShowError(a.window, fmt.Sprintf("This URL is already in the download list:\n%s", existing.Filename))
		return
	}

	info, err := core.GetFileInfo(req.URL, a.cfg.ProxyURL, a.cfg.AllowLocalHosts)
	if err != nil {
		components.ShowError(a.window, fmt.Sprintf("Failed to fetch file info:\n%v", err))
		return
	}

	filename := req.Filename
	if filename == "" {
		filename = info.Filename
	}
	filename, err = core.ValidateFilename(filename)
	if err != nil {
		components.ShowError(a.window, fmt.Sprintf("Invalid filename:\n%v", err))
		return
	}
	category := req.Category
	if category == "" {
		category = string(filecat.FromFilename(filename))
	}

	original := filename
	ext := extOf(original)
	base := original[:len(original)-len(ext)]
	for i := 1; a.sm.FilenameExists(filename); i++ {
		filename = fmt.Sprintf("%s (#%d)%s", base, i, ext)
	}

	saveDir := req.SaveDir
	if saveDir == "" {
		saveDir = a.cfg.DownloadDir
	}
	if _, err := core.SafeDownloadPath(saveDir, filename); err != nil {
		components.ShowError(a.window, fmt.Sprintf("Invalid download destination:\n%v", err))
		return
	}

	numChunks := a.cfg.ChunksForSize(info.Size)
	if !info.SupportsRange {
		numChunks = 1
	}
	chunks := core.CalculateChunks(info.Size, numChunks)
	starts := make([]int64, len(chunks))
	ends := make([]int64, len(chunks))
	for i, c := range chunks {
		starts[i] = c.Start
		ends[i] = c.End
	}
	id, err := a.sm.CreateDownloadWithChunks(req.URL, filename, saveDir, category, info.Size, info.SupportsRange, starts, ends, info.ETag, info.LastModified)
	if err != nil {
		components.ShowError(a.window, fmt.Sprintf("Failed to save download:\n%v", err))
		return
	}
	if req.SpeedLimit > 0 {
		a.sm.UpdateSpeedLimit(id, req.SpeedLimit) //nolint:errcheck
	}

	if req.ScheduledAt != "" {
		a.sm.UpdateDownloadStatus(id, "scheduled") //nolint:errcheck
		a.sm.SetScheduledAt(id, &req.ScheduledAt)  //nolint:errcheck
		a.downloads.Refresh()
		return
	}

	a.qm.TryStart(id) //nolint:errcheck
	a.downloads.Refresh()
}

// EnqueueScheduled starts a scheduled download or places it in the queue.
func (a *App) EnqueueScheduled(id int64) {
	if a.qm != nil {
		_ = a.qm.EnqueueScheduled(id)
	}
}

// RecoverDownloads reconciles persisted runtime-only states and starts queued
// work up to the configured concurrency limit.
func (a *App) RecoverDownloads() {
	if err := a.sm.ReconcileInterrupted(); err == nil && a.qm != nil {
		a.qm.Drain()
	}
}

func extOf(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
		if filename[i] == '/' || filename[i] == '\\' {
			break
		}
	}
	return ""
}
