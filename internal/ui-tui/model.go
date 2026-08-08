package uitui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/core"
	"github.com/lyravein/lunefetch/internal/notify"
	"github.com/lyravein/lunefetch/internal/queue"
	"github.com/lyravein/lunefetch/internal/storage"
)

var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
)

type activeDownload struct {
	downloader *core.Downloader
	state      *storage.DownloadRecord
	started    time.Time
	done       atomic.Bool
}

type page int

const (
	pageList page = iota
	pageDetail
	pageAddURL
	pageSchedule
	pageConflict
	pageDuplicate
	pageRename
	pageSetFolder
	pageSpeedLimit
	pageHistory
)

// conflictState holds the context needed to resolve a file conflict.
type conflictState struct {
	downloadID int64
	tmpPath    string
	finalPath  string
	filename   string
}

// duplicateState holds the pending download info when a duplicate URL is detected.
type duplicateState struct {
	url          string
	existingID   int64
	existingName string
}

// filterStatus mendefinisikan filter status yang aktif di list view.
type filterStatus int

const (
	filterAll filterStatus = iota
	filterActive
	filterDownloading
	filterQueued
	filterPaused
	filterCompleted
	filterFailed
	filterScheduled
)

func (f filterStatus) String() string {
	switch f {
	case filterActive:
		return "Active"
	case filterDownloading:
		return "Downloading"
	case filterQueued:
		return "Queued"
	case filterPaused:
		return "Paused"
	case filterCompleted:
		return "Completed"
	case filterFailed:
		return "Failed"
	case filterScheduled:
		return "Scheduled"
	default:
		return "All"
	}
}

// sortField mendefinisikan kolom yang jadi dasar sort.
type sortField int

const (
	sortDefault sortField = iota // urutan queue-aware dari SQL
	sortName
	sortSize
	sortStatus
)

func (s sortField) String() string {
	switch s {
	case sortName:
		return "Name"
	case sortSize:
		return "Size"
	case sortStatus:
		return "Status"
	default:
		return "Default"
	}
}

type model struct {
	state           *storage.StateManager
	cfg             *config.Config
	program         *tea.Program
	queue           *queue.Manager
	downloads       []storage.DownloadRecord
	visible         []storage.DownloadRecord // m.downloads setelah filter+search+sort
	activeDownloads map[int64]*activeDownload
	currentPage     page
	table           table.Model
	selectedID      int64
	urlInput        textinput.Model
	renameInput     textinput.Model
	folderInput     textinput.Model
	scheduleInput   textinput.Model
	spinner         spinner.Model
	err             error
	width           int
	height          int
	lastClick       time.Time // for double-click detection
	lastClickRow    int
	conflict        *conflictState
	duplicate       *duplicateState
	pendingURL      string // URL waiting to be downloaded after rename/folder/duplicate resolved
	pendingFilename string // filename override (from rename page)
	pendingFolder   string // folder override (from set-folder page)

	globalLimiter *core.Limiter // shared by every active download
	globalLimit   int64         // bytes/sec, 0 = unlimited

	notifier *notify.Notifier // notifikasi desktop, no-op kalau disabled/absen

	// itemLimits menyimpan limit per download, dikunci dengan ID download.
	// Hanya berisi entri untuk download yang benar-benar dibatasi; download
	// tanpa entri jalan tanpa limit individual. Entri dihapus saat download
	// selesai atau dibatalkan supaya map tidak bocor.
	itemLimits map[int64]int64

	speedInput textinput.Model
	speedScope speedScope // which limit the speed page is editing

	// speedTarget adalah download yang akan dibatasi kalau scope-nya
	// scopeSelected. Di-capture saat page dibuka supaya nilainya tidak
	// berubah kalau tabel ter-refresh di belakang.
	speedTargetID   int64
	speedTargetName string

	// Filter, sort, search — state session-only, tidak disimpan ke config.
	statusFilter filterStatus
	sortBy       sortField
	sortDesc     bool
	searchQuery  string // filter aktif (kosong = tidak ada)
	searchPrev   string // query sebelum search dibuka, untuk restore saat esc
	searchActive bool   // true = search input sedang terbuka
	searchInput  textinput.Model

	// History page state.
	history         []storage.DownloadRecord
	historyTable    table.Model
	historySelected int64
}

// speedScope selects which limit the speed-limit page edits.
type speedScope int

const (
	scopeGlobal speedScope = iota
	// scopeSelected membatasi hanya download yang di-highlight di list.
	scopeSelected
)

// limitableStatus melaporkan apakah download dengan status ini masih bisa
// dibatasi. Download yang sudah selesai atau gagal tidak akan menarik
// bandwidth lagi, jadi limit untuk mereka tidak ada artinya.
func limitableStatus(status string) bool {
	switch status {
	case "queued", "paused", "downloading", "retrying", "scheduled":
		return true
	}
	return false
}

func NewModel(sm *storage.StateManager, cfg *config.Config) *model {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "#", Width: 4},
			{Title: "File", Width: 40},
			{Title: "Size", Width: 10},
			{Title: "Progress", Width: 12},
			{Title: "Speed", Width: 12},
			{Title: "Status", Width: 12},
			{Title: "_id", Width: 0}, // hidden column — stores the real DB ID
		}),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	ti := textinput.New()
	ti.Placeholder = "Enter download URL..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 60

	si := textinput.New()
	si.Placeholder = "HH:MM (e.g. 14:30)"
	si.CharLimit = 5
	si.Width = 20

	ri := textinput.New()
	ri.Placeholder = "Nama file baru..."
	ri.CharLimit = 255
	ri.Width = 60

	fi := textinput.New()
	fi.Placeholder = "Folder tujuan (kosong = default)..."
	fi.CharLimit = 500
	fi.Width = 60

	spd := textinput.New()
	spd.Placeholder = "contoh: 500k, 2m, 1.5m (kosong = unlimited)"
	spd.CharLimit = 20
	spd.Width = 40

	srch := textinput.New()
	srch.Placeholder = "cari nama file..."
	srch.CharLimit = 200
	srch.Width = 40

	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := &model{
		state:           sm,
		cfg:             cfg,
		activeDownloads: make(map[int64]*activeDownload),
		currentPage:     pageList,
		table:           t,
		urlInput:        ti,
		scheduleInput:   si,
		renameInput:     ri,
		folderInput:     fi,
		speedInput:      spd,
		searchInput:     srch,
		spinner:         sp,
		globalLimit:     cfg.GlobalSpeedLimit,
		globalLimiter:   core.NewLimiter(cfg.GlobalSpeedLimit),
		itemLimits:      make(map[int64]int64),
		notifier:        notify.New(cfg.Notifications),
	}

	// History table — kolom sama dengan main table kecuali Speed diganti DeletedAt.
	ht := table.New(
		table.WithColumns([]table.Column{
			{Title: "#", Width: 4},
			{Title: "File", Width: 40},
			{Title: "Size", Width: 10},
			{Title: "Status", Width: 12},
			{Title: "Deleted", Width: 16},
			{Title: "_id", Width: 0},
		}),
		table.WithFocused(true),
		table.WithHeight(20),
	)
	ht.SetStyles(s)
	m.historyTable = ht

	// Wire queue manager — startDownload is called by the queue when a slot opens.
	m.queue = queue.NewManager(sm, cfg.MaxConcurrent, m.startDownload)

	m.loadDownloads()
	return m
}

// SetProgram wires the tea.Program so goroutines can send messages back to
// the TUI update loop (e.g. to trigger a refresh when a download finishes).
func (m *model) SetProgram(p *tea.Program) {
	m.program = p
}

// resizeToWindow recalculates table column widths and height based on the
// current terminal dimensions. Called on WindowSizeMsg.
func (m *model) resizeToWindow() {
	if m.width == 0 || m.height == 0 {
		return
	}

	// Without the baseStyle border wrapper, usable = full terminal width.
	usable := m.width
	if usable < 40 {
		usable = 40
	}
	m.table.SetWidth(usable)

	// Fixed columns: # (4), Size (10), Progress (10), Speed (10), Status (10)
	// bubbles/table adds 1-char left+right padding per column = 2 chars × 6 visible cols = 12
	const fixedCols = 4 + 10 + 10 + 10 + 12 + 12
	fileWidth := usable - fixedCols
	if fileWidth < 10 {
		fileWidth = 10
	}
	m.table.SetColumns([]table.Column{
		{Title: "#", Width: 4},
		{Title: "File", Width: fileWidth},
		{Title: "Size", Width: 10},
		{Title: "Progress", Width: 10},
		{Title: "Speed", Width: 10},
		{Title: "Status", Width: 12},
		{Title: "_id", Width: 0},
	})

	// Reserve rows for:
	// title(1) + blank(1) + table-header(1) + help-line-1(1) + help-line-2(1) + padding(2) = 7
	// +1 kalau baris speed limit aktif
	// +1 kalau baris filter/search aktif
	// +1 kalau search input sedang terbuka
	reserved := 7
	if m.globalLimit > 0 || len(m.itemLimits) > 0 {
		reserved++
	}
	if m.statusFilter != filterAll || m.searchQuery != "" {
		reserved++
	}
	if m.searchActive {
		reserved++
	}
	tableHeight := m.height - reserved
	if tableHeight < 3 {
		tableHeight = 3
	}
	m.table.SetHeight(tableHeight)

	// Input width: leave some margin
	inputWidth := m.width - 10
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.urlInput.Width = inputWidth
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.refreshCmd,
	)
}

func (m *model) refreshCmd() tea.Msg {
	return tickMsg{}
}

type tickMsg struct{}

func (m *model) loadDownloads() {
	downloads, err := m.state.ListDownloads()
	if err != nil {
		m.err = err
		return
	}
	m.downloads = downloads
	m.applyView()
}

// applyView menjalankan pipeline filter → search → sort terhadap m.downloads
// dan menyimpan hasilnya di m.visible, lalu rebuild tabel.
func (m *model) applyView() {
	src := m.downloads

	// 1. Filter status.
	if m.statusFilter != filterAll {
		filtered := src[:0:0]
		for _, d := range src {
			if m.matchesFilter(d) {
				filtered = append(filtered, d)
			}
		}
		src = filtered
	}

	// 2. Filter search (case-insensitive substring pada nama file).
	if m.searchQuery != "" {
		q := strings.ToLower(m.searchQuery)
		matched := src[:0:0]
		for _, d := range src {
			if strings.Contains(strings.ToLower(d.Filename), q) {
				matched = append(matched, d)
			}
		}
		src = matched
	}

	// 3. Sort.
	if m.sortBy != sortDefault {
		sorted := make([]storage.DownloadRecord, len(src))
		copy(sorted, src)
		m.sortRecords(sorted)
		src = sorted
	}

	m.visible = src
	m.updateTable()
}

// matchesFilter melaporkan apakah record d lulus filter status yang aktif.
func (m *model) matchesFilter(d storage.DownloadRecord) bool {
	switch m.statusFilter {
	case filterActive:
		return d.Status == "downloading" || d.Status == "queued" || d.Status == "paused"
	case filterDownloading:
		return d.Status == "downloading"
	case filterQueued:
		return d.Status == "queued"
	case filterPaused:
		return d.Status == "paused"
	case filterCompleted:
		return d.Status == "completed"
	case filterFailed:
		return d.Status == "failed"
	case filterScheduled:
		return d.Status == "scheduled"
	}
	return true
}

// sortRecords mengurutkan slice in-place sesuai m.sortBy dan m.sortDesc.
func (m *model) sortRecords(recs []storage.DownloadRecord) {
	less := func(i, j int) bool {
		a, b := recs[i], recs[j]
		var cmp int
		switch m.sortBy {
		case sortName:
			cmp = strings.Compare(strings.ToLower(a.Filename), strings.ToLower(b.Filename))
		case sortSize:
			cmp = int(a.TotalSize - b.TotalSize)
		case sortStatus:
			cmp = strings.Compare(a.Status, b.Status)
		}
		if cmp == 0 {
			cmp = int(a.ID - b.ID)
		}
		if m.sortDesc {
			return cmp > 0
		}
		return cmp < 0
	}
	// insertion sort — list kecil, tidak perlu sort.Slice overhead
	for i := 1; i < len(recs); i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			recs[j], recs[j-1] = recs[j-1], recs[j]
		}
	}
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return formatInt(bytes) + " B"
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return formatInt(int64(float64(bytes)/float64(div))) + " " + string("KMGTPE"[exp]) + "B"
}

func formatInt(n int64) string {
	if n < 1000 {
		return formatIntSmall(n)
	}
	return formatIntSmall(n/1000) + "," + format3Digits(n%1000)
}

func formatIntSmall(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func format3Digits(n int64) string {
	if n < 10 {
		return "00" + formatIntSmall(n)
	}
	if n < 100 {
		return "0" + formatIntSmall(n)
	}
	return formatIntSmall(n)
}

func formatFloat(f float64) string {
	if f < 10 {
		return formatIntSmall(int64(f * 100))[:1] + "." + formatIntSmall(int64(f * 100))[1:]
	}
	return formatIntSmall(int64(f)) + "." + formatIntSmall(int64(f*10)%10)
}

func formatSpeed(bytesPerSec float64) string {
	return formatSize(int64(bytesPerSec)) + "/s"
}

// parseSpeedLimit mengubah input human-readable jadi bytes/sec.
// Menerima "500k", "2m", "1.5m", "1g", "1024" (bytes), atau kosong (unlimited).
func parseSpeedLimit(s string) (int64, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "0" {
		return 0, nil
	}
	s = strings.TrimSuffix(s, "b/s")
	s = strings.TrimSuffix(s, "/s")
	s = strings.TrimSuffix(s, "b")
	s = strings.TrimSpace(s)

	mult := int64(1)
	if len(s) > 0 {
		switch s[len(s)-1] {
		case 'k':
			mult = 1 << 10
			s = s[:len(s)-1]
		case 'm':
			mult = 1 << 20
			s = s[:len(s)-1]
		case 'g':
			mult = 1 << 30
			s = s[:len(s)-1]
		}
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("format tidak valid: %q (contoh: 500k, 2m)", s)
	}
	if val < 0 {
		return 0, fmt.Errorf("speed limit tidak boleh negatif")
	}
	limit := int64(val * float64(mult))
	if limit > 0 && limit < 1024 {
		return 0, fmt.Errorf("speed limit minimal 1k")
	}
	return limit, nil
}

// formatSpeedLimit menampilkan limit dalam bentuk ringkas untuk UI.
func formatSpeedLimit(bytesPerSec int64) string {
	if bytesPerSec <= 0 {
		return "unlimited"
	}
	return formatSize(bytesPerSec) + "/s"
}

// speedInputValue mengubah limit jadi string yang bisa diedit user di input.
func speedInputValue(bytesPerSec int64) string {
	switch {
	case bytesPerSec <= 0:
		return ""
	case bytesPerSec%(1<<30) == 0:
		return formatIntSmall(bytesPerSec/(1<<30)) + "g"
	case bytesPerSec%(1<<20) == 0:
		return formatIntSmall(bytesPerSec/(1<<20)) + "m"
	case bytesPerSec%(1<<10) == 0:
		return formatIntSmall(bytesPerSec/(1<<10)) + "k"
	default:
		return formatIntSmall(bytesPerSec)
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return formatInt(int64(h)) + "h" + formatInt(int64(m)) + "m" + formatInt(int64(s)) + "s"
	}
	if m > 0 {
		return formatInt(int64(m)) + "m" + formatInt(int64(s)) + "s"
	}
	return formatInt(int64(s)) + "s"
}

func (m *model) loadHistory() {
	deleted, err := m.state.ListDeleted()
	if err != nil {
		m.err = err
		return
	}
	m.history = deleted

	rows := make([]table.Row, 0, len(deleted))
	for i, d := range deleted {
		deletedAt := ""
		if d.DeletedAt.Valid {
			deletedAt = d.DeletedAt.Time.Format("2006-01-02 15:04")
		}
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", i+1),
			d.Filename,
			formatSize(d.TotalSize),
			d.Status,
			deletedAt,
			fmt.Sprintf("%d", d.ID),
		})
	}
	m.historyTable.SetRows(rows)
}

func (m *model) updateTable() {
	// Saat tickMsg tiba (bukan loadDownloads), re-apply view supaya progress
	// yang aktif ikut terupdate tanpa perlu reload dari DB.
	rows := make([]table.Row, 0, len(m.visible))
	for i, d := range m.visible {
		progress := "0%"
		speed := "-"
		status := d.Status

		if ad, ok := m.activeDownloads[d.ID]; ok && !ad.done.Load() {
			p := ad.downloader.GetProgress()
			sp := ad.downloader.GetSpeed()
			if p.TotalSize > 0 {
				progress = formatFloat(float64(p.DownloadedSize)/float64(p.TotalSize)*100) + "%"
			}
			speed = formatSpeed(sp)
			status = "Downloading"
		} else if d.Status == "completed" {
			status = "Completed"
			progress = "100%"
		} else if d.Status == "failed" {
			status = "Failed"
		} else if d.Status == "paused" {
			status = "Paused"
			if d.TotalSize > 0 {
				progress = formatFloat(float64(d.DownloadedSize)/float64(d.TotalSize)*100) + "%"
			}
		} else if d.Status == "queued" {
			status = fmt.Sprintf("Q#%d", d.QueuePosition.Int64)
		} else if d.Status == "scheduled" {
			status = "Sched " + d.ScheduledAt.String
		}

		rows = append(rows, table.Row{
			fmt.Sprintf("%d", i+1),
			d.Filename,
			formatSize(d.TotalSize),
			progress,
			speed,
			status,
			fmt.Sprintf("%d", d.ID), // hidden DB ID
		})
	}
	m.table.SetRows(rows)
}

// startDownload builds and launches a downloader for the given ID.
// It is called directly (for resume) or by the queue manager.
func (m *model) startDownload(id int64) {
	download, err := m.state.GetDownload(id)
	if err != nil || download == nil {
		return
	}

	chunks, err := m.state.GetChunks(id)
	if err != nil {
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

	ext := filepath.Ext(download.Filename)
	baseName := download.Filename[:len(download.Filename)-len(ext)]
	tmpPath := filepath.Join(m.cfg.DownloadDir, baseName+".tmp"+ext)

	downloader := core.NewDownloader(
		download.URL,
		tmpPath,
		download.TotalSize,
		chunkDefs,
		download.NumChunks,
		m.cfg.MaxRetries,
	)
	downloader.SetProgressCallback(func(chunkIndex int, downloadedSize int64, status string) {
		m.state.UpdateChunkProgress(id, chunkIndex, downloadedSize, status)
	})

	// Global limiter dibagi ke semua download. Limit individual hanya dipasang
	// kalau download ini memang punya entri sendiri — download lain tidak
	// terpengaruh.
	downloader.SetGlobalLimiter(m.globalLimiter)
	if lim := m.itemLimits[id]; lim > 0 {
		downloader.SetLimiter(core.NewLimiter(lim))
	}

	ad := &activeDownload{
		downloader: downloader,
		state:      download,
		started:    time.Now(),
	}
	m.activeDownloads[id] = ad

	go func() {
		err := downloader.Start(context.Background())
		switch {
		case errors.Is(err, context.Canceled):
			// Cancelled on purpose (pause/delete). The pause handler already
			// set the status; don't clobber it with "failed".
		case err != nil:
			m.state.UpdateDownloadStatus(id, "failed")
			ad.done.Store(true)
			m.queue.OnDone(id)
			if m.program != nil {
				m.program.Send(downloadDoneMsg{id: id})
			}
			return
		default:
			finalPath := filepath.Join(m.cfg.DownloadDir, download.Filename)
			// Check if destination file already exists.
			if _, statErr := os.Stat(finalPath); statErr == nil {
				// File exists — pause here and ask the user what to do.
				m.state.UpdateDownloadStatus(id, "paused")
				ad.done.Store(true)
				if m.program != nil {
					m.program.Send(fileConflictMsg{
						downloadID: id,
						tmpPath:    tmpPath,
						finalPath:  finalPath,
						filename:   download.Filename,
					})
				}
				return
			}
			if mvErr := core.MoveFile(tmpPath, finalPath); mvErr != nil {
				m.err = mvErr
				m.state.UpdateDownloadStatus(id, "failed")
			} else {
				m.state.UpdateDownloadStatus(id, "completed")
			}
		}
		ad.done.Store(true)
		// Notify the queue that a slot is now free.
		m.queue.OnDone(id)
		// Notify the TUI to refresh immediately.
		if m.program != nil {
			m.program.Send(downloadDoneMsg{id: id})
		}
	}()
}
