// Package singleinstance provides a cross-platform advisory lock that keeps a
// second Lunefetch process from sharing one user's database and .part files.
//
// Two instances are genuinely unsafe here: the SQLite handle is limited to a
// single connection, and both processes would write the same
// `.lunefetch-<id>.part` file for the same download.
package singleinstance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrAlreadyRunning is returned when another process already holds the lock.
var ErrAlreadyRunning = errors.New("another Lunefetch instance is already running")

// Lock is an acquired single-instance lock. Call Release on shutdown.
type Lock struct {
	path string
	file *os.File
}

// Acquire takes the lock file in dir. It returns ErrAlreadyRunning if another
// live process holds it. A lock left behind by a crashed process is reclaimed.
func Acquire(dir string) (*Lock, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}
	path := filepath.Join(dir, "lunefetch.lock")

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := lockFile(file); err != nil {
		file.Close()
		if errors.Is(err, ErrAlreadyRunning) {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("lock: %w", err)
	}

	// Record the pid for diagnostics. The lock itself is held by the OS, not by
	// this content, so a stale pid can never cause a false positive.
	if err := file.Truncate(0); err == nil {
		fmt.Fprintf(file, "%d\n", os.Getpid())
	}

	return &Lock{path: path, file: file}, nil
}

// Release drops the lock. It is safe to call more than once.
func (l *Lock) Release() {
	if l == nil || l.file == nil {
		return
	}
	unlockFile(l.file) //nolint:errcheck // the fd is closed next regardless
	l.file.Close()
	l.file = nil
}
