package pages

import (
	"context"
	"sync"
	"testing"
)

// pauseActiveWorkers must cancel every active worker. PauseAll (Ctrl+P) is built
// on it; the shortcut was documented in the README but had nothing to call.
func TestPauseActiveWorkersCancelsEveryDownload(t *testing.T) {
	dp := &DownloadsPage{active: map[int64]*downloadEntry{}}

	var mu sync.Mutex
	cancelled := map[int64]bool{}
	for _, id := range []int64{1, 2, 3} {
		id := id
		_, cancel := context.WithCancel(context.Background())
		dp.active[id] = &downloadEntry{
			id: id,
			cancelFn: func() {
				mu.Lock()
				cancelled[id] = true
				mu.Unlock()
				cancel()
			},
		}
	}

	paused := dp.pauseActiveWorkers()
	if len(paused) != 3 {
		t.Fatalf("paused %d downloads, want 3", len(paused))
	}

	mu.Lock()
	defer mu.Unlock()
	for _, id := range []int64{1, 2, 3} {
		if !cancelled[id] {
			t.Errorf("download %d was not cancelled", id)
		}
	}
}

func TestPauseActiveWorkersWithNothingRunningIsANoOp(t *testing.T) {
	dp := &DownloadsPage{active: map[int64]*downloadEntry{}}
	if paused := dp.pauseActiveWorkers(); len(paused) != 0 {
		t.Fatalf("paused %d downloads, want 0", len(paused))
	}
}
