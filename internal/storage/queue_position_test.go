package storage

import (
	"path/filepath"
	"testing"
)

func newQueueState(t *testing.T) *StateManager {
	t.Helper()
	sm, err := NewStateManager(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	t.Cleanup(func() { sm.Close() })
	return sm
}

func newQueuedRow(t *testing.T, sm *StateManager, name string, pos int64) int64 {
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

func positionOf(t *testing.T, sm *StateManager, id int64) int64 {
	t.Helper()
	rec, err := sm.GetDownload(id)
	if err != nil {
		t.Fatalf("get download: %v", err)
	}
	if !rec.QueuePosition.Valid {
		t.Fatalf("download %d has no queue position", id)
	}
	return rec.QueuePosition.Int64
}

func TestMoveQueuePositionSwapsAdjacentEntries(t *testing.T) {
	sm := newQueueState(t)
	first := newQueuedRow(t, sm, "first.bin", 1)
	second := newQueuedRow(t, sm, "second.bin", 2)

	if err := sm.MoveQueuePosition(second, -1); err != nil {
		t.Fatalf("MoveQueuePosition: %v", err)
	}
	if got := positionOf(t, sm, second); got != 1 {
		t.Fatalf("moved item position = %d, want 1", got)
	}
	if got := positionOf(t, sm, first); got != 2 {
		t.Fatalf("displaced item position = %d, want 2", got)
	}
}

// Moving past either end must clamp and leave the queue a gap-free 1..N range.
func TestMoveQueuePositionClampsToQueueBounds(t *testing.T) {
	sm := newQueueState(t)
	first := newQueuedRow(t, sm, "first.bin", 1)
	second := newQueuedRow(t, sm, "second.bin", 2)
	third := newQueuedRow(t, sm, "third.bin", 3)

	if err := sm.MoveQueuePosition(first, -5); err != nil {
		t.Fatalf("move up past start: %v", err)
	}
	if got := positionOf(t, sm, first); got != 1 {
		t.Fatalf("position after moving past start = %d, want 1", got)
	}

	if err := sm.MoveQueuePosition(first, 99); err != nil {
		t.Fatalf("move down past end: %v", err)
	}
	if got := positionOf(t, sm, first); got != 3 {
		t.Fatalf("position after moving past end = %d, want 3", got)
	}

	seen := map[int64]bool{}
	for _, id := range []int64{first, second, third} {
		pos := positionOf(t, sm, id)
		if pos < 1 || pos > 3 {
			t.Fatalf("position %d out of range for id %d", pos, id)
		}
		if seen[pos] {
			t.Fatalf("duplicate queue position %d", pos)
		}
		seen[pos] = true
	}
}

// A soft-deleted row still carries a queue_position; it must never be swapped
// with, and must not be movable itself.
func TestMoveQueuePositionIgnoresDeletedAndNonQueuedRows(t *testing.T) {
	sm := newQueueState(t)
	live := newQueuedRow(t, sm, "live.bin", 1)
	deleted := newQueuedRow(t, sm, "deleted.bin", 2)
	if err := sm.DeleteDownload(deleted); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	if err := sm.MoveQueuePosition(live, 1); err != nil {
		t.Fatalf("MoveQueuePosition on live row: %v", err)
	}
	if got := positionOf(t, sm, live); got != 1 {
		t.Fatalf("live position = %d, want it unchanged at 1", got)
	}

	if err := sm.MoveQueuePosition(deleted, -1); err != nil {
		t.Fatalf("MoveQueuePosition on deleted row: %v", err)
	}

	id, err := sm.CreateDownloadWithChunks("https://example.com/idle.bin", "idle.bin", t.TempDir(), "Other", 10, true, []int64{0}, []int64{9}, "", "")
	if err != nil {
		t.Fatalf("create download: %v", err)
	}
	if err := sm.MoveQueuePosition(id, -1); err != nil {
		t.Fatalf("MoveQueuePosition on non-queued row: %v", err)
	}
	rec, err := sm.GetDownload(id)
	if err != nil {
		t.Fatalf("get download: %v", err)
	}
	if rec.QueuePosition.Valid {
		t.Fatal("a non-queued download was given a queue position")
	}
}

func TestShiftQueuePositionsCompactsGaps(t *testing.T) {
	sm := newQueueState(t)
	a := newQueuedRow(t, sm, "a.bin", 10)
	b := newQueuedRow(t, sm, "b.bin", 40)
	c := newQueuedRow(t, sm, "c.bin", 90)

	if err := sm.ShiftQueuePositions(); err != nil {
		t.Fatalf("ShiftQueuePositions: %v", err)
	}
	for want, id := range []int64{a, b, c} {
		if got := positionOf(t, sm, id); got != int64(want+1) {
			t.Fatalf("position for id %d = %d, want %d", id, got, want+1)
		}
	}
}

// Chunk cascade deletes depend on foreign keys being enabled for every
// connection, which is why the pragma now lives in the DSN.
func TestPragmasAreEnforced(t *testing.T) {
	sm := newQueueState(t)

	var foreignKeys, busyTimeout int
	if err := sm.DB().QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	if err := sm.DB().QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout <= 0 {
		t.Fatalf("busy_timeout = %d, want a positive timeout", busyTimeout)
	}
}

// ResetProgress backs the "Download Again" flow. It must clear every recorded
// byte, because the caller deletes the file on disk straight afterwards: any
// surviving progress would make the next run resume against bytes that are gone.
func TestResetProgressClearsDownloadAndChunkBytes(t *testing.T) {
	sm := newQueueState(t)

	id, err := sm.CreateDownloadWithChunks("https://example.com/f.zip", "f.zip", t.TempDir(),
		"Compressed", 100, true, []int64{0, 50}, []int64{49, 99}, "etag", "mod")
	if err != nil {
		t.Fatalf("create download: %v", err)
	}
	if err := sm.UpdateChunkProgress(id, 0, 50, "completed"); err != nil {
		t.Fatalf("chunk 0 progress: %v", err)
	}
	if err := sm.UpdateChunkProgress(id, 1, 50, "completed"); err != nil {
		t.Fatalf("chunk 1 progress: %v", err)
	}
	if err := sm.UpdateDownloadStatus(id, "completed"); err != nil {
		t.Fatalf("set status: %v", err)
	}

	if err := sm.ResetProgress(id, "pending"); err != nil {
		t.Fatalf("ResetProgress: %v", err)
	}

	rec, err := sm.GetDownload(id)
	if err != nil {
		t.Fatalf("get download: %v", err)
	}
	if rec.DownloadedSize != 0 {
		t.Errorf("downloaded_size = %d, want 0", rec.DownloadedSize)
	}
	if rec.Status != "pending" {
		t.Errorf("status = %q, want pending", rec.Status)
	}
	if rec.QueuePosition.Valid {
		t.Errorf("queue_position = %v, want NULL", rec.QueuePosition.Int64)
	}
	// The URL and filename must survive: the flow reuses the record instead of
	// re-resolving the URL over the network.
	if rec.URL != "https://example.com/f.zip" || rec.Filename != "f.zip" {
		t.Errorf("record identity changed: url=%q filename=%q", rec.URL, rec.Filename)
	}

	chunks, err := sm.GetChunks(id)
	if err != nil {
		t.Fatalf("get chunks: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(chunks))
	}
	for _, c := range chunks {
		if c.DownloadedSize != 0 || c.Status != "pending" {
			t.Errorf("chunk %d = (%d bytes, %q), want (0, pending)", c.ChunkIndex, c.DownloadedSize, c.Status)
		}
		if c.StartByte == c.EndByte {
			t.Errorf("chunk %d lost its byte range", c.ChunkIndex)
		}
	}
}

// A missing row must be reported, not silently ignored: the caller deletes the
// user's file immediately after a successful reset.
func TestResetProgressFailsForUnknownDownload(t *testing.T) {
	sm := newQueueState(t)
	if err := sm.ResetProgress(9999, "pending"); err == nil {
		t.Fatal("ResetProgress accepted a nonexistent download")
	}
}

func TestEnqueueDownloadAssignsNextPosition(t *testing.T) {
	sm := newQueueState(t)
	first := newQueuedRow(t, sm, "first.bin", 1)
	second := newQueuedRow(t, sm, "second.bin", 2)
	third, err := sm.CreateDownloadWithChunks("https://example.com/third.bin", "third.bin", t.TempDir(), "Other", 10, true, []int64{0}, []int64{9}, "", "")
	if err != nil {
		t.Fatalf("create third download: %v", err)
	}
	if err := sm.EnqueueDownload(third); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if got := positionOf(t, sm, third); got != 3 {
		t.Fatalf("third position = %d, want 3", got)
	}
	for _, id := range []int64{first, second} {
		if got := positionOf(t, sm, id); got < 1 || got > 3 {
			t.Fatalf("existing position for %d = %d, want 1..3", id, got)
		}
	}
}

func TestMarkDownloadingClearsQueuePosition(t *testing.T) {
	sm := newQueueState(t)
	id := newQueuedRow(t, sm, "queued.bin", 1)
	if err := sm.MarkDownloading(id); err != nil {
		t.Fatalf("mark downloading: %v", err)
	}
	rec, err := sm.GetDownload(id)
	if err != nil {
		t.Fatalf("get download: %v", err)
	}
	if rec.Status != "downloading" || rec.QueuePosition.Valid {
		t.Fatalf("record = status %q, queue position valid=%v; want downloading, NULL", rec.Status, rec.QueuePosition.Valid)
	}
}

func TestResetProgressFailsForDeletedDownload(t *testing.T) {
	sm := newQueueState(t)
	id := newQueuedRow(t, sm, "gone.zip", 1)
	if err := sm.DeleteDownload(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := sm.ResetProgress(id, "pending"); err == nil {
		t.Fatal("ResetProgress accepted a soft-deleted download")
	}
}
