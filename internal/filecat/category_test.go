package filecat

import "testing"

func TestAllReturnsCanonicalCategories(t *testing.T) {
	want := []Category{Compressed, Document, Media, Program, Other}
	got := All()
	if len(got) != len(want) {
		t.Fatalf("All() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("All()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFromFilenameUsesCanonicalCategories(t *testing.T) {
	tests := map[string]Category{
		"movie.mp4": Media, "song.flac": Media, "photo.PNG": Media,
		"archive.zip": Compressed, "report.pdf": Document,
		"installer.exe": Program, "unknown.bin": Other,
	}
	for filename, want := range tests {
		if got := FromFilename(filename); got != want {
			t.Errorf("FromFilename(%q) = %q, want %q", filename, got, want)
		}
	}
}

func TestFromURLStripsQueryAndFragment(t *testing.T) {
	if got := FromURL("https://example.com/report.pdf?download=1#top"); got != Document {
		t.Fatalf("FromURL() = %q, want %q", got, Document)
	}
}
