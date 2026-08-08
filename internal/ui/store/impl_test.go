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
	video, err := sm.CreateDownload("https://example.com/demo.mp4", "Demo Movie.mp4", t.TempDir(), "Videos", 10, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := sm.UpdateDownloadStatus(video, "failed"); err != nil {
		t.Fatal(err)
	}
	music, err := sm.CreateDownload("https://example.com/demo.mp3", "Demo Song.mp3", t.TempDir(), "Music", 10, true, 1)
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

	s.SetCategory("Music")
	got = s.Downloads()
	if len(got) != 0 {
		t.Fatalf("category must clear status but preserve search: got %#v", got)
	}
	s.SetSearch("song")
	got = s.Downloads()
	if len(got) != 1 || got[0].ID != music {
		t.Fatalf("music + song = %#v, want only music %d", got, music)
	}
}
