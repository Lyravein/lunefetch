package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestDownloaderRespectsLimiter memverifikasi throttle benar-benar memperlambat
// download nyata lewat HTTP server lokal.
func TestDownloaderRespectsLimiter(t *testing.T) {
	const size = 256 * 1024 // 256 KiB
	payload := strings.Repeat("x", size)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "data.bin", time.Now(), strings.NewReader(payload))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "out.bin")

	chunks := CalculateChunks(size, 4)
	d := NewDownloader(srv.URL, dst, size, chunks, 4, 0)

	// 256 KiB/s untuk 256 KiB => ~1 detik.
	d.SetGlobalLimiter(NewLimiter(256 * 1024))

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := d.Start(ctx); err != nil {
		t.Fatalf("download failed: %v", err)
	}
	elapsed := time.Since(start)

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() != size {
		t.Errorf("size = %d, want %d", info.Size(), size)
	}

	// Burst hanya 32 KiB, jadi hampir seluruh transfer kena throttle.
	if elapsed < 600*time.Millisecond {
		t.Errorf("download of %d bytes at 256 KiB/s took %v, expected throttling",
			size, elapsed)
	}
	t.Logf("throttled download took %v", elapsed)
}

// TestDownloaderUnthrottledIsFaster memastikan tanpa limiter download jauh
// lebih cepat, membuktikan perlambatan di test sebelumnya berasal dari limiter.
func TestDownloaderUnthrottledIsFaster(t *testing.T) {
	const size = 512 * 1024
	payload := strings.Repeat("x", size)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "data.bin", time.Now(), strings.NewReader(payload))
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	chunks := CalculateChunks(size, 4)
	d := NewDownloader(srv.URL, dst, size, chunks, 4, 0)

	start := time.Now()
	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("download failed: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("unthrottled download took %v, expected to be fast", elapsed)
	}
	t.Logf("unthrottled download took %v", elapsed)
}

// TestGlobalLimiterSharedAcrossDownloads memverifikasi dua download yang share
// satu global limiter total bandwidth-nya dibatasi bersama.
func TestGlobalLimiterSharedAcrossDownloads(t *testing.T) {
	const size = 256 * 1024
	payload := strings.Repeat("y", size)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "data.bin", time.Now(), strings.NewReader(payload))
	}))
	defer srv.Close()

	shared := NewLimiter(256 * 1024)
	tmpDir := t.TempDir()

	done := make(chan error, 2)
	start := time.Now()

	for i := 0; i < 2; i++ {
		dst := filepath.Join(tmpDir, "out"+strconv.Itoa(i)+".bin")
		chunks := CalculateChunks(size, 2)
		d := NewDownloader(srv.URL, dst, size, chunks, 2, 0)
		d.SetGlobalLimiter(shared)
		go func() { done <- d.Start(context.Background()) }()
	}

	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatalf("download %d failed: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	// Total 512 KiB melalui limiter 256 KiB/s bersama => ~2 detik.
	if elapsed < 1200*time.Millisecond {
		t.Errorf("two downloads sharing 256 KiB/s took %v, expected throttling", elapsed)
	}
	t.Logf("two shared downloads took %v", elapsed)
}
