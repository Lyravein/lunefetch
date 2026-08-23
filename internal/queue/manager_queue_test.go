package queue

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lyravein/lunefetch/internal/storage"
)

func timeoutAfterSeconds(n int) <-chan time.Time {
	return time.After(time.Duration(n) * time.Second)
}

func newTestState(t *testing.T) *storage.StateManager {
	t.Helper()
	sm, err := storage.NewStateManager(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	t.Cleanup(func() { sm.Close() })
	return sm
}

func newQueued(t *testing.T, sm *storage.StateManager, name string, pos int64) int64 {
	t.Helper()
	id, err := sm.CreateDownloadWithChunks("https://example.com/"+name, name, t.TempDir(), "Other", 10, true, []int64{0}, []int64{9}, "", "")
	if err != nil {
		t.Fatalf("create download: %v", err)
	}
	if err := sm.UpdateDownloadStatus(id, "queued"); err != nil {
		t.Fatalf("set status: %v", err)
	}
	if err := sm.SetQueuePosition(id, &pos); err != nil {
		t.Fatalf("set position: %v", err)
	}
	return id
}

// drainQueue must not spin forever when the next queued row is already active.
// It previously looped while holding the manager mutex, deadlocking the queue.
func TestDrainTerminatesWhenNextItemIsAlreadyActive(t *testing.T) {
	sm := newTestState(t)
	id := newQueued(t, sm, "busy.bin", 1)

	m := NewManager(sm, 4, func(int64) {})
	m.active[id] = struct{}{}

	done := make(chan struct{})
	go func() {
		m.Drain()
		close(done)
	}()

	select {
	case <-done:
	case <-timeoutAfterSeconds(5):
		t.Fatal("Drain did not terminate; the queue loop is spinning")
	}

	// The manager must still be usable, i.e. the mutex was released.
	if got := m.Active(); got != 1 {
		t.Fatalf("active = %d, want 1", got)
	}
}

// Queue execution order must follow ascending queue_position.
func TestDrainStartsLowestQueuePositionFirst(t *testing.T) {
	sm := newTestState(t)
	second := newQueued(t, sm, "second.bin", 2)
	first := newQueued(t, sm, "first.bin", 1)

	started := make(chan int64, 2)
	m := NewManager(sm, 1, func(id int64) { started <- id })
	m.Drain()

	select {
	case got := <-started:
		if got != first {
			t.Fatalf("started id = %d, want the lowest position %d", got, first)
		}
	case <-timeoutAfterSeconds(5):
		t.Fatal("no download was started")
	}

	if _, active := m.active[second]; active {
		t.Fatal("the higher queue position was started despite a limit of 1")
	}
}

// A scheduled download promoted directly to running must not keep a stale
// queue position, which would put it ahead of genuinely queued work later.
func TestEnqueueScheduledClearsQueuePosition(t *testing.T) {
	sm := newTestState(t)
	id := newQueued(t, sm, "scheduled.bin", 7)

	m := NewManager(sm, 4, func(int64) {})
	if err := m.EnqueueScheduled(id); err != nil {
		t.Fatalf("EnqueueScheduled: %v", err)
	}

	rec, err := sm.GetDownload(id)
	if err != nil {
		t.Fatalf("get download: %v", err)
	}
	if rec.QueuePosition.Valid {
		t.Fatalf("queue_position = %d, want cleared once running", rec.QueuePosition.Int64)
	}
}

// Soft-deleted rows must not inflate the next assigned queue position.
func TestEnqueueIgnoresDeletedRowsWhenAssigningPositions(t *testing.T) {
	sm := newTestState(t)
	stale := newQueued(t, sm, "stale.bin", 50)
	if err := sm.DeleteDownload(stale); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	fresh, err := sm.CreateDownloadWithChunks("https://example.com/fresh.bin", "fresh.bin", t.TempDir(), "Other", 10, true, []int64{0}, []int64{9}, "", "")
	if err != nil {
		t.Fatalf("create download: %v", err)
	}

	m := NewManager(sm, 1, func(int64) {})
	m.active[int64(-1)] = struct{}{} // occupy the only slot so fresh gets queued

	if _, err := m.TryStart(fresh); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	rec, err := sm.GetDownload(fresh)
	if err != nil {
		t.Fatalf("get download: %v", err)
	}
	if !rec.QueuePosition.Valid || rec.QueuePosition.Int64 != 1 {
		t.Fatalf("queue_position = %v, want 1", rec.QueuePosition)
	}
}
