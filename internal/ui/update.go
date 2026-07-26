package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lyravein/lunefetch/internal/core"
	"github.com/lyravein/lunefetch/internal/storage"
)

type downloadReadyMsg struct{ id int64 }
type downloadErrMsg struct{ err error }
type downloadDoneMsg struct{ id int64 }

// fileConflictMsg is sent when a completed download's destination file already exists.
type fileConflictMsg struct {
	downloadID int64
	tmpPath    string
	finalPath  string
	filename   string
}

// duplicateURLMsg is sent when the user tries to add a URL that already exists in DB.
type duplicateURLMsg struct {
	url          string
	existingID   int64
	existingName string
}

// ScheduledReadyMsg is sent by the scheduler goroutine when a scheduled
// download's time has arrived.
type ScheduledReadyMsg struct{ ID int64 }

// AddURLMsg is sent by the HTTP API server when the browser extension
// forwards a URL to download.
type AddURLMsg struct{ URL string }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeToWindow()

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.MouseMsg:
		return m.handleMouseMsg(msg)

	case tickMsg:
		m.applyView()
		return m, m.refreshCmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case downloadReadyMsg:
		// Route through queue — may start immediately or enqueue.
		m.queue.TryStart(msg.id) //nolint:errcheck
		m.loadDownloads()
		return m, nil

	case downloadErrMsg:
		m.err = msg.err
		return m, nil

	case downloadDoneMsg:
		delete(m.activeDownloads, msg.id)
		// Download selesai (atau gagal permanen) — limit individualnya tidak
		// relevan lagi. Pause tidak lewat sini, jadi limit download yang
		// di-pause tetap tersimpan untuk resume.
		delete(m.itemLimits, msg.id)
		m.loadDownloads()

		// Notifikasi desktop — jalankan async supaya TUI tidak nge-block.
		if dl, err := m.state.GetDownload(msg.id); err == nil && dl != nil {
			go func(filename, status string) {
				switch status {
				case "completed":
					m.notifier.Send("Download selesai", filename)
				case "failed":
					m.notifier.Send("Download gagal", filename)
				}
			}(dl.Filename, dl.Status)
		}
		return m, nil

	case proceedToRenameMsg:
		m.pendingURL = msg.url
		m.renameInput.SetValue("")
		m.renameInput.Focus()
		m.currentPage = pageRename
		return m, textinput.Blink

	case fileConflictMsg:
		m.conflict = &conflictState{
			downloadID: msg.downloadID,
			tmpPath:    msg.tmpPath,
			finalPath:  msg.finalPath,
			filename:   msg.filename,
		}
		delete(m.activeDownloads, msg.downloadID)
		m.currentPage = pageConflict
		m.loadDownloads()
		return m, nil

	case duplicateURLMsg:
		m.duplicate = &duplicateState{
			url:          msg.url,
			existingID:   msg.existingID,
			existingName: msg.existingName,
		}
		m.currentPage = pageDuplicate
		return m, nil

	case ScheduledReadyMsg:
		// Scheduled time arrived — route through queue.
		m.queue.TryStart(msg.ID) //nolint:errcheck
		m.loadDownloads()
		return m, nil

	case AddURLMsg:
		m.pendingURL = msg.URL
		m.pendingFilename = ""
		m.pendingFolder = ""
		return m, m.checkDuplicateCmd(msg.URL)
	}

	return m, nil
}

func (m *model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.currentPage != pageList {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.table.MoveUp(1)
		return m, nil

	case tea.MouseButtonWheelDown:
		m.table.MoveDown(1)
		return m, nil

	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionRelease {
			return m, nil
		}
		const rowOffset = 3
		clickedRow := msg.Y - rowOffset
		rows := m.table.Rows()
		if clickedRow < 0 || clickedRow >= len(rows) {
			return m, nil
		}

		now := time.Now()
		isDoubleClick := clickedRow == m.lastClickRow &&
			now.Sub(m.lastClick) < 400*time.Millisecond

		m.lastClick = now
		m.lastClickRow = clickedRow

		m.table.GotoTop()
		m.table.MoveDown(clickedRow)

		if isDoubleClick {
			id := m.selectedRowID()
			if id > 0 {
				m.selectedID = id
				m.currentPage = pageDetail
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.currentPage {
	case pageList:
		return m.handleListKey(msg)
	case pageDetail:
		return m.handleDetailKey(msg)
	case pageAddURL:
		return m.handleAddURLKey(msg)
	case pageSchedule:
		return m.handleScheduleKey(msg)
	case pageConflict:
		return m.handleConflictKey(msg)
	case pageDuplicate:
		return m.handleDuplicateKey(msg)
	case pageRename:
		return m.handleRenameKey(msg)
	case pageSetFolder:
		return m.handleSetFolderKey(msg)
	case pageSpeedLimit:
		return m.handleSpeedLimitKey(msg)
	case pageHistory:
		return m.handleHistoryKey(msg)
	}
	return m, nil
}

func (m *model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Kalau search input sedang aktif, semua ketikan masuk ke sana dulu.
	if m.searchActive {
		switch msg.String() {
		case "esc":
			// Batal — kembalikan query ke sebelum dibuka.
			m.searchActive = false
			m.searchQuery = m.searchPrev
			m.searchInput.SetValue(m.searchQuery)
			m.searchInput.Blur()
			m.applyView()
			return m, nil
		case "enter":
			// Kunci hasil search.
			m.searchQuery = m.searchInput.Value()
			m.searchActive = false
			m.searchInput.Blur()
			m.applyView()
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		default:
			m.searchInput, cmd = m.searchInput.Update(msg)
			// Live filter saat ketik.
			m.searchQuery = m.searchInput.Value()
			m.applyView()
			return m, cmd
		}
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "n":
		m.currentPage = pageAddURL
		m.urlInput.SetValue("")
		m.urlInput.Focus()
		return m, textinput.Blink

	case "l", "L":
		// Buka page speed limit. Target per-download di-capture sekarang dari
		// baris yang di-highlight; ctrl+l dihindari karena banyak terminal
		// memakainya untuk clear screen.
		m.captureSpeedTarget()
		m.speedScope = scopeGlobal
		m.speedInput.SetValue(speedInputValue(m.globalLimit))
		m.speedInput.Focus()
		m.currentPage = pageSpeedLimit
		return m, textinput.Blink

	case "enter":
		id := m.selectedRowID()
		if id == 0 {
			return m, nil
		}
		m.selectedID = id
		m.currentPage = pageDetail
		return m, nil

	case "d":
		// Soft delete — entri masuk history, file di disk dibiarkan utuh.
		id := m.selectedRowID()
		if id > 0 {
			if ad, ok := m.activeDownloads[id]; ok {
				ad.downloader.Cancel()
				delete(m.activeDownloads, id)
			}
			delete(m.itemLimits, id)
			m.state.DeleteDownload(id)
			m.loadDownloads()
		}
		return m, nil

	case "D":
		// Hard delete — entri masuk history + file di disk dihapus.
		id := m.selectedRowID()
		if id > 0 {
			if ad, ok := m.activeDownloads[id]; ok {
				ad.downloader.Cancel()
				delete(m.activeDownloads, id)
			}
			delete(m.itemLimits, id)
			if dl, err := m.state.GetDownload(id); err == nil && dl != nil {
				ext := filepath.Ext(dl.Filename)
				baseName := dl.Filename[:len(dl.Filename)-len(ext)]
				tmpPath := filepath.Join(m.cfg.DownloadDir, baseName+".tmp"+ext)
				finalPath := filepath.Join(m.cfg.DownloadDir, dl.Filename)
				os.Remove(tmpPath)   //nolint:errcheck
				os.Remove(finalPath) //nolint:errcheck
			}
			m.state.DeleteWithFile(id)
			m.loadDownloads()
		}
		return m, nil

	case "r":
		id := m.selectedRowID()
		if id > 0 {
			if _, err := m.queue.TryStart(id); err != nil {
				m.err = err
			}
			m.loadDownloads()
		}
		return m, nil

	case "p":
		id := m.selectedRowID()
		if id > 0 {
			if ad, ok := m.activeDownloads[id]; ok && !ad.done.Load() {
				ad.downloader.Cancel()
				m.state.UpdateDownloadStatus(id, "paused")
				delete(m.activeDownloads, id)
				m.loadDownloads()
			}
		}
		return m, nil

	case "s":
		id := m.selectedRowID()
		if id > 0 {
			dl, err := m.state.GetDownload(id)
			if err == nil && dl != nil && (dl.Status == "paused" || dl.Status == "queued") {
				m.selectedID = id
				m.scheduleInput.SetValue("")
				m.scheduleInput.Focus()
				m.currentPage = pageSchedule
				return m, textinput.Blink
			}
		}
		return m, nil

	case "x":
		id := m.selectedRowID()
		if id > 0 {
			dl, err := m.state.GetDownload(id)
			if err == nil && dl != nil && dl.Status == "scheduled" {
				m.state.SetScheduledAt(id, nil) //nolint:errcheck
				if dl.QueuePosition.Valid {
					m.state.UpdateDownloadStatus(id, "queued") //nolint:errcheck
				} else {
					m.state.UpdateDownloadStatus(id, "paused") //nolint:errcheck
				}
				m.loadDownloads()
			}
		}
		return m, nil

	case "shift+up":
		id := m.selectedRowID()
		if id > 0 {
			m.state.MoveQueuePosition(id, -1) //nolint:errcheck
			m.loadDownloads()
		}
		return m, nil

	case "shift+down":
		id := m.selectedRowID()
		if id > 0 {
			m.state.MoveQueuePosition(id, 1) //nolint:errcheck
			m.loadDownloads()
		}
		return m, nil

	case "F":
		// Cycle filter: All → Active → Downloading → Queued → Paused → Completed → Failed → Scheduled → All
		m.statusFilter = (m.statusFilter + 1) % 8
		m.applyView()
		return m, nil

	case "o":
		// Cycle sort field: Default → Name → Size → Status → Default
		m.sortBy = (m.sortBy + 1) % 4
		m.sortDesc = false
		m.applyView()
		return m, nil

	case "O":
		// Balik arah sort; kalau sortDefault tidak ada efek.
		if m.sortBy != sortDefault {
			m.sortDesc = !m.sortDesc
			m.applyView()
		}
		return m, nil

	case "h":
		m.loadHistory()
		m.currentPage = pageHistory
		return m, nil

	case "/":
		// Buka search input inline.
		m.searchPrev = m.searchQuery
		m.searchActive = true
		m.searchInput.SetValue(m.searchQuery)
		m.searchInput.Focus()
		m.searchInput.CursorEnd()
		return m, textinput.Blink

	case "c":
		// Clear filter + search sekaligus.
		m.statusFilter = filterAll
		m.searchQuery = ""
		m.searchActive = false
		m.searchInput.SetValue("")
		m.searchInput.Blur()
		m.applyView()
		return m, nil
	}

	// Navigation keys (up/down/j/k/pgup/pgdn/home/end/g/G/space/b/f/u)
	// fall through to the table's own handler.
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// selectedRowID returns the download ID of the currently selected table row,
// or 0 if there is no valid selection.
func (m *model) selectedRowID() int64 {
	if len(m.table.Rows()) == 0 {
		return 0
	}
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return 0
	}
	return parseID(row[6]) // column 6 is the hidden DB ID
}

func (m *model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.currentPage = pageList
		m.updateTable()
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) handleAddURLKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.currentPage = pageList
		m.pendingURL = ""
		m.pendingFilename = ""
		m.pendingFolder = ""

	case "enter":
		url := m.urlInput.Value()
		if url == "" {
			return m, nil
		}
		m.pendingURL = url
		m.pendingFilename = ""
		m.pendingFolder = ""
		// Check for duplicate before proceeding; page will be set by the msg handler.
		return m, m.checkDuplicateCmd(url)
	}

	var cmd tea.Cmd
	m.urlInput, cmd = m.urlInput.Update(msg)
	return m, cmd
}

func (m *model) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Skip rename, proceed to folder selection.
		m.pendingFilename = ""
		m.folderInput.SetValue(m.cfg.DownloadDir)
		m.folderInput.Focus()
		m.currentPage = pageSetFolder
		return m, textinput.Blink

	case "enter":
		val := m.renameInput.Value()
		if val != "" {
			m.pendingFilename = val
		}
		m.folderInput.SetValue(m.cfg.DownloadDir)
		m.folderInput.Focus()
		m.currentPage = pageSetFolder
		return m, textinput.Blink
	}

	var cmd tea.Cmd
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m *model) handleSetFolderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.currentPage = pageList
		m.pendingURL = ""
		m.pendingFilename = ""
		m.pendingFolder = ""

	case "enter":
		val := m.folderInput.Value()
		folder := val
		if folder == "" {
			folder = m.cfg.DownloadDir
		}
		url := m.pendingURL
		filename := m.pendingFilename
		m.pendingURL = ""
		m.pendingFilename = ""
		m.pendingFolder = ""
		m.currentPage = pageList
		m.updateTable()
		return m, m.createDownloadWithOptsCmd(url, filename, folder)
	}

	var cmd tea.Cmd
	m.folderInput, cmd = m.folderInput.Update(msg)
	return m, cmd
}

// captureSpeedTarget mencatat download yang di-highlight sebagai target limit
// per-download. Kalau tidak ada baris yang dipilih, atau download-nya sudah
// selesai/gagal, target dikosongkan sehingga scope per-download tidak bisa
// dipakai dan user jatuh ke scope global saja.
func (m *model) captureSpeedTarget() {
	m.speedTargetID = 0
	m.speedTargetName = ""

	id := m.selectedRowID()
	if id == 0 {
		return
	}
	for i := range m.downloads {
		d := &m.downloads[i]
		if d.ID != id {
			continue
		}
		if !limitableStatus(d.Status) {
			return
		}
		m.speedTargetID = d.ID
		m.speedTargetName = d.Filename
		return
	}
}

// hasSpeedTarget melaporkan apakah scope per-download tersedia.
func (m *model) hasSpeedTarget() bool { return m.speedTargetID != 0 }

func (m *model) handleSpeedLimitKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.currentPage = pageList
		return m, nil

	case "tab", "shift+tab":
		// Toggle antara limit global dan limit download terpilih. Kalau tidak
		// ada download yang bisa dibatasi, scope tetap di global.
		if !m.hasSpeedTarget() {
			return m, nil
		}
		if m.speedScope == scopeGlobal {
			m.speedScope = scopeSelected
			m.speedInput.SetValue(speedInputValue(m.itemLimits[m.speedTargetID]))
		} else {
			m.speedScope = scopeGlobal
			m.speedInput.SetValue(speedInputValue(m.globalLimit))
		}
		m.speedInput.CursorEnd()
		return m, nil

	case "enter":
		limit, err := parseSpeedLimit(m.speedInput.Value())
		if err != nil {
			m.err = err
			return m, nil
		}

		if m.speedScope == scopeGlobal {
			m.globalLimit = limit
			m.cfg.GlobalSpeedLimit = limit
			// Global limiter dibagi ke semua download, jadi update rate-nya
			// langsung berlaku ke download yang sedang berjalan.
			m.globalLimiter.SetRate(limit)
			if err := m.cfg.Save(); err != nil {
				m.err = err
			}
		} else {
			m.applyItemLimit(m.speedTargetID, limit)
		}

		// Baris "Limit:" muncul/hilang, jadi tinggi tabel perlu dihitung ulang.
		m.resizeToWindow()
		m.currentPage = pageList
		m.updateTable()
		return m, nil
	}

	var cmd tea.Cmd
	m.speedInput, cmd = m.speedInput.Update(msg)
	return m, cmd
}

// applyItemLimit menyimpan limit untuk satu download dan langsung menerapkannya
// kalau download itu sedang jalan. Download lain tidak disentuh — inilah
// bedanya dengan limit global.
//
// Limit disimpan di map supaya tetap berlaku saat download yang masih queued
// atau paused dijalankan/di-resume nanti (startDownload membacanya kembali).
func (m *model) applyItemLimit(id, limit int64) {
	if id == 0 {
		return
	}

	if limit > 0 {
		m.itemLimits[id] = limit
	} else {
		delete(m.itemLimits, id)
	}

	ad, ok := m.activeDownloads[id]
	if !ok || ad.done.Load() {
		return
	}
	if limit > 0 {
		ad.downloader.SetLimiter(core.NewLimiter(limit))
	} else {
		ad.downloader.SetLimiter(nil)
	}
}

func (m *model) handleDuplicateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.duplicate == nil {
		m.currentPage = pageList
		return m, nil
	}

	switch msg.String() {
	case "a", "A":
		// Add anyway — generate suffix #1, #2, dst lalu set ke renameInput supaya user bisa edit
		existingName := m.duplicate.existingName
		url := m.duplicate.url
		m.duplicate = nil

		// Cari suffix yang belum dipakai
		ext := filepath.Ext(existingName)
		base := existingName[:len(existingName)-len(ext)]
		suggested := existingName
		for i := 1; i < 1000; i++ {
			candidate := fmt.Sprintf("%s #%d%s", base, i, ext)
			if !m.state.FilenameExists(candidate) {
				suggested = candidate
				break
			}
		}

		m.renameInput.SetValue(suggested)
		m.renameInput.Focus()
		m.currentPage = pageRename
		m.pendingURL = url
		return m, textinput.Blink

	case "r", "R":
		// Replace — hapus yang lama, download ulang
		if m.duplicate != nil {
			oldID := m.duplicate.existingID
			if ad, ok := m.activeDownloads[oldID]; ok {
				ad.downloader.Cancel()
				delete(m.activeDownloads, oldID)
			}
			delete(m.itemLimits, oldID)
			if dl, err := m.state.GetDownload(oldID); err == nil && dl != nil {
				ext := filepath.Ext(dl.Filename)
				baseName := dl.Filename[:len(dl.Filename)-len(ext)]
				os.Remove(filepath.Join(m.cfg.DownloadDir, baseName+".tmp"+ext))
				os.Remove(filepath.Join(m.cfg.DownloadDir, dl.Filename))
			}
			m.state.DeleteDownload(oldID)
		}
		url := m.duplicate.url
		m.duplicate = nil
		m.renameInput.SetValue("")
		m.renameInput.Focus()
		m.currentPage = pageRename
		m.pendingURL = url
		return m, textinput.Blink

	case "b", "B", "esc":
		// Block — batalkan
		m.duplicate = nil
		m.pendingURL = ""
		m.currentPage = pageList
		m.loadDownloads()
	}

	return m, nil
}

var reHHMM = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)

func (m *model) handleScheduleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.currentPage = pageList
		return m, nil

	case "enter":
		val := m.scheduleInput.Value()
		if !reHHMM.MatchString(val) {
			m.err = fmt.Errorf("format waktu tidak valid, gunakan HH:MM (contoh: 14:30)")
			m.currentPage = pageList
			return m, nil
		}

		id := m.selectedID
		if id > 0 {
			if err := m.state.SetScheduledAt(id, &val); err != nil {
				m.err = err
			} else {
				// Hapus queue_position supaya tidak ikut urutan queue.
				m.state.SetQueuePosition(id, nil)             //nolint:errcheck
				m.state.UpdateDownloadStatus(id, "scheduled") //nolint:errcheck
			}
		}
		m.currentPage = pageList
		m.loadDownloads()
		return m, nil
	}

	var cmd tea.Cmd
	m.scheduleInput, cmd = m.scheduleInput.Update(msg)
	return m, cmd
}

func (m *model) handleConflictKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.conflict == nil {
		m.currentPage = pageList
		return m, nil
	}

	switch msg.String() {
	case "o", "O":
		// Overwrite — hapus file lama lalu move .tmp ke final
		os.Remove(m.conflict.finalPath)
		if mvErr := core.MoveFile(m.conflict.tmpPath, m.conflict.finalPath); mvErr != nil {
			m.err = mvErr
			m.state.UpdateDownloadStatus(m.conflict.downloadID, "failed")
		} else {
			m.state.UpdateDownloadStatus(m.conflict.downloadID, "completed")
		}
		m.queue.OnDone(m.conflict.downloadID)
		m.conflict = nil
		m.currentPage = pageList
		m.loadDownloads()

	case "r", "R":
		// Rename — cari nama yang belum terpakai: file (1).zip, file (2).zip, dst
		newPath := resolveConflictName(m.conflict.finalPath)
		if mvErr := core.MoveFile(m.conflict.tmpPath, newPath); mvErr != nil {
			m.err = mvErr
			m.state.UpdateDownloadStatus(m.conflict.downloadID, "failed")
		} else {
			m.state.UpdateDownloadStatus(m.conflict.downloadID, "completed")
		}
		m.queue.OnDone(m.conflict.downloadID)
		m.conflict = nil
		m.currentPage = pageList
		m.loadDownloads()

	case "c", "C", "esc":
		// Cancel — hapus .tmp, set status failed
		os.Remove(m.conflict.tmpPath)
		m.state.UpdateDownloadStatus(m.conflict.downloadID, "failed")
		m.queue.OnDone(m.conflict.downloadID)
		m.conflict = nil
		m.currentPage = pageList
		m.loadDownloads()
	}

	return m, nil
}

// resolveConflictName returns a non-conflicting path by appending (1), (2), etc.
func resolveConflictName(path string) string {
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return path + ".new"
}

// checkDuplicateCmd checks if the URL already exists in the DB.
// If duplicate found, sends duplicateURLMsg; otherwise proceeds to rename page.
func (m *model) checkDuplicateCmd(url string) tea.Cmd {
	return func() tea.Msg {
		existing, err := m.state.FindByURL(url)
		if err == nil && existing != nil {
			return duplicateURLMsg{
				url:          url,
				existingID:   existing.ID,
				existingName: existing.Filename,
			}
		}
		// No duplicate — signal to open rename page (handled via pendingURL).
		return proceedToRenameMsg{url: url}
	}
}

// proceedToRenameMsg triggers the rename page after duplicate check passes.
type proceedToRenameMsg struct{ url string }

// createDownloadWithOptsCmd creates a download with optional filename and folder overrides.
func (m *model) createDownloadWithOptsCmd(url, filenameOverride, folder string) tea.Cmd {
	return func() tea.Msg {
		info, err := core.GetFileInfo(url)
		if err != nil {
			return downloadErrMsg{err}
		}

		filename := info.Filename
		if filenameOverride != "" {
			filename = filenameOverride
		}

		downloadDir := folder
		if downloadDir == "" {
			downloadDir = m.cfg.DownloadDir
		}

		numChunks := m.cfg.ChunksForSize(info.Size)
		id, err := m.state.CreateDownload(url, filename, info.Size, info.SupportsRange, numChunks)
		if err != nil {
			return downloadErrMsg{err}
		}

		chunks := core.CalculateChunks(info.Size, numChunks)
		startBytes := make([]int64, len(chunks))
		endBytes := make([]int64, len(chunks))
		for i, c := range chunks {
			startBytes[i] = c.Start
			endBytes[i] = c.End
		}

		if err := m.state.CreateChunks(id, startBytes, endBytes); err != nil {
			return downloadErrMsg{err}
		}

		return downloadReadyMsg{id: id}
	}
}

// createDownloadCmd is kept for backward compat (browser extension AddURLMsg).
func (m *model) createDownloadCmd(url string) tea.Cmd {
	return m.createDownloadWithOptsCmd(url, "", m.cfg.DownloadDir)
}

// handleHistoryKey menangani key di pageHistory.
// r = restore, D = purge satu, X = purge semua, esc/q = kembali ke list.
// Navigasi (up/down/j/k/etc.) diteruskan ke historyTable.
func (m *model) handleHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.currentPage = pageList
		return m, nil

	case "r":
		// Restore download: batal soft delete, status jadi paused.
		// Kalau .tmp ada dan chunk progress valid, resume bisa dilanjut;
		// kalau tidak, user tinggal tekan r dari list untuk download ulang.
		id := m.historySelectedID()
		if id > 0 {
			if err := m.state.RestoreDownload(id); err != nil {
				m.err = err
				return m, nil
			}
			// Validasi .tmp — kalau tidak ada atau ukurannya tidak konsisten,
			// reset chunk progress supaya resume mulai dari nol.
			if dl, err := m.state.GetDownload(id); err == nil && dl != nil {
				m.validateRestoredTmp(id, dl)
			}
			m.loadHistory()
			m.loadDownloads()
		}
		return m, nil

	case "D":
		// Purge satu entri history secara permanen.
		id := m.historySelectedID()
		if id > 0 {
			if err := m.state.PurgeDownload(id); err != nil {
				m.err = err
				return m, nil
			}
			m.loadHistory()
		}
		return m, nil

	case "X":
		// Purge seluruh history.
		if err := m.state.PurgeAllDeleted(); err != nil {
			m.err = err
			return m, nil
		}
		m.loadHistory()
		return m, nil
	}

	// Navigasi diteruskan ke historyTable.
	var cmd tea.Cmd
	m.historyTable, cmd = m.historyTable.Update(msg)
	return m, cmd
}

// historySelectedID mengembalikan DB ID dari baris yang dipilih di historyTable.
func (m *model) historySelectedID() int64 {
	if len(m.historyTable.Rows()) == 0 {
		return 0
	}
	row := m.historyTable.SelectedRow()
	if len(row) == 0 {
		return 0
	}
	return parseID(row[5]) // kolom 5 = hidden DB ID
}

// validateRestoredTmp memeriksa apakah .tmp masih ada dan ukurannya konsisten
// dengan chunk progress yang tersimpan. Kalau tidak, chunk di-reset supaya
// resume mulai dari nol dan tidak menghasilkan file korup.
func (m *model) validateRestoredTmp(id int64, dl *storage.DownloadRecord) {
	ext := filepath.Ext(dl.Filename)
	baseName := dl.Filename[:len(dl.Filename)-len(ext)]
	tmpPath := filepath.Join(m.cfg.DownloadDir, baseName+".tmp"+ext)

	info, err := os.Stat(tmpPath)
	if err != nil {
		// .tmp tidak ada — reset chunk supaya resume mulai dari nol.
		m.resetChunkProgress(id)
		return
	}

	// .tmp ada — cek ukurannya >= downloaded_size di DB.
	// Kalau lebih kecil, berarti file korup atau bukan milik kita.
	if info.Size() < dl.DownloadedSize {
		m.resetChunkProgress(id)
	}
}

// resetChunkProgress mereset semua chunk ke pending dan downloaded_size ke 0.
func (m *model) resetChunkProgress(id int64) {
	m.state.DB().Exec( //nolint:errcheck
		`UPDATE chunks SET downloaded_size = 0, status = 'pending', updated_at = CURRENT_TIMESTAMP
		 WHERE download_id = ?`, id,
	)
	m.state.DB().Exec( //nolint:errcheck
		`UPDATE downloads SET downloaded_size = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id,
	)
}

func parseID(s string) int64 {
	var id int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			id = id*10 + int64(c-'0')
		}
	}
	return id
}
