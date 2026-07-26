package ui

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
	return NewModel(sm, cfg)
}

func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestSpeedPageOpensOnBothCases(t *testing.T) {
	for _, key := range []string{"l", "L"} {
		m := newTestModel(t)
		m.width, m.height = 100, 30
		m.Update(runes(key))
		if m.currentPage != pageSpeedLimit {
			t.Errorf("key %q: page = %d, want %d", key, m.currentPage, pageSpeedLimit)
		}
		if !strings.Contains(m.View(), "Speed Limit") {
			t.Errorf("key %q: view missing Speed Limit", key)
		}
	}
}

func TestSpeedScopeToggleAndSave(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 30
	m.Update(runes("l"))

	if m.speedScope != scopeGlobal {
		t.Fatalf("scope = %d, want global", m.speedScope)
	}

	// Ketik 500k lalu simpan sebagai global.
	for _, r := range "500k" {
		m.Update(runes(string(r)))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.globalLimit != 500*1024 {
		t.Errorf("globalLimit = %d, want %d", m.globalLimit, 500*1024)
	}
	if m.perDownloadLimit != 0 {
		t.Errorf("perDownloadLimit = %d, want 0 (harus tidak ikut berubah)", m.perDownloadLimit)
	}

	// Buka lagi, toggle ke per-download, simpan 2m.
	m.Update(runes("l"))
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.speedScope != scopePerDownload {
		t.Fatalf("after tab scope = %d, want per-download", m.speedScope)
	}
	// Input harus menampilkan nilai per-download (kosong), bukan global.
	if v := m.speedInput.Value(); v != "" {
		t.Errorf("after tab input = %q, want empty (nilai per-download)", v)
	}

	for _, r := range "2m" {
		m.Update(runes(string(r)))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.perDownloadLimit != 2*1024*1024 {
		t.Errorf("perDownloadLimit = %d, want %d", m.perDownloadLimit, 2*1024*1024)
	}
	if m.globalLimit != 500*1024 {
		t.Errorf("globalLimit = %d, want tetap %d", m.globalLimit, 500*1024)
	}
}

func TestSpeedPageEscDoesNotSave(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 30
	m.Update(runes("l"))
	for _, r := range "9m" {
		m.Update(runes(string(r)))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.globalLimit != 0 {
		t.Errorf("globalLimit = %d, want 0 setelah esc", m.globalLimit)
	}
	if m.currentPage != pageList {
		t.Errorf("page = %d, want pageList", m.currentPage)
	}
}
