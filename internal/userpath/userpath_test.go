package userpath

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Every path must be absolute on every supported platform. These used to derive
// from os.Getenv("HOME"), which is normally unset on Windows and produced paths
// relative to the process working directory.
func TestPathsAreAbsoluteAndNamespaced(t *testing.T) {
	paths := map[string]string{
		"Home":      Home(),
		"Config":    Config(),
		"Data":      Data(),
		"Downloads": Downloads(),
	}
	for name, got := range paths {
		if got == "" {
			t.Errorf("%s is empty", name)
			continue
		}
		if !filepath.IsAbs(got) {
			t.Errorf("%s = %q, want an absolute path", name, got)
		}
		if strings.HasPrefix(got, "."+string(filepath.Separator)) || got == "." {
			t.Errorf("%s = %q, want it outside the working directory", name, got)
		}
	}

	for _, name := range []string{"Config", "Data"} {
		if !strings.Contains(paths[name], "lunefetch") {
			t.Errorf("%s = %q, want it namespaced under lunefetch", name, paths[name])
		}
	}
}

// Data() and Config() must not collide: the lock file and database live in Data,
// the config and API token in Config.
func TestDataAndConfigAreDistinct(t *testing.T) {
	if Data() == Config() {
		t.Fatalf("Data and Config both resolve to %q", Data())
	}
}

func TestDataFollowsPlatformConvention(t *testing.T) {
	got := Data()
	if runtime.GOOS == "windows" {
		if strings.Contains(got, ".local") {
			t.Fatalf("Data = %q, want a Windows AppData path rather than an XDG one", got)
		}
		return
	}
	if !strings.Contains(got, filepath.Join(".local", "share")) {
		t.Fatalf("Data = %q, want an XDG data path", got)
	}
}

func TestDataHonoursXDGDataHomeOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG_DATA_HOME is not a Windows convention")
	}
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "xdg-data"))
	got := Data()
	if !strings.Contains(got, "xdg-data") {
		t.Fatalf("Data = %q, want it under XDG_DATA_HOME", got)
	}
}

// A relative XDG_DATA_HOME must be ignored rather than producing a relative
// database path.
func TestRelativeXDGDataHomeIsIgnored(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG_DATA_HOME is not a Windows convention")
	}
	t.Setenv("XDG_DATA_HOME", "relative/data")
	if got := Data(); !filepath.IsAbs(got) {
		t.Fatalf("Data = %q, want an absolute path", got)
	}
}
