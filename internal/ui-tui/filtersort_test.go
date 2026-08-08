package uitui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Filter tests
// ---------------------------------------------------------------------------

func TestFilterCycleAllStatuses(t *testing.T) {
	m := newTestModel(t)
	// Mula-mula filterAll (0).
	if m.statusFilter != filterAll {
		t.Fatalf("initial filter = %d, want filterAll", m.statusFilter)
	}
	// Tekan F 8x harus kembali ke filterAll.
	for i := 0; i < 8; i++ {
		m.Update(runes("F"))
	}
	if m.statusFilter != filterAll {
		t.Errorf("after 8xF filter = %d, want filterAll", m.statusFilter)
	}
}

func TestFilterHidesOtherStatuses(t *testing.T) {
	m := newTestModel(t)
	addDownload(t, m, "completed.bin", "completed")
	addDownload(t, m, "paused.bin", "paused")
	addDownload(t, m, "failed.bin", "failed")

	// Set filter ke Completed.
	m.statusFilter = filterCompleted
	m.applyView()

	if len(m.visible) != 1 {
		t.Fatalf("visible = %d, want 1 (only completed)", len(m.visible))
	}
	if m.visible[0].Filename != "completed.bin" {
		t.Errorf("visible[0] = %q, want completed.bin", m.visible[0].Filename)
	}
}

func TestFilterActiveGrouping(t *testing.T) {
	m := newTestModel(t)
	addDownload(t, m, "a.bin", "downloading")
	addDownload(t, m, "b.bin", "queued")
	addDownload(t, m, "c.bin", "paused")
	addDownload(t, m, "d.bin", "completed")
	addDownload(t, m, "e.bin", "failed")

	m.statusFilter = filterActive
	m.applyView()

	if len(m.visible) != 3 {
		t.Errorf("active filter visible = %d, want 3 (downloading+queued+paused)", len(m.visible))
	}
	for _, d := range m.visible {
		switch d.Status {
		case "downloading", "queued", "paused":
			// ok
		default:
			t.Errorf("active filter includes status %q", d.Status)
		}
	}
}

func TestFilterAllShowsEverything(t *testing.T) {
	m := newTestModel(t)
	addDownload(t, m, "a.bin", "completed")
	addDownload(t, m, "b.bin", "paused")
	addDownload(t, m, "c.bin", "failed")

	m.statusFilter = filterAll
	m.applyView()

	if len(m.visible) != 3 {
		t.Errorf("filterAll visible = %d, want 3", len(m.visible))
	}
}

func TestFilterKeyFCycles(t *testing.T) {
	m := newTestModel(t)
	// F berturut-turut harus increment statusFilter.
	m.Update(runes("F"))
	if m.statusFilter != filterActive {
		t.Errorf("after 1xF = %d, want filterActive(%d)", m.statusFilter, filterActive)
	}
	m.Update(runes("F"))
	if m.statusFilter != filterDownloading {
		t.Errorf("after 2xF = %d, want filterDownloading(%d)", m.statusFilter, filterDownloading)
	}
}

// ---------------------------------------------------------------------------
// Sort tests
// ---------------------------------------------------------------------------

func TestSortByNameAscending(t *testing.T) {
	m := newTestModel(t)
	addDownload(t, m, "zebra.bin", "completed")
	addDownload(t, m, "apple.bin", "completed")
	addDownload(t, m, "mango.bin", "completed")

	m.sortBy = sortName
	m.sortDesc = false
	m.applyView()

	if len(m.visible) != 3 {
		t.Fatalf("visible = %d, want 3", len(m.visible))
	}
	if m.visible[0].Filename != "apple.bin" || m.visible[2].Filename != "zebra.bin" {
		t.Errorf("sort asc: got %q %q %q, want apple mango zebra",
			m.visible[0].Filename, m.visible[1].Filename, m.visible[2].Filename)
	}
}

func TestSortByNameDescending(t *testing.T) {
	m := newTestModel(t)
	addDownload(t, m, "zebra.bin", "completed")
	addDownload(t, m, "apple.bin", "completed")
	addDownload(t, m, "mango.bin", "completed")

	m.sortBy = sortName
	m.sortDesc = true
	m.applyView()

	if m.visible[0].Filename != "zebra.bin" {
		t.Errorf("sort desc: first = %q, want zebra.bin", m.visible[0].Filename)
	}
}

func TestSortBySize(t *testing.T) {
	m := newTestModel(t)
	// addDownload pakai size=1024; override via state langsung.
	id1 := addDownload(t, m, "big.bin", "completed")
	id2 := addDownload(t, m, "small.bin", "completed")
	m.state.DB().Exec(`UPDATE downloads SET total_size=9999 WHERE id=?`, id1)
	m.state.DB().Exec(`UPDATE downloads SET total_size=1 WHERE id=?`, id2)
	m.loadDownloads()

	m.sortBy = sortSize
	m.sortDesc = false
	m.applyView()

	if m.visible[0].Filename != "small.bin" {
		t.Errorf("sort size asc: first = %q, want small.bin", m.visible[0].Filename)
	}
}

func TestSortKeyOCycles(t *testing.T) {
	m := newTestModel(t)
	// o cycle: Default→Name→Size→Status→Default
	fields := []sortField{sortName, sortSize, sortStatus, sortDefault}
	for i, want := range fields {
		m.Update(runes("o"))
		if m.sortBy != want {
			t.Errorf("after %dx'o' sortBy = %d, want %d", i+1, m.sortBy, want)
		}
	}
}

func TestSortKeyOReverses(t *testing.T) {
	m := newTestModel(t)
	m.sortBy = sortName
	m.Update(runes("O"))
	if !m.sortDesc {
		t.Error("O should set sortDesc=true")
	}
	m.Update(runes("O"))
	if m.sortDesc {
		t.Error("second O should set sortDesc=false")
	}
}

func TestSortDefaultNoOpOnO(t *testing.T) {
	m := newTestModel(t)
	// sortDefault tidak bisa di-reverse.
	m.Update(runes("O"))
	if m.sortDesc {
		t.Error("O on sortDefault should not set sortDesc")
	}
}

// ---------------------------------------------------------------------------
// Search tests
// ---------------------------------------------------------------------------

func TestSearchOpensOnSlash(t *testing.T) {
	m := newTestModel(t)
	m.Update(runes("/"))
	if !m.searchActive {
		t.Error("/ should set searchActive=true")
	}
}

func TestSearchLiveFilter(t *testing.T) {
	m := newTestModel(t)
	addDownload(t, m, "hello.bin", "completed")
	addDownload(t, m, "world.bin", "completed")

	m.Update(runes("/"))
	typeText(m, "hel")

	if len(m.visible) != 1 {
		t.Fatalf("visible after search 'hel' = %d, want 1", len(m.visible))
	}
	if m.visible[0].Filename != "hello.bin" {
		t.Errorf("search result = %q, want hello.bin", m.visible[0].Filename)
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	m := newTestModel(t)
	addDownload(t, m, "MyFile.bin", "completed")
	addDownload(t, m, "other.bin", "completed")

	m.Update(runes("/"))
	typeText(m, "MYFILE")

	if len(m.visible) != 1 || m.visible[0].Filename != "MyFile.bin" {
		t.Errorf("case-insensitive search failed: visible=%v", m.visible)
	}
}

func TestSearchEscRestoresPrevious(t *testing.T) {
	m := newTestModel(t)
	addDownload(t, m, "a.bin", "completed")
	addDownload(t, m, "b.bin", "completed")

	// Set query yang sudah ada.
	m.searchQuery = "a"
	m.applyView()
	prevVisible := len(m.visible) // 1

	// Buka search, ketik sesuatu, lalu esc.
	m.Update(runes("/"))
	typeText(m, "zzz")
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.searchActive {
		t.Error("esc should close search")
	}
	if m.searchQuery != "a" {
		t.Errorf("esc should restore previous query, got %q", m.searchQuery)
	}
	if len(m.visible) != prevVisible {
		t.Errorf("visible after esc = %d, want %d", len(m.visible), prevVisible)
	}
}

func TestSearchEnterLocksQuery(t *testing.T) {
	m := newTestModel(t)
	addDownload(t, m, "foo.bin", "completed")
	addDownload(t, m, "bar.bin", "completed")

	m.Update(runes("/"))
	typeText(m, "foo")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.searchActive {
		t.Error("enter should close search input")
	}
	if m.searchQuery != "foo" {
		t.Errorf("searchQuery = %q, want foo", m.searchQuery)
	}
	if len(m.visible) != 1 {
		t.Errorf("visible = %d, want 1", len(m.visible))
	}
}

func TestClearKeyResetsAll(t *testing.T) {
	m := newTestModel(t)
	addDownload(t, m, "x.bin", "completed")
	addDownload(t, m, "y.bin", "paused")

	m.statusFilter = filterCompleted
	m.searchQuery = "x"
	m.sortBy = sortName
	m.applyView()

	m.Update(runes("c"))

	if m.statusFilter != filterAll {
		t.Errorf("c: statusFilter = %d, want filterAll", m.statusFilter)
	}
	if m.searchQuery != "" {
		t.Errorf("c: searchQuery = %q, want empty", m.searchQuery)
	}
	if len(m.visible) != 2 {
		t.Errorf("c: visible = %d, want 2 (all)", len(m.visible))
	}
}

// ---------------------------------------------------------------------------
// Selection stability tests
// ---------------------------------------------------------------------------

// TestSelectionStableAfterFilter: kursor harus tetap nunjuk ke download yang
// benar (via hidden ID column) walau list menyusut karena filter.
func TestSelectionStableAfterFilter(t *testing.T) {
	m := newTestModel(t)
	addDownload(t, m, "a.bin", "completed")
	targetID := addDownload(t, m, "b.bin", "paused")
	addDownload(t, m, "c.bin", "completed")

	// Highlight b.bin (index 1 di list penuh, index 0 di list filtered paused).
	m.table.SetCursor(1)
	if got := m.selectedRowID(); got != targetID {
		t.Fatalf("pre-filter selectedRowID = %d, want %d", got, targetID)
	}

	// Filter ke Paused — list jadi 1 baris (b.bin).
	m.statusFilter = filterPaused
	m.applyView()
	m.table.SetCursor(0) // pilih satu-satunya baris

	if got := m.selectedRowID(); got != targetID {
		t.Errorf("post-filter selectedRowID = %d, want %d", got, targetID)
	}
}

// TestViewShowsFilterStatus: view harus menampilkan info filter aktif.
func TestViewShowsFilterStatus(t *testing.T) {
	m := newTestModel(t)
	m.statusFilter = filterCompleted
	m.applyView()

	v := m.View()
	if !strings.Contains(v, "filter:Completed") {
		t.Errorf("view missing filter status line, got:\n%s", v)
	}
}

func TestViewShowsSearchQuery(t *testing.T) {
	m := newTestModel(t)
	m.searchQuery = "hello"
	m.applyView()

	v := m.View()
	if !strings.Contains(v, "search:hello") {
		t.Errorf("view missing search query, got:\n%s", v)
	}
}

func TestViewShowsSortField(t *testing.T) {
	m := newTestModel(t)
	m.sortBy = sortName
	m.applyView()

	v := m.View()
	if !strings.Contains(v, "sort:Name") {
		t.Errorf("view missing sort field, got:\n%s", v)
	}
}
