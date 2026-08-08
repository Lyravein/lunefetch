package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lyravein/lunefetch/internal/core"
)

func payload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

// throttledRangeServer serves ~ 'bytesPerReq' then a tiny delay, so a download
// takes long enough to interrupt (pause) mid-flight.
func throttledRangeServer(data []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		rng := r.Header.Get("Range")
		start, end := 0, len(data)-1
		code := http.StatusOK
		if rng != "" {
			spec := strings.TrimPrefix(rng, "bytes=")
			parts := strings.SplitN(spec, "-", 2)
			start, _ = strconv.Atoi(parts[0])
			if len(parts) == 2 && parts[1] != "" {
				end, _ = strconv.Atoi(parts[1])
			}
			code = http.StatusPartialContent
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		}
		seg := data[start : end+1]
		w.Header().Set("Content-Length", strconv.Itoa(len(seg)))
		w.WriteHeader(code)
		// stream slowly
		flusher, _ := w.(http.Flusher)
		const step = 4 * 1024
		for i := 0; i < len(seg); i += step {
			j := i + step
			if j > len(seg) {
				j = len(seg)
			}
			w.Write(seg[i:j])
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(20 * time.Millisecond)
		}
	}))
}

// buildDownloaderFromDB mirrors exactly what the TUI's startDownload does.
func buildDownloaderFromDB(t *testing.T, sm *StateManager, id int64, url, tmp string, retries int) *core.Downloader {
	t.Helper()
	dl, err := sm.GetDownload(id)
	if err != nil || dl == nil {
		t.Fatalf("GetDownload: %v", err)
	}
	chunks, err := sm.GetChunks(id)
	if err != nil {
		t.Fatalf("GetChunks: %v", err)
	}
	defs := make([]core.Chunk, len(chunks))
	for i, c := range chunks {
		defs[i] = core.Chunk{Index: c.ChunkIndex, Start: c.StartByte, End: c.EndByte, Downloaded: c.DownloadedSize}
	}
	d := core.NewDownloader(url, tmp, dl.TotalSize, defs, dl.NumChunks, retries)
	d.SetValidators(dl.ETag.String, dl.LastModified.String)
	d.SetHTTPClient(&http.Client{Transport: http.DefaultTransport})
	d.SetProgressCallback(func(ci int, size int64, status string) {
		sm.UpdateChunkProgress(id, ci, size, status)
	})
	return d
}

func TestPauseResumeEndToEnd(t *testing.T) {
	data := payload(256 * 1024)
	srv := throttledRangeServer(data)
	defer srv.Close()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	sm, err := NewStateManager(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sm.Close()

	numChunks := 4
	saveDir := t.TempDir()
	id, err := sm.CreateDownload(srv.URL, "file.bin", saveDir, "", int64(len(data)), true, numChunks)
	if err != nil {
		t.Fatal(err)
	}
	chunks := core.CalculateChunks(int64(len(data)), numChunks)
	sb := make([]int64, len(chunks))
	eb := make([]int64, len(chunks))
	for i, c := range chunks {
		sb[i], eb[i] = c.Start, c.End
	}
	if err := sm.CreateChunks(id, sb, eb); err != nil {
		t.Fatal(err)
	}

	tmp := filepath.Join(saveDir, "file.tmp.bin")

	// --- First run: start, then PAUSE mid-flight ---
	d1 := buildDownloaderFromDB(t, sm, id, srv.URL, tmp, 3)
	var wg sync.WaitGroup
	wg.Add(1)
	var firstErr error
	go func() {
		defer wg.Done()
		firstErr = d1.Start(context.Background())
	}()
	time.Sleep(150 * time.Millisecond) // let some bytes land
	d1.Cancel()                        // pause
	sm.UpdateDownloadStatus(id, "paused")
	wg.Wait()
	t.Logf("first run returned: %v", firstErr)

	dl, _ := sm.GetDownload(id)
	t.Logf("after pause: status=%s downloaded=%d/%d", dl.Status, dl.DownloadedSize, dl.TotalSize)
	if dl.DownloadedSize == 0 {
		t.Fatalf("BUG: nothing persisted after pause; resume will restart from 0")
	}
	if dl.DownloadedSize >= int64(len(data)) {
		t.Skip("download finished before we could pause; rerun")
	}

	// --- Resume: rebuild from DB and finish ---
	d2 := buildDownloaderFromDB(t, sm, id, srv.URL, tmp, 3)
	if err := d2.Start(context.Background()); err != nil {
		t.Fatalf("resume Start returned error: %v", err)
	}

	got, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(data) {
		t.Fatalf("size mismatch after resume: got %d want %d", len(got), len(data))
	}
	for i := range got {
		if got[i] != data[i] {
			t.Fatalf("BUG: content corrupt at byte %d after resume: got %d want %d", i, got[i], data[i])
		}
	}
	t.Log("resume produced byte-perfect file")
}
