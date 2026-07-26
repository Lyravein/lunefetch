package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/core"
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
)

type model struct {
	state           *storage.StateManager
	cfg             *config.Config
	program         *tea.Program
	downloads       []storage.DownloadRecord
	activeDownloads map[int64]*activeDownload
	currentPage     page
	table           table.Model
	selectedID      int64
	urlInput        textinput.Model
	spinner         spinner.Model
	err             error
	width           int
	height          int
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

	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := &model{
		state:           sm,
		cfg:             cfg,
		activeDownloads: make(map[int64]*activeDownload),
		currentPage:     pageList,
		table:           t,
		urlInput:        ti,
		spinner:         sp,
	}

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
	const fixedCols = 4 + 10 + 10 + 10 + 10 + 12
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
		{Title: "Status", Width: 10},
		{Title: "_id", Width: 0},
	})

	// Reserve rows for: title (1) + blank (1) + header (1) + help (2) + padding (2) = 7
	tableHeight := m.height - 7
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
	m.updateTable()
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

func (m *model) updateTable() {
	rows := make([]table.Row, 0, len(m.downloads))
	for i, d := range m.downloads {
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
		default:
			if mvErr := core.MoveFile(tmpPath, filepath.Join(m.cfg.DownloadDir, download.Filename)); mvErr != nil {
				m.err = mvErr
				m.state.UpdateDownloadStatus(id, "failed")
			} else {
				m.state.UpdateDownloadStatus(id, "completed")
			}
		}
		ad.done.Store(true)
		// Notify the TUI to refresh immediately so status updates without
		// waiting for the next tick.
		if m.program != nil {
			m.program.Send(downloadDoneMsg{id: id})
		}
	}()
}
