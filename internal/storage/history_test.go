package storage_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lyravein/lunefetch/internal/storage"
)

func newSM(t *testing.T) *storage.StateManager {
	t.Helper()
	sm, err := storage.NewStateManager(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("state manager: %v", err)
	}
	t.Cleanup(func() { sm.Close() })
	return sm
}

func createDL(t *testing.T, sm *storage.StateManager, filename, status string) int64 {
	t.Helper()
	id, err := sm.CreateDownload("http://example.com/"+filename, filename, 1024, true, 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := sm.UpdateDownloadStatus(id, status); err != nil {
		t.Fatalf("status: %v", err)
	}
	return id
}

// ---------------------------------------------------------------------------
// Soft delete
// ---------------------------------------------------------------------------

func TestSoftDeleteHidesFromList(t *testing.T) {
	sm := newSM(t)
	id := createDL(t, sm, "file.bin", "completed")

	if err := sm.DeleteDownload(id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	list, _ := sm.ListDownloads()
	for _, d := range list {
		if d.ID == id {
			t.Errorf("deleted download still visible in ListDownloads")
		}
	}
}

func TestSoftDeleteAppearsInHistory(t *testing.T) {
	sm := newSM(t)
	id := createDL(t, sm, "file.bin", "completed")
	sm.DeleteDownload(id) //nolint:errcheck

	history, err := sm.ListDeleted()
	if err != nil {
		t.Fatalf("list deleted: %v", err)
	}
	found := false
	for _, d := range history {
		if d.ID == id {
			found = true
			if !d.DeletedAt.Valid {
				t.Errorf("deleted_at should be set")
			}
			if d.Status != "completed" {
				t.Errorf("status should be preserved, got %q", d.Status)
			}
		}
	}
	if !found {
		t.Errorf("deleted download not found in history")
	}
}

func TestSoftDeletePreservesStatus(t *testing.T) {
	sm := newSM(t)
	id := createDL(t, sm, "failed.bin", "failed")
	sm.DeleteDownload(id) //nolint:errcheck

	history, _ := sm.ListDeleted()
	for _, d := range history {
		if d.ID == id && d.Status != "failed" {
			t.Errorf("status changed on delete: got %q, want failed", d.Status)
		}
	}
}

// ---------------------------------------------------------------------------
// DeleteWithFile
// ---------------------------------------------------------------------------

func TestDeleteWithFileResetsChunkProgress(t *testing.T) {
	sm := newSM(t)
	id := createDL(t, sm, "big.bin", "paused")
	sm.CreateChunks(id, []int64{0}, []int64{1023})    //nolint:errcheck
	sm.UpdateChunkProgress(id, 0, 512, "downloading") //nolint:errcheck

	if err := sm.DeleteWithFile(id); err != nil {
		t.Fatalf("delete with file: %v", err)
	}

	dl, err := sm.GetDownload(id)
	if err != nil || dl == nil {
		t.Fatalf("get download: %v", err)
	}
	if dl.DownloadedSize != 0 {
		t.Errorf("downloaded_size should be 0 after DeleteWithFile, got %d", dl.DownloadedSize)
	}

	chunks, _ := sm.GetChunks(id)
	for _, c := range chunks {
		if c.DownloadedSize != 0 {
			t.Errorf("chunk downloaded_size should be 0, got %d", c.DownloadedSize)
		}
	}
}

// ---------------------------------------------------------------------------
// Duplicate detection ignores deleted
// ---------------------------------------------------------------------------

func TestFindByURLIgnoresDeleted(t *testing.T) {
	sm := newSM(t)
	id := createDL(t, sm, "file.bin", "completed")
	sm.DeleteDownload(id) //nolint:errcheck

	rec, err := sm.FindByURL("http://example.com/file.bin")
	if err != nil {
		t.Fatalf("find by url: %v", err)
	}
	if rec != nil {
		t.Errorf("FindByURL should return nil for deleted download, got id=%d", rec.ID)
	}
}

func TestFilenameExistsIgnoresDeleted(t *testing.T) {
	sm := newSM(t)
	id := createDL(t, sm, "file.bin", "completed")
	sm.DeleteDownload(id) //nolint:errcheck

	if sm.FilenameExists("file.bin") {
		t.Errorf("FilenameExists should return false for deleted download")
	}
}

// ---------------------------------------------------------------------------
// Restore
// ---------------------------------------------------------------------------

func TestRestoreAppearsInList(t *testing.T) {
	sm := newSM(t)
	id := createDL(t, sm, "file.bin", "completed")
	sm.DeleteDownload(id) //nolint:errcheck

	if err := sm.RestoreDownload(id); err != nil {
		t.Fatalf("restore: %v", err)
	}

	list, _ := sm.ListDownloads()
	found := false
	for _, d := range list {
		if d.ID == id {
			found = true
			if d.Status != "paused" {
				t.Errorf("restored status = %q, want paused", d.Status)
			}
			if d.DeletedAt.Valid {
				t.Errorf("deleted_at should be NULL after restore")
			}
		}
	}
	if !found {
		t.Errorf("restored download not found in list")
	}
}

func TestRestoreDisappearsFromHistory(t *testing.T) {
	sm := newSM(t)
	id := createDL(t, sm, "file.bin", "completed")
	sm.DeleteDownload(id)  //nolint:errcheck
	sm.RestoreDownload(id) //nolint:errcheck

	history, _ := sm.ListDeleted()
	for _, d := range history {
		if d.ID == id {
			t.Errorf("restored download still in history")
		}
	}
}

// ---------------------------------------------------------------------------
// Purge
// ---------------------------------------------------------------------------

func TestPurgeDownload(t *testing.T) {
	sm := newSM(t)
	id := createDL(t, sm, "file.bin", "completed")
	sm.DeleteDownload(id) //nolint:errcheck

	if err := sm.PurgeDownload(id); err != nil {
		t.Fatalf("purge: %v", err)
	}

	dl, _ := sm.GetDownload(id)
	if dl != nil {
		t.Errorf("purged download should not exist, got id=%d", dl.ID)
	}
}

func TestPurgeAllDeleted(t *testing.T) {
	sm := newSM(t)
	for _, f := range []string{"a.bin", "b.bin", "c.bin"} {
		id := createDL(t, sm, f, "completed")
		sm.DeleteDownload(id) //nolint:errcheck
	}

	if err := sm.PurgeAllDeleted(); err != nil {
		t.Fatalf("purge all: %v", err)
	}

	history, _ := sm.ListDeleted()
	if len(history) != 0 {
		t.Errorf("history should be empty after PurgeAllDeleted, got %d", len(history))
	}
}

func TestPurgeOlderThan(t *testing.T) {
	sm := newSM(t)

	// Buat dua download, hapus keduanya.
	idOld := createDL(t, sm, "old.bin", "completed")
	idNew := createDL(t, sm, "new.bin", "completed")
	sm.DeleteDownload(idOld) //nolint:errcheck
	sm.DeleteDownload(idNew) //nolint:errcheck

	// Paksa deleted_at yang lama langsung via SQL.
	sm.DB().Exec( //nolint:errcheck
		`UPDATE downloads SET deleted_at = '2020-01-01 00:00:00' WHERE id = ?`, idOld,
	)

	cutoff := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := sm.PurgeOlderThan(cutoff); err != nil {
		t.Fatalf("purge older than: %v", err)
	}

	history, _ := sm.ListDeleted()
	for _, d := range history {
		if d.ID == idOld {
			t.Errorf("old entry should be purged")
		}
	}
	found := false
	for _, d := range history {
		if d.ID == idNew {
			found = true
		}
	}
	if !found {
		t.Errorf("new entry should still be in history")
	}
}

// ---------------------------------------------------------------------------
// Active download tidak dihapus saat NextInQueue
// ---------------------------------------------------------------------------

func TestNextInQueueIgnoresDeleted(t *testing.T) {
	sm := newSM(t)
	id := createDL(t, sm, "file.bin", "queued")
	pos := int64(1)
	sm.SetQueuePosition(id, &pos) //nolint:errcheck
	sm.DeleteDownload(id)         //nolint:errcheck

	next, err := sm.NextInQueue()
	if err != nil {
		t.Fatalf("next in queue: %v", err)
	}
	if next != nil {
		t.Errorf("NextInQueue should ignore deleted, got id=%d", next.ID)
	}
}
