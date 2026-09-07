package store

import (
	"path/filepath"
	"testing"

	"github.com/lyravein/lunefetch/internal/storage"
)

func newTestStore(t *testing.T) (*storage.StateManager, *DownloadStore) {
	t.Helper()
	sm, err := storage.NewStateManager(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sm.Close() })
	return sm, NewDownloadStore(sm)
}

func TestDescendingSortIsStrictAndDeterministic(t *testing.T) {
	sm, store := newTestStore(t)

	first, err := sm.CreateDownload("https://example.com/1", "same.bin", t.TempDir(), "", 10, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sm.CreateDownload("https://example.com/2", "same.bin", t.TempDir(), "", 10, true, 1)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Load(); err != nil {
		t.Fatal(err)
	}
	store.SetSort(ColName, false)
	records := store.Downloads()
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if records[0].ID != second || records[1].ID != first {
		t.Fatalf("descending tie order = [%d, %d], want [%d, %d]", records[0].ID, records[1].ID, second, first)
	}
}

func TestFilterCategoryAndSearchCompose(t *testing.T) {
	sm, s := newTestStore(t)
	video, err := sm.CreateDownload("https://example.com/demo.mp4", "Demo Movie.mp4", t.TempDir(), "Media", 10, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := sm.UpdateDownloadStatus(video, "failed"); err != nil {
		t.Fatal(err)
	}
	music, err := sm.CreateDownload("https://example.com/demo.mp3", "Demo Song.mp3", t.TempDir(), "Media", 10, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := sm.UpdateDownloadStatus(music, "failed"); err != nil {
		t.Fatal(err)
	}
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	s.SetFilter(StatusFailed)
	s.SetSearch("movie")
	got := s.Downloads()
	if len(got) != 1 || got[0].ID != video {
		t.Fatalf("failed + search = %#v, want only video %d", got, video)
	}

	s.SetCategory("Media")
	got = s.Downloads()
	if len(got) != 1 || got[0].ID != video {
		t.Fatalf("category must clear status but preserve search: got %#v, want video %d", got, video)
	}
	s.SetSearch("song")
	got = s.Downloads()
	if len(got) != 1 || got[0].ID != music {
		t.Fatalf("media + song = %#v, want only music %d", got, music)
	}
	s.SetCategory("All")
	s.SetSearch("")
	if got = s.Downloads(); len(got) != 2 {
		t.Fatalf("UI All category returned %d records, want 2", len(got))
	}
}

func TestAllReturnsUnfilteredRecords(t *testing.T) {
	sm, s := newTestStore(t)
	first, err := sm.CreateDownload("https://example.com/1", "alpha.bin", t.TempDir(), "", 10, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sm.CreateDownload("https://example.com/2", "beta.bin", t.TempDir(), "", 10, true, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	s.SetFilter(StatusDownloading) // no matches; view is empty

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d records, want 2 (filter must not apply)", len(all))
	}
	if all[0].ID != first {
		t.Fatalf("All() order = [%d, %d], want %d first", all[0].ID, all[1].ID, first)
	}
}

func TestStatusCounts(t *testing.T) {
	sm, s := newTestStore(t)
	a, err := sm.CreateDownload("https://example.com/a", "a.bin", t.TempDir(), "", 10, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := sm.UpdateDownloadStatus(a, "completed"); err != nil {
		t.Fatal(err)
	}
	b, err := sm.CreateDownload("https://example.com/b", "b.bin", t.TempDir(), "", 10, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := sm.UpdateDownloadStatus(b, "downloading"); err != nil {
		t.Fatal(err)
	}
	if _, err := sm.CreateDownload("https://example.com/c", "c.bin", t.TempDir(), "", 10, true, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	counts, total := s.StatusCounts()
	if total != 3 {
		t.Fatalf("StatusCounts total = %d, want 3", total)
	}
	if counts["completed"] != 1 {
		t.Fatalf("completed count = %d, want 1", counts["completed"])
	}
	if counts["downloading"] != 1 {
		t.Fatalf("downloading count = %d, want 1", counts["downloading"])
	}
	if counts["pending"] != 1 {
		t.Fatalf("pending count = %d, want 1", counts["pending"])
	}
}
