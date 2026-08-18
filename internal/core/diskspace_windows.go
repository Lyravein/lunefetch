//go:build windows

package core

import (
	"fmt"
	"golang.org/x/sys/windows"
)

// DiskSpaceAvailable checks if there's enough free space for a download.
func DiskSpaceAvailable(dir string, size int64) error {
	if dir == "" || size <= 0 {
		return nil
	}

	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}

	var freeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(path, &freeBytes, nil, nil); err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}
	return validateAvailableSpace(freeBytes, size)
}
