//go:build windows

package singleinstance

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// Windows byte-range locks are mandatory, not advisory: a locked region cannot
// be read even by another handle in the same process. Locking byte 0 therefore
// made the pid unreadable, so the lock is taken on a sentinel byte past any
// plausible file content instead. Every process locks the same region, so
// mutual exclusion is unaffected.
const (
	lockOffsetLow  uint32 = 0
	lockOffsetHigh uint32 = 1 // offset 2^32
)

func lockRegion() *windows.Overlapped {
	return &windows.Overlapped{Offset: lockOffsetLow, OffsetHigh: lockOffsetHigh}
}

// lockFile takes an exclusive, non-blocking lock on the sentinel byte. Windows
// releases it when the handle closes, including on abnormal termination.
func lockFile(file *os.File) error {
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, lockRegion(),
	)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
		return ErrAlreadyRunning
	}
	return err
}

func unlockFile(file *os.File) error {
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, lockRegion())
}
