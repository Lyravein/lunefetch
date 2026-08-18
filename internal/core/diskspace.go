package core

import "fmt"

func validateAvailableSpace(availableBytes uint64, size int64) error {
	availableMB := availableBytes / (1024 * 1024)
	requiredMB := size / (1024 * 1024)

	if availableMB < uint64(requiredMB) {
		return fmt.Errorf("insufficient disk space: need %d MB, have %d MB", requiredMB, availableMB)
	}
	return nil
}
