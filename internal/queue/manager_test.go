package queue

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lyravein/lunefetch/internal/storage"
)

func newTestManager(t *testing.T, max int) (*Manager, *storage.StateManager, <-chan int64) {
	t.Helper()
	sm, err := storage.NewStateManager(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sm.Close() })
	started := make(chan int64, 10)
	return NewManager(sm, max, func(id int64) { started <- id }), sm, started
}

func addDownload(t *testing.T, sm *storage.StateManager, name string) int64 {
	t.Helper()
	id, err := sm.CreateDownload("https://example.com/"+name, name, t.TempDir(), "", 1, false, 1)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func receiveID(t *testing.T, ch <-chan int64) int64 {
	t.Helper()
	select {
	case id := <-ch:
		return id
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for start")
		return 0
	}
}

func TestOnDoneIsIdempotent(t *testing.T) {
	m, sm, started := newTestManager(t, 1)
	first := addDownload(t, sm, "first")
	second := addDownload(t, sm, "second")
	third := addDownload(t, sm, "third")

	m.TryStart(first)
	m.TryStart(second)
	m.TryStart(third)
	if got := receiveID(t, started); got != first {
		t.Fatalf("first start = %d, want %d", got, first)
	}

	m.OnDone(first)
	if got := receiveID(t, started); got != second {
		t.Fatalf("second start = %d, want %d", got, second)
	}
	m.OnDone(first)
	select {
	case got := <-started:
		t.Fatalf("duplicate completion started %d", got)
	case <-time.After(50 * time.Millisecond):
	}
	if m.Active() != 1 {
		t.Fatalf("active = %d, want 1", m.Active())
	}
}

func TestTryStartSameIDOnce(t *testing.T) {
	m, sm, started := newTestManager(t, 2)
	id := addDownload(t, sm, "same")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.TryStart(id)
		}()
	}
	wg.Wait()
	if got := receiveID(t, started); got != id {
		t.Fatalf("start = %d, want %d", got, id)
	}
	select {
	case got := <-started:
		t.Fatalf("duplicate start for %d", got)
	case <-time.After(50 * time.Millisecond):
	}
	if m.Active() != 1 {
		t.Fatalf("active = %d, want 1", m.Active())
	}
}

func TestIncreasingLimitDrainsQueue(t *testing.T) {
	m, sm, started := newTestManager(t, 1)
	first := addDownload(t, sm, "first")
	second := addDownload(t, sm, "second")
	m.TryStart(first)
	m.TryStart(second)
	receiveID(t, started)
	m.SetMaxConcurrent(2)
	if got := receiveID(t, started); got != second {
		t.Fatalf("drained start = %d, want %d", got, second)
	}
}
