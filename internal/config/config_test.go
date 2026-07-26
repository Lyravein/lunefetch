package config

import (
	"path/filepath"
	"testing"
)

func TestSaveRoundTripUsesInjectedPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.SetPath(p)
	cfg.GlobalSpeedLimit = 500 * 1024

	if err := cfg.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Baca ulang lewat unmarshal manual (Load selalu pakai path default).
	got := Default()
	data, err := readFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := unmarshal(data, got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.GlobalSpeedLimit != 500*1024 {
		t.Errorf("GlobalSpeedLimit = %d, want %d", got.GlobalSpeedLimit, 500*1024)
	}
}

func TestSpeedLimitDefaultsToUnlimited(t *testing.T) {
	cfg := Default()
	if cfg.GlobalSpeedLimit != 0 {
		t.Errorf("default GlobalSpeedLimit = %d, want 0 (unlimited)", cfg.GlobalSpeedLimit)
	}
}

// TestMissingFieldsStayUnlimited memastikan config lama tanpa field speed limit
// tetap terbaca sebagai unlimited, bukan error.
func TestMissingFieldsStayUnlimited(t *testing.T) {
	old := []byte("download_dir: /tmp/dl\nmax_retries: 3\n")
	cfg := Default()
	if err := unmarshal(old, cfg); err != nil {
		t.Fatalf("unmarshal legacy config: %v", err)
	}
	if cfg.GlobalSpeedLimit != 0 {
		t.Errorf("legacy config GlobalSpeedLimit = %d, want 0", cfg.GlobalSpeedLimit)
	}
	if cfg.DownloadDir != "/tmp/dl" {
		t.Errorf("DownloadDir = %q, want /tmp/dl", cfg.DownloadDir)
	}
}
