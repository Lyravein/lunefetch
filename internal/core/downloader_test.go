package core

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// makePayload returns deterministic bytes so tests can assert exact content.
func makePayload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

// rangeServer serves payload with Range support and records how many bytes it
// actually sent, so tests can prove resume skips already-downloaded bytes.
func rangeServer(t *testing.T, payload []byte, served *int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		rng := r.Header.Get("Range")
		if rng == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			atomic.AddInt64(served, int64(len(payload)))
			w.WriteHeader(http.StatusOK)
			w.Write(payload)
			return
		}

		// Parse "bytes=start-end".
		spec := strings.TrimPrefix(rng, "bytes=")
		parts := strings.SplitN(spec, "-", 2)
		start, _ := strconv.Atoi(parts[0])
		end := len(payload) - 1
		if len(parts) == 2 && parts[1] != "" {
			end, _ = strconv.Atoi(parts[1])
		}
		if start < 0 || end >= len(payload) || start > end {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}

		seg := payload[start : end+1]
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.Header().Set("Content-Length", strconv.Itoa(len(seg)))
		atomic.AddInt64(served, int64(len(seg)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(seg)
	}))
}

func TestCalculateChunks(t *testing.T) {
	cases := []struct {
		size      int64
		numChunks int
		want      int
	}{
		{1000, 4, 4},
		{1000, 1, 1},
		{0, 4, 1},   // zero size collapses to a single chunk
		{3, 4, 3},   // more chunks than bytes clamps to bytes
		{1001, 4, 4}, // remainder distributed
	}
	for _, c := range cases {
		chunks := CalculateChunks(c.size, c.numChunks)
		if len(chunks) != c.want {
			t.Errorf("CalculateChunks(%d,%d): got %d chunks, want %d", c.size, c.numChunks, len(chunks), c.want)
			continue
		}
		if c.size <= 0 {
			continue
		}
		// Chunks must be contiguous and cover [0, size-1] exactly.
		var next int64
		for i, ch := range chunks {
			if ch.Start != next {
				t.Errorf("size %d chunk %d: start %d, want %d", c.size, i, ch.Start, next)
			}
			next = ch.End + 1
		}
		if next != c.size {
			t.Errorf("size %d: chunks cover %d bytes, want %d", c.size, next, c.size)
		}
	}
}

func TestNewDownloaderResumeInit(t *testing.T) {
	chunks := []Chunk{
		{Index: 0, Start: 0, End: 99, Downloaded: 100}, // already complete
		{Index: 1, Start: 100, End: 199, Downloaded: 40},
	}
	d := NewDownloader("http://x", "/tmp/x", 200, chunks, 2, 3)
	p := d.GetProgress()

	if p.DownloadedSize != 140 {
		t.Errorf("DownloadedSize = %d, want 140", p.DownloadedSize)
	}
	if p.Chunks[0].Status != "completed" {
		t.Errorf("chunk 0 status = %q, want completed", p.Chunks[0].Status)
	}
	if p.Chunks[1].Status != "pending" {
		t.Errorf("chunk 1 status = %q, want pending", p.Chunks[1].Status)
	}
	if p.Chunks[1].DownloadedSize != 40 {
		t.Errorf("chunk 1 downloaded = %d, want 40", p.Chunks[1].DownloadedSize)
	}
}

func TestMoveFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	want := []byte("hello world")
	if err := os.WriteFile(src, want, 0644); err != nil {
		t.Fatal(err)
	}
	if err := MoveFile(src, dst); err != nil {
		t.Fatalf("MoveFile: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source still exists after move")
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("dst content = %q, want %q", got, want)
	}
}

func TestDownloadFull(t *testing.T) {
	payload := makePayload(64 * 1024)
	var served int64
	srv := rangeServer(t, payload, &served)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	chunks := CalculateChunks(int64(len(payload)), 4)
	d := NewDownloader(srv.URL, dst, int64(len(payload)), chunks, 4, 2)

	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("downloaded content mismatch (len got=%d want=%d)", len(got), len(payload))
	}
}

// TestResumeSkipsDownloaded proves resume re-fetches only the missing bytes:
// it pre-seeds the first half of every chunk on disk, marks it as downloaded,
// and asserts the server serves only the remaining half while the final file
// is byte-for-byte correct.
func TestResumeSkipsDownloaded(t *testing.T) {
	payload := makePayload(64 * 1024)
	var served int64
	srv := rangeServer(t, payload, &served)
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}

	chunks := CalculateChunks(int64(len(payload)), 4)
	var expectRemaining int64
	for i := range chunks {
		size := chunks[i].End - chunks[i].Start + 1
		half := size / 2
		// Seed the first half of this chunk at its absolute offset.
		if _, err := f.WriteAt(payload[chunks[i].Start:chunks[i].Start+half], chunks[i].Start); err != nil {
			t.Fatal(err)
		}
		chunks[i].Downloaded = half
		expectRemaining += size - half
	}
	f.Close()

	d := NewDownloader(srv.URL, dst, int64(len(payload)), chunks, 4, 2)
	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("resumed content mismatch (len got=%d want=%d)", len(got), len(payload))
	}
	if s := atomic.LoadInt64(&served); s != expectRemaining {
		t.Errorf("server served %d bytes, want %d (resume should skip already-downloaded)", s, expectRemaining)
	}
}
