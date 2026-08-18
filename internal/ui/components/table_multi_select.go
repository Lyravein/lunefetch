package components

import "sort"

// multiSelectHandler manages multi-selection state for the table
type multiSelectHandler struct {
	table    *DownloadTable
	selected map[int64]bool // download ID -> true
	mode     bool           // true = multi-select mode active
}

// newMultiSelectHandler creates a new handler attached to the table
func newMultiSelectHandler(table *DownloadTable) *multiSelectHandler {
	return &multiSelectHandler{
		table:    table,
		selected: make(map[int64]bool),
		mode:     false,
	}
}

// toggleMode switches between single and multi-select mode
func (m *multiSelectHandler) toggleMode() {
	m.mode = !m.mode
	m.selected = make(map[int64]bool)
}

// selectRange selects all items from first to last in current selection
func (m *multiSelectHandler) selectRange(firstID, lastID int64) {
	if !m.mode || len(m.table.records) == 0 {
		return
	}

	startIdx := -1
	endIdx := -1

	for i, rec := range m.table.records {
		if rec.ID == firstID {
			startIdx = i
		}
		if rec.ID == lastID {
			endIdx = i
		}
	}

	if startIdx >= 0 && endIdx >= 0 {
		for i := startIdx; i <= endIdx; i++ {
			m.selected[m.table.records[i].ID] = true
		}
	}
}

// isSelected checks if a specific download is selected
func (m *multiSelectHandler) isSelected(id int64) bool {
	return m.selected[id]
}

// clearSelection clears all selections
func (m *multiSelectHandler) clearSelection() {
	if len(m.selected) > 0 {
		m.selected = make(map[int64]bool)
	}
}

func (m *multiSelectHandler) toggle(id int64) {
	if m.selected[id] {
		delete(m.selected, id)
		return
	}
	m.selected[id] = true
}

func (m *multiSelectHandler) count() int { return len(m.selected) }

// getSelectedIDs returns list of currently selected download IDs
func (m *multiSelectHandler) getSelectedIDs() []int64 {
	result := make([]int64, 0, len(m.selected))
	for id := range m.selected {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
