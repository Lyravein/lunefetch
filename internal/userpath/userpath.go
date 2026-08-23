// Package userpath resolves per-user directories in a way that works on both
// Linux and Windows. os.Getenv("HOME") is empty on Windows, which previously
// made config, database, and API-token paths resolve relative to the process
// working directory.
package userpath

import (
	"os"
	"path/filepath"
)

// Home returns the user's home directory, falling back to $HOME and finally the
// current directory so callers always receive a usable path.
func Home() string {
	if dir, err := os.UserHomeDir(); err == nil && dir != "" {
		return dir
	}
	if dir := os.Getenv("HOME"); dir != "" {
		return dir
	}
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return "."
}

// Config returns the directory for Lunefetch configuration files.
// Linux: ~/.config/lunefetch. Windows: %AppData%\lunefetch.
func Config() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "lunefetch")
	}
	return filepath.Join(Home(), ".config", "lunefetch")
}

// Data returns the directory for Lunefetch state such as the SQLite database and
// the single-instance lock. Its layout is platform specific, so the body lives
// in data_unix.go / data_windows.go.
func Data() string {
	return dataDir()
}

// Downloads returns the default download directory.
func Downloads() string {
	return filepath.Join(Home(), "Downloads")
}
