//go:build windows

package userpath

import (
	"os"
	"path/filepath"
)

// dataDir uses %LocalAppData%, the conventional location for per-machine user
// state on Windows. A ~/.local/share path would be wrong there, and
// XDG_DATA_HOME is not a Windows convention.
func dataDir() string {
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "lunefetch")
	}
	if dir := os.Getenv("LocalAppData"); dir != "" {
		return filepath.Join(dir, "lunefetch")
	}
	return filepath.Join(Home(), "AppData", "Local", "lunefetch")
}
