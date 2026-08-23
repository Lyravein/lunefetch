//go:build !windows

package userpath

import (
	"os"
	"path/filepath"
)

// dataDir follows the XDG base directory spec.
func dataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" && filepath.IsAbs(dir) {
		return filepath.Join(dir, "lunefetch")
	}
	return filepath.Join(Home(), ".local", "share", "lunefetch")
}
