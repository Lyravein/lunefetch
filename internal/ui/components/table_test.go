package components

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/lyravein/lunefetch/internal/storage"
	"github.com/lyravein/lunefetch/internal/ui/store"
)

func TestMultiSelectTracksAndSortsIDs(t *testing.T) {
	dt := &DownloadTable{records: []*storage.DownloadRecord{{ID: 30}, {ID: 10}, {ID: 20}}}
	m := newMultiSelectHandler(dt)
	m.toggle(30)
	m.toggle(10)
	m.toggle(20)
	got := m.getSelectedIDs()
	want := []int64{10, 20, 30}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("selected IDs = %v, want %v", got, want)
		}
	}
	m.toggle(20)
	if m.count() != 2 || m.isSelected(20) {
		t.Fatalf("toggle did not remove ID 20: %v", m.getSelectedIDs())
	}
}

func TestMultiSelectRangeUsesRecordOrder(t *testing.T) {
	dt := &DownloadTable{records: []*storage.DownloadRecord{{ID: 30}, {ID: 10}, {ID: 20}, {ID: 40}}}
	m := newMultiSelectHandler(dt)
	m.mode = true
	m.selectRange(10, 40)
	got := m.getSelectedIDs()
	want := []int64{10, 20, 40}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("range IDs = %v, want %v", got, want)
		}
	}
}

func TestDownloadTableAcceptsBulkActionCallback(t *testing.T) {
	var gotIDs []int64
	var gotAction string
	dt := NewDownloadTable(nil, nil, nil, func(ids []int64, action string) {
		gotIDs = append([]int64(nil), ids...)
		gotAction = action
	})
	dt.multiHandler.toggle(30)
	dt.multiHandler.toggle(10)
	dt.onBulkAction(dt.multiHandler.getSelectedIDs(), "delete")
	if gotAction != "delete" {
		t.Fatalf("bulk action = %q, want delete", gotAction)
	}
	if len(gotIDs) != 2 || gotIDs[0] != 10 || gotIDs[1] != 30 {
		t.Fatalf("bulk IDs = %v, want [10 30]", gotIDs)
	}
}

func TestDownloadRowUsesLayoutManagedProgressAndMetadata(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	dt := NewDownloadTable(nil, nil, nil, nil)
	dt.SetRecords([]*storage.DownloadRecord{{
		ID: 1, Filename: "archive.zip", TotalSize: 1000, DownloadedSize: 250, Status: "downloading",
	}})
	dt.SetSpeeds(map[int64]float64{1: 100})
	row := newDownloadRow()
	dt.updateRow(0, row)
	if row.progress.Value != 0.25 {
		t.Fatalf("progress = %v, want 0.25", row.progress.Value)
	}
	if row.meta.Text != "250 B  •  1000 B  •  25%" {
		t.Fatalf("metadata = %q", row.meta.Text)
	}
	if row.eta.Text == "" {
		t.Fatal("ETA is empty for an active download")
	}
}

func TestDownloadTableStartsWithSelectionControlsHidden(t *testing.T) {
	dt := NewDownloadTable(nil, nil, nil, nil)
	if dt.multiHandler.mode {
		t.Fatal("download table starts in multi-select mode")
	}
	row := newDownloadRow()
	dt.records = []*storage.DownloadRecord{{ID: 1, Filename: "file.txt"}}
	dt.updateRow(0, row)
	if !row.check.Hidden {
		t.Fatal("selection checkbox is visible outside selection mode")
	}
	dt.multiHandler.toggleMode()
	dt.updateRow(0, row)
	if row.check.Hidden {
		t.Fatal("selection checkbox remains hidden in selection mode")
	}
}

func TestSidebarSelectionAndMinimumWidth(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	var selected store.DownloadStatus
	sb := NewSidebar(func(status store.DownloadStatus) { selected = status }, nil, nil)
	if got := sb.Container().MinSize().Width; got < 260 {
		t.Fatalf("sidebar minimum width = %v, want at least 260", got)
	}
	sb.SelectFilter(store.StatusFailed)
	if selected != store.StatusFailed {
		t.Fatalf("selected = %q, want failed", selected)
	}
}

// Queue reordering is only meaningful for a download that is still waiting, so
// the two entries appear for "queued" rows and nowhere else.
func TestRowMenuOffersQueueReorderOnlyForQueuedRows(t *testing.T) {
	dt := NewDownloadTable(nil, nil, nil, nil)
	dt.window = test.NewWindow(nil)
	defer dt.window.Close()

	labels := func(status string) map[string]bool {
		got := map[string]bool{}
		for _, it := range dt.rowMenuItems(&storage.DownloadRecord{ID: 1, Status: status}) {
			got[it.Label] = true
		}
		return got
	}

	queued := labels("queued")
	if !queued["Move Up in Queue"] || !queued["Move Down in Queue"] {
		t.Errorf("queued row is missing reorder entries: %v", queued)
	}

	for _, status := range []string{"downloading", "completed", "paused", "failed"} {
		got := labels(status)
		if got["Move Up in Queue"] || got["Move Down in Queue"] {
			t.Errorf("status %q must not offer queue reordering: %v", status, got)
		}
	}
}

func TestRowMenuQueueItemsFireDirectionalActions(t *testing.T) {
	var actions []string
	dt := NewDownloadTable(nil, nil, func(id int64, action string) {
		actions = append(actions, action)
	}, nil)
	dt.window = test.NewWindow(nil)
	defer dt.window.Close()

	for _, it := range dt.rowMenuItems(&storage.DownloadRecord{ID: 7, Status: "queued"}) {
		if it.Label == "Move Up in Queue" || it.Label == "Move Down in Queue" {
			it.Action()
		}
	}
	if len(actions) != 2 || actions[0] != "queue_up" || actions[1] != "queue_down" {
		t.Fatalf("actions = %v, want [queue_up queue_down]", actions)
	}
}

// Open File must open the file. It previously shared a menu entry with a
// destructive re-download, so a completed row had no way to be opened at all.
func TestCompletedRowOffersOpenAndDownloadAgainSeparately(t *testing.T) {
	var actions []string
	dt := NewDownloadTable(nil, nil, func(id int64, action string) {
		actions = append(actions, action)
	}, nil)
	dt.window = test.NewWindow(nil)
	defer dt.window.Close()

	items := dt.rowMenuItems(&storage.DownloadRecord{ID: 1, Status: "completed", Filename: "f.zip"})

	byLabel := map[string]*fyne.MenuItem{}
	for _, it := range items {
		byLabel[it.Label] = it
	}
	open, ok := byLabel["Open File"]
	if !ok {
		t.Fatal("completed row has no Open File entry")
	}
	again, ok := byLabel["Download Again…"]
	if !ok {
		t.Fatal("completed row has no Download Again entry")
	}

	open.Action()
	again.Action()
	if len(actions) != 2 || actions[0] != "open_file" || actions[1] != "download_again" {
		t.Fatalf("actions = %v, want [open_file download_again]", actions)
	}
}

// Re-fetching is destructive, so it must not appear on a row whose bytes are
// still wanted or in flight.
func TestDownloadAgainOnlyOnFinishedRows(t *testing.T) {
	dt := NewDownloadTable(nil, nil, nil, nil)
	dt.window = test.NewWindow(nil)
	defer dt.window.Close()

	has := func(status, label string) bool {
		for _, it := range dt.rowMenuItems(&storage.DownloadRecord{ID: 1, Status: status}) {
			if it.Label == label {
				return true
			}
		}
		return false
	}

	for _, status := range []string{"completed", "cancelled"} {
		if !has(status, "Download Again…") {
			t.Errorf("status %q should offer Download Again", status)
		}
	}
	for _, status := range []string{"downloading", "queued", "paused", "failed"} {
		if has(status, "Download Again…") {
			t.Errorf("status %q must not offer Download Again", status)
		}
	}
}

func TestCompactProgressSetValueClampsAndUpdates(t *testing.T) {
	p := newCompactProgress()
	p.SetValue(-0.5)
	if p.Value != 0 {
		t.Fatalf("SetValue(-0.5) = %v, want 0", p.Value)
	}
	p.SetValue(1.5)
	if p.Value != 1 {
		t.Fatalf("SetValue(1.5) = %v, want 1", p.Value)
	}
	p.SetValue(0.42)
	if p.Value != 0.42 {
		t.Fatalf("SetValue(0.42) = %v, want 0.42", p.Value)
	}
}

// TestUpdateRowReusesExistingValues verifies that updateRow only writes
// when the underlying value has changed. A download that is paused, queued,
// or finished with a steady speed would otherwise rewrite every label and
// icon on every 500ms tick.
func TestUpdateRowReusesExistingValues(t *testing.T) {
	dt := NewDownloadTable(nil, nil, nil, nil)
	dt.records = []*storage.DownloadRecord{{
		ID:             1,
		Filename:       "ubuntu-24.04.iso",
		TotalSize:      5_000_000_000,
		DownloadedSize: 2_500_000_000,
		Status:         "downloading",
	}}
	dt.speeds = map[int64]float64{1: 1_000_000}
	row := newDownloadRow()
	dt.updateRow(0, row)

	// Snapshot of the current state, then apply identical data again.
	prevName := row.name.Text
	prevMeta := row.meta.Text
	prevStatus := row.status.Text
	prevSpeed := row.speed.Text
	prevEta := row.eta.Text
	prevIcon := row.icon.Resource
	prevCheck := row.check.Checked
	prevProgress := row.progress.Value

	dt.updateRow(0, row)
	if row.name.Text != prevName {
		t.Fatalf("name rewritten: %q -> %q", prevName, row.name.Text)
	}
	if row.meta.Text != prevMeta {
		t.Fatalf("meta rewritten: %q -> %q", prevMeta, row.meta.Text)
	}
	if row.status.Text != prevStatus {
		t.Fatalf("status rewritten: %q -> %q", prevStatus, row.status.Text)
	}
	if row.speed.Text != prevSpeed {
		t.Fatalf("speed rewritten: %q -> %q", prevSpeed, row.speed.Text)
	}
	if row.eta.Text != prevEta {
		t.Fatalf("eta rewritten: %q -> %q", prevEta, row.eta.Text)
	}
	if row.icon.Resource != prevIcon {
		t.Fatalf("icon replaced when filename unchanged")
	}
	if row.check.Checked != prevCheck {
		t.Fatalf("check toggled when selection unchanged")
	}
	if row.progress.Value != prevProgress {
		t.Fatalf("progress rewritten: %v -> %v", prevProgress, row.progress.Value)
	}
}

// TestUpdateRowReactsToProgress ensures progress changes still get applied
// even when most other fields are identical.
func TestUpdateRowReactsToProgress(t *testing.T) {
	dt := NewDownloadTable(nil, nil, nil, nil)
	dt.records = []*storage.DownloadRecord{{
		ID: 1, Filename: "f.bin", TotalSize: 1000, DownloadedSize: 100, Status: "downloading",
	}}
	row := newDownloadRow()
	dt.updateRow(0, row)
	if row.progress.Value != 0.1 {
		t.Fatalf("first progress = %v, want 0.1", row.progress.Value)
	}
	dt.records[0].DownloadedSize = 750
	dt.updateRow(0, row)
	if row.progress.Value != 0.75 {
		t.Fatalf("updated progress = %v, want 0.75", row.progress.Value)
	}
}
