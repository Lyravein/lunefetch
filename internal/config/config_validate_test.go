package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateRejectsOutOfRangeValues(t *testing.T) {
	cases := map[string]func(*Config){
		"max_concurrent too low":   func(c *Config) { c.MaxConcurrent = 0 },
		"max_concurrent too high":  func(c *Config) { c.MaxConcurrent = 65 },
		"max_retries negative":     func(c *Config) { c.MaxRetries = -1 },
		"max_retries too high":     func(c *Config) { c.MaxRetries = 21 },
		"retry_backoff too low":    func(c *Config) { c.RetryBackoffS = 0 },
		"retry_backoff too high":   func(c *Config) { c.RetryBackoffS = 301 },
		"min_free_space negative":  func(c *Config) { c.MinFreeSpaceMB = -1 },
		"timeout too low":          func(c *Config) { c.Timeout = 0 },
		"timeout too high":         func(c *Config) { c.Timeout = 3601 },
		"chunk count zero":         func(c *Config) { c.ChunkRules.Small = 0 },
		"chunk count too high":     func(c *Config) { c.ChunkRules.XLarge = 65 },
		"thresholds not ascending": func(c *Config) { c.MediumSize = c.SmallSize },
		"negative speed limit":     func(c *Config) { c.GlobalSpeedLimit = -1 },
		"negative retention":       func(c *Config) { c.HistoryRetentionDays = -1 },
		"proxy missing scheme":     func(c *Config) { c.ProxyURL = "127.0.0.1:8080" },
		"proxy bad scheme":         func(c *Config) { c.ProxyURL = "ftp://127.0.0.1:8080" },
		"proxy bad port":           func(c *Config) { c.ProxyURL = "http://127.0.0.1:99999" },
	}
	for name, mutate := range cases {
		cfg := Default()
		mutate(cfg)
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: Validate accepted an invalid config", name)
		}
	}
}

func TestValidateAcceptsDefaultsAndSupportedProxies(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("default config rejected: %v", err)
	}
	for _, proxy := range []string{"http://127.0.0.1:8080", "https://proxy.example:3128", "socks5://127.0.0.1:9050"} {
		cfg := Default()
		cfg.ProxyURL = proxy
		if err := cfg.Validate(); err != nil {
			t.Errorf("proxy %q rejected: %v", proxy, err)
		}
	}
}

func TestSaveRejectsInvalidConfigBeforeWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Default()
	cfg.path = path
	cfg.MaxConcurrent = 0

	if err := cfg.Save(); err == nil {
		t.Fatal("Save accepted an invalid config")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("Save wrote a file despite failing validation")
	}
}

func TestSaveThenLoadRoundTripsThroughItsOwnPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Default()
	cfg.path = path
	cfg.MaxConcurrent = 5
	cfg.RetryBackoffS = 3
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	loaded := Default()
	if err := unmarshal(raw, loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.MaxConcurrent != 5 || loaded.RetryBackoffS != 3 {
		t.Fatalf("round trip lost values: concurrent=%d backoff=%d", loaded.MaxConcurrent, loaded.RetryBackoffS)
	}
}

// Save must replace the config atomically and leave no temp files behind, so a
// crash mid-write can never produce a truncated, unparseable config.
func TestSaveIsAtomicAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.path = path
	if err := cfg.Save(); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	cfg.MaxConcurrent = 7
	if err := cfg.Save(); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "config.yaml" {
			t.Errorf("leftover file %q, want only config.yaml", entry.Name())
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	// Windows has no POSIX mode bits; Go reports 0666 there regardless of the
	// Chmod call, so the confidentiality check is Unix-only.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("config mode = %v, want 0600", perm)
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	reloaded := Default()
	if err := unmarshal(raw, reloaded); err != nil {
		t.Fatalf("saved config is not parseable: %v", err)
	}
	if reloaded.MaxConcurrent != 7 {
		t.Fatalf("max_concurrent = %d, want 7", reloaded.MaxConcurrent)
	}
}

// Paths must be absolute on every supported platform. They previously came from
// $HOME, which is normally unset on Windows and produced relative paths.
func TestUserPathsAreAbsolute(t *testing.T) {
	for name, got := range map[string]string{
		"config path":  DefaultPath(),
		"download dir": Default().DownloadDir,
	} {
		if !filepath.IsAbs(got) {
			t.Errorf("%s = %q, want an absolute path", name, got)
		}
		if strings.HasPrefix(got, ".") {
			t.Errorf("%s = %q, want it outside the working directory", name, got)
		}
	}
}
