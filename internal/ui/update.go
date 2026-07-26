package ui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lyravein/lunefetch/internal/core"
)

type downloadReadyMsg struct{ id int64 }
type downloadErrMsg struct{ err error }

// AddURLMsg is sent by the HTTP API server when the browser extension
// forwards a URL to download.
type AddURLMsg struct{ URL string }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width - 4)

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tickMsg:
		m.updateTable()
		return m, m.refreshCmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case downloadReadyMsg:
		m.startDownload(msg.id)
		m.loadDownloads()
		return m, nil

	case downloadErrMsg:
		m.err = msg.err
		return m, nil

	case AddURLMsg:
		return m, m.createDownloadCmd(msg.URL)
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
	}
	return m, nil
}

func (m *model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "n":
		m.currentPage = pageAddURL
		m.urlInput.SetValue("")
		m.urlInput.Focus()
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
		id := m.selectedRowID()
		if id > 0 {
			if ad, ok := m.activeDownloads[id]; ok {
				ad.downloader.Cancel()
				delete(m.activeDownloads, id)
			}
			m.state.DeleteDownload(id)
			m.loadDownloads()
		}

	case "r":
		id := m.selectedRowID()
		if id > 0 {
			m.startDownload(id)
			m.loadDownloads()
		}

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
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// selectedRowID returns the download ID of the currently selected table row,
// or 0 if there is no valid selection. SelectedRow may return an empty slice
// even when the table has rows (e.g. cursor out of range after a refresh),
// so both the row list and the row itself must be checked.
func (m *model) selectedRowID() int64 {
	if len(m.table.Rows()) == 0 {
		return 0
	}
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return 0
	}
	return parseID(row[0])
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

	case "enter":
		url := m.urlInput.Value()
		if url == "" {
			return m, nil
		}

		m.currentPage = pageList
		m.updateTable()
		return m, m.createDownloadCmd(url)
	}

	var cmd tea.Cmd
	m.urlInput, cmd = m.urlInput.Update(msg)
	return m, cmd
}

// createDownloadCmd fetches file info and creates the download + chunk records
// in the background, returning a message the Update loop handles on the main
// goroutine (so model state is never mutated from a stray goroutine).
func (m *model) createDownloadCmd(url string) tea.Cmd {
	return func() tea.Msg {
		info, err := core.GetFileInfo(url)
		if err != nil {
			return downloadErrMsg{err}
		}

		numChunks := m.cfg.ChunksForSize(info.Size)

		id, err := m.state.CreateDownload(url, info.Filename, info.Size, info.SupportsRange, numChunks)
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

func parseID(s string) int64 {
	var id int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			id = id*10 + int64(c-'0')
		}
	}
	return id
}
