package uitui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/storage"
)

func newTestModel(t *testing.T) *model {
	t.Helper()
	db := filepath.Join(t.TempDir(), "test.db")
	sm, err := storage.NewStateManager(db)
	if err != nil {
		t.Fatalf("state manager: %v", err)
	}
	t.Cleanup(func() { sm.Close() })
	cfg := config.Default()
	// Arahkan Save() ke temp dir supaya test tidak menimpa config asli user.
	cfg.SetPath(filepath.Join(t.TempDir(), "config.yaml"))
	m := NewModel(sm, cfg)
	m.width, m.height = 100, 30
	return m
}

// addDownload menyisipkan satu download dengan status tertentu lalu me-refresh
// tabel supaya baris-nya bisa di-highlight seperti di UI sebenarnya.
func addDownload(t *testing.T, m *model, filename, status string) int64 {
	t.Helper()
	id, err := m.state.CreateDownload("http://example.com/"+filename, filename, t.TempDir(), "", 1024, true, 1)
	if err != nil {
		t.Fatalf("create download: %v", err)
	}
	if err := m.state.UpdateDownloadStatus(id, status); err != nil {
		t.Fatalf("set status: %v", err)
	}
	m.loadDownloads()
	return id
}

func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func typeText(m *model, s string) {
	for _, r := range s {
		m.Update(runes(string(r)))
	}
}

func TestSpeedPageOpensOnBothCases(t *testing.T) {
	for _, key := range []string{"l", "L"} {
		m := newTestModel(t)
		m.Update(runes(key))
		if m.currentPage != pageSpeedLimit {
			t.Errorf("key %q: page = %d, want %d", key, m.currentPage, pageSpeedLimit)
		}
		if !strings.Contains(m.View(), "Speed Limit") {
			t.Errorf("key %q: view missing Speed Limit", key)
		}
	}
}

// TestPerFileLimitDoesNotAffectOthers adalah regresi untuk bug utama: limit
// per-file dulu di-apply ke semua activeDownloads, jadi terasa seperti limit
// global. Sekarang hanya download yang di-highlight yang boleh kena.
func TestPerFileLimitDoesNotAffectOthers(t *testing.T) {
	m := newTestModel(t)
	a := addDownload(t, m, "a.bin", "downloading")
	b := addDownload(t, m, "b.bin", "downloading")

	m.table.SetCursor(0) // highlight a.bin
	m.Update(runes("l"))
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.speedScope != scopeSelected {
		t.Fatalf("scope = %d, want scopeSelected", m.speedScope)
	}
	if m.speedTargetID != a {
		t.Fatalf("speedTargetID = %d, want %d (a.bin)", m.speedTargetID, a)
	}

	typeText(m, "500k")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if got := m.itemLimits[a]; got != 500*1024 {
		t.Errorf("limit a.bin = %d, want %d", got, 500*1024)
	}
	if _, ok := m.itemLimits[b]; ok {
		t.Error("b.bin ikut kena limit, padahal tidak dipilih")
	}
	if m.globalLimit != 0 {
		t.Errorf("globalLimit = %d, want 0 (tidak boleh tersentuh)", m.globalLimit)
	}
}

// TestPerFileLimitsAreIndependent memastikan tiap download bisa punya limit
// sendiri yang berbeda.
func TestPerFileLimitsAreIndependent(t *testing.T) {
	m := newTestModel(t)
	a := addDownload(t, m, "a.bin", "downloading")
	b := addDownload(t, m, "b.bin", "paused")

	setLimit := func(row int, text string) {
		m.table.SetCursor(row)
		m.Update(runes("l"))
		m.Update(tea.KeyMsg{Type: tea.KeyTab})
		typeText(m, text)
		m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}

	setLimit(0, "500k")
	setLimit(1, "2m")

	if got := m.itemLimits[a]; got != 500*1024 {
		t.Errorf("limit a.bin = %d, want %d", got, 500*1024)
	}
	if got := m.itemLimits[b]; got != 2*1024*1024 {
		t.Errorf("limit b.bin = %d, want %d", got, 2*1024*1024)
	}
}

// TestNoSelectionFallsBackToGlobal: kalau list kosong, scope per-file tidak
// tersedia dan tab tidak boleh memindahkan scope.
func TestNoSelectionFallsBackToGlobal(t *testing.T) {
	m := newTestModel(t)
	m.Update(runes("l"))

	if m.hasSpeedTarget() {
		t.Error("hasSpeedTarget() = true padahal list kosong")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.speedScope != scopeGlobal {
		t.Errorf("scope = %d, want tetap global saat tidak ada pilihan", m.speedScope)
	}

	typeText(m, "500k")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.globalLimit != 500*1024 {
		t.Errorf("globalLimit = %d, want %d", m.globalLimit, 500*1024)
	}
}

// TestCompletedDownloadCannotBeLimited: limit hanya relevan untuk download yang
// masih akan menarik bandwidth.
func TestCompletedDownloadCannotBeLimited(t *testing.T) {
	for _, status := range []string{"completed", "failed"} {
		m := newTestModel(t)
		addDownload(t, m, "done.bin", status)
		m.table.SetCursor(0)
		m.Update(runes("l"))

		if m.hasSpeedTarget() {
			t.Errorf("status %q: hasSpeedTarget() = true, want false", status)
		}
	}
}

func TestLimitableStatuses(t *testing.T) {
	limitable := []string{"queued", "paused", "downloading"}
	for _, s := range limitable {
		if !limitableStatus(s) {
			t.Errorf("limitableStatus(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"completed", "failed"} {
		if limitableStatus(s) {
			t.Errorf("limitableStatus(%q) = true, want false", s)
		}
	}
}

// TestItemLimitSurvivesForResume: limit download yang di-pause harus tetap
// tersimpan supaya berlaku lagi saat di-resume.
func TestItemLimitSurvivesForResume(t *testing.T) {
	m := newTestModel(t)
	id := addDownload(t, m, "a.bin", "paused")

	m.table.SetCursor(0)
	m.Update(runes("l"))
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	typeText(m, "500k")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if got := m.itemLimits[id]; got != 500*1024 {
		t.Fatalf("limit = %d, want %d", got, 500*1024)
	}

	// Pause (key "p") tidak boleh membuang limit.
	m.table.SetCursor(0)
	m.Update(runes("p"))
	if got := m.itemLimits[id]; got != 500*1024 {
		t.Errorf("setelah pause limit = %d, want tetap %d", got, 500*1024)
	}
}

// TestItemLimitClearedWhenDone: entri dibuang saat download selesai supaya map
// tidak menumpuk.
func TestItemLimitClearedWhenDone(t *testing.T) {
	m := newTestModel(t)
	id := addDownload(t, m, "a.bin", "downloading")
	m.itemLimits[id] = 500 * 1024

	m.Update(downloadDoneMsg{id: id})

	if _, ok := m.itemLimits[id]; ok {
		t.Error("limit masih ada setelah downloadDoneMsg")
	}
}

// TestClearItemLimit: input kosong menghapus limit, bukan menyimpan 0.
func TestClearItemLimit(t *testing.T) {
	m := newTestModel(t)
	id := addDownload(t, m, "a.bin", "downloading")
	m.itemLimits[id] = 500 * 1024

	m.table.SetCursor(0)
	m.Update(runes("l"))
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Input sudah terisi "500k" dari limit lama; kosongkan.
	m.speedInput.SetValue("")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if _, ok := m.itemLimits[id]; ok {
		t.Error("limit masih ada setelah di-set kosong")
	}
}

func TestSpeedScopeToggleShowsScopeValue(t *testing.T) {
	m := newTestModel(t)
	id := addDownload(t, m, "a.bin", "downloading")
	m.globalLimit = 500 * 1024
	m.itemLimits[id] = 2 * 1024 * 1024

	m.table.SetCursor(0)
	m.Update(runes("l"))
	if v := m.speedInput.Value(); v != "500k" {
		t.Errorf("input global = %q, want 500k", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if v := m.speedInput.Value(); v != "2m" {
		t.Errorf("input per-file = %q, want 2m", v)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if v := m.speedInput.Value(); v != "500k" {
		t.Errorf("input kembali ke global = %q, want 500k", v)
	}
}

func TestSpeedPageEscDoesNotSave(t *testing.T) {
	m := newTestModel(t)
	m.Update(runes("l"))
	typeText(m, "9m")
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.globalLimit != 0 {
		t.Errorf("globalLimit = %d, want 0 setelah esc", m.globalLimit)
	}
	if m.currentPage != pageList {
		t.Errorf("page = %d, want pageList", m.currentPage)
	}
}
