//go:build !windows

package core

import (
	"fmt"
	"syscall"
)

// DiskSpaceAvailable checks if there's enough free space for a download.
func DiskSpaceAvailable(dir string, size int64) error {
	if dir == "" || size <= 0 {
		return nil
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}

	availableBytes := uint64(stat.Bavail) * uint64(stat.Bsize)
	return validateAvailableSpace(availableBytes, size)
}
