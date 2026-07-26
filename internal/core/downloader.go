package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// readBufSize adalah ukuran buffer baca per iterasi. Nilai ini juga menentukan
// burst maksimum limiter (lihat burstFor) supaya throttle langsung terasa.
const readBufSize = 32 * 1024

type Chunk struct {
	Index      int
	Start      int64
	End        int64
	Downloaded int64
}

// ProgressCallback is invoked when a chunk's progress changes so callers can
// persist it (e.g. to the database). It must be safe for concurrent use.
type ProgressCallback func(chunkIndex int, downloadedSize int64, status string)

type ChunkProgress struct {
	ChunkIndex     int
	DownloadedSize int64
	TotalSize      int64
	Status         string
	Speed          float64
	Error          error
}

type DownloadProgress struct {
	TotalSize      int64
	DownloadedSize int64
	Chunks         []ChunkProgress
	StartTime      time.Time
}

type Downloader struct {
	url       string
	filePath  string
	totalSize int64
	chunks    []Chunk
	numChunks int
	retries   int

	file       *os.File
	progress   DownloadProgress
	done       chan struct{}
	cancel     context.CancelFunc
	mu         sync.RWMutex
	active     int32
	speedBuf   []int64
	onProgress ProgressCallback
	limiter    *Limiter // per-download, nil = unlimited
	globalLim  *Limiter // shared across all downloads, nil = unlimited
}

func NewDownloader(url, filePath string, totalSize int64, chunks []Chunk, numChunks, retries int) *Downloader {
	chunkProgress := make([]ChunkProgress, len(chunks))
	var alreadyDownloaded int64
	for i, c := range chunks {
		total := c.End - c.Start + 1
		status := "pending"
		if c.Downloaded >= total && total > 0 {
			status = "completed"
		}
		chunkProgress[i] = ChunkProgress{
			ChunkIndex:     c.Index,
			DownloadedSize: c.Downloaded,
			TotalSize:      total,
			Status:         status,
		}
		alreadyDownloaded += c.Downloaded
	}

	return &Downloader{
		url:       url,
		filePath:  filePath,
		totalSize: totalSize,
		chunks:    chunks,
		numChunks: numChunks,
		retries:   retries,
		progress: DownloadProgress{
			TotalSize:      totalSize,
			DownloadedSize: alreadyDownloaded,
			Chunks:         chunkProgress,
			StartTime:      time.Now(),
		},
		done:     make(chan struct{}),
		speedBuf: make([]int64, 0, 10),
	}
}

// SetProgressCallback registers a callback invoked whenever a chunk's progress
// or status changes. Call before Start.
func (d *Downloader) SetProgressCallback(cb ProgressCallback) {
	d.onProgress = cb
}

// SetLimiter sets the per-download bandwidth limiter.
// Bisa dipanggil kapan saja, termasuk saat download sedang berjalan.
// Pass nil untuk unlimited.
func (d *Downloader) SetLimiter(lim *Limiter) {
	d.mu.Lock()
	d.limiter = lim
	d.mu.Unlock()
}

// SetGlobalLimiter sets the shared limiter used by every active download.
// Karena limiter ini dibagi, total bandwidth semua download akan dibatasi
// bersama-sama. Pass nil untuk unlimited.
func (d *Downloader) SetGlobalLimiter(lim *Limiter) {
	d.mu.Lock()
	d.globalLim = lim
	d.mu.Unlock()
}

// emitProgress invokes the progress callback if set. Must be called without
// holding d.mu since the callback may perform slow I/O (e.g. a DB write).
func (d *Downloader) emitProgress(chunkIndex int, downloadedSize int64, status string) {
	if d.onProgress != nil {
		d.onProgress(chunkIndex, downloadedSize, status)
	}
}

// flushProgress persists the current downloaded size and status of every chunk
// via the progress callback. It is called when Start returns so that a pause
// landing between trackSpeed ticks never loses progress. Safe to call with no
// callback set.
func (d *Downloader) flushProgress() {
	if d.onProgress == nil {
		return
	}
	d.mu.RLock()
	type snap struct {
		index      int
		downloaded int64
		status     string
	}
	snaps := make([]snap, len(d.progress.Chunks))
	for i, cp := range d.progress.Chunks {
		status := cp.Status
		// A chunk still "downloading" when we stop is really paused; persist a
		// resumable status rather than a transient one.
		if status == "downloading" || status == "retrying" {
			status = "paused"
		}
		snaps[i] = snap{cp.ChunkIndex, cp.DownloadedSize, status}
	}
	d.mu.RUnlock()

	for _, s := range snaps {
		d.emitProgress(s.index, s.downloaded, s.status)
	}
}

func (d *Downloader) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithCancel(ctx)
	d.mu.Lock()
	d.cancel = cancel
	d.done = make(chan struct{}) // reset done channel for each Start call
	d.mu.Unlock()
	defer cancel()

	var err error
	d.file, err = os.OpenFile(d.filePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer d.file.Close()

	if err := d.file.Truncate(d.totalSize); err != nil {
		return fmt.Errorf("truncate file: %w", err)
	}

	sem := make(chan struct{}, d.numChunks)
	var wg sync.WaitGroup

	// Seed lastDownloaded with the bytes already on disk (from a resume) so the
	// first speed tick doesn't report a spike counting pre-existing progress.
	d.mu.RLock()
	lastDownloaded := d.progress.DownloadedSize
	d.mu.RUnlock()

	go d.trackSpeed(ctx, &lastDownloaded)

	for i := range d.chunks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)
		atomic.AddInt32(&d.active, 1)

		go func(ch Chunk) {
			defer func() {
				<-sem
				wg.Done()
				atomic.AddInt32(&d.active, -1)
			}()

			d.downloadChunkWithRetry(ctx, ch)
		}(d.chunks[i])
	}

	wg.Wait()
	close(d.done)

	// Flush every chunk's current progress to persistent storage. trackSpeed
	// only persists once per second, so a pause/cancel that lands between ticks
	// would otherwise lose the last bytes and force resume to restart. This
	// guarantees the DB reflects exactly what's on disk before we return.
	d.flushProgress()

	// If the context was cancelled (pause/delete), report that specifically so
	// callers can distinguish an intentional stop from a real failure.
	if ctx.Err() != nil {
		return ctx.Err()
	}

	d.mu.RLock()
	allDone := true
	for _, cp := range d.progress.Chunks {
		if cp.Status != "completed" {
			allDone = false
			break
		}
	}
	d.mu.RUnlock()

	if !allDone {
		return fmt.Errorf("some chunks failed")
	}

	return nil
}

func (d *Downloader) downloadChunkWithRetry(ctx context.Context, ch Chunk) {
	var lastErr error
	for attempt := 0; attempt <= d.retries; attempt++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := d.downloadChunk(ctx, ch)
		if err == nil {
			d.mu.Lock()
			d.progress.Chunks[ch.Index].Status = "completed"
			done := d.progress.Chunks[ch.Index].DownloadedSize
			d.mu.Unlock()
			d.emitProgress(ch.Index, done, "completed")
			return
		}

		// A cancelled context is not a chunk failure; stop retrying and leave
		// the chunk's persisted progress intact for a later resume.
		if ctx.Err() != nil {
			return
		}

		lastErr = err
		d.mu.Lock()
		d.progress.Chunks[ch.Index].Error = err
		if attempt < d.retries {
			d.progress.Chunks[ch.Index].Status = "retrying"
		}
		done := d.progress.Chunks[ch.Index].DownloadedSize
		d.mu.Unlock()

		if attempt < d.retries {
			d.emitProgress(ch.Index, done, "retrying")
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(1<<attempt) * time.Second):
			}
		}
	}

	// If the context was cancelled during the last retry, don't mark the chunk
	// as failed — leave it for flushProgress to persist as "paused" so resume
	// can pick up from where it left off.
	if ctx.Err() != nil {
		return
	}

	d.mu.Lock()
	d.progress.Chunks[ch.Index].Status = "failed"
	d.progress.Chunks[ch.Index].Error = lastErr
	done := d.progress.Chunks[ch.Index].DownloadedSize
	d.mu.Unlock()
	d.emitProgress(ch.Index, done, "failed")
}

func (d *Downloader) downloadChunk(ctx context.Context, ch Chunk) error {
	d.mu.Lock()
	downloaded := d.progress.Chunks[ch.Index].DownloadedSize
	d.mu.Unlock()

	start := ch.Start + downloaded
	if start > ch.End {
		return nil
	}

	client := &http.Client{Timeout: 0}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	rangeHeader := fmt.Sprintf("bytes=%d-%d", start, ch.End)
	req.Header.Set("Range", rangeHeader)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	buf := make([]byte, readBufSize)
	written := start
	chunkDownloaded := downloaded
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			// Throttle jika limiter aktif. Tunggu global dulu lalu per-download;
			// karena keduanya harus lolos, yang paling ketat jadi bottleneck.
			d.mu.RLock()
			lim, glim := d.limiter, d.globalLim
			d.mu.RUnlock()
			if glim != nil {
				if werr := glim.Wait(ctx, n); werr != nil {
					return werr
				}
			}
			if lim != nil {
				if werr := lim.Wait(ctx, n); werr != nil {
					return werr
				}
			}

			if _, werr := d.file.WriteAt(buf[:n], written); werr != nil {
				return fmt.Errorf("write at offset %d: %w", written, werr)
			}
			written += int64(n)
			chunkDownloaded += int64(n)

			d.mu.Lock()
			d.progress.Chunks[ch.Index].DownloadedSize = chunkDownloaded
			d.progress.Chunks[ch.Index].Status = "downloading"
			d.mu.Unlock()
			// Progress is persisted periodically by trackSpeed rather than on
			// every read, to avoid hammering the single DB connection.
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
	}

	return nil
}

func (d *Downloader) trackSpeed(ctx context.Context, lastDownloaded *int64) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.mu.RLock()
			var total int64
			type snap struct {
				index      int
				downloaded int64
				status     string
			}
			snaps := make([]snap, 0, len(d.progress.Chunks))
			for _, cp := range d.progress.Chunks {
				total += cp.DownloadedSize
				if cp.Status == "downloading" {
					snaps = append(snaps, snap{cp.ChunkIndex, cp.DownloadedSize, cp.Status})
				}
			}
			d.mu.RUnlock()

			diff := total - *lastDownloaded
			*lastDownloaded = total

			d.mu.Lock()
			d.speedBuf = append(d.speedBuf, diff)
			if len(d.speedBuf) > 10 {
				d.speedBuf = d.speedBuf[1:]
			}
			d.progress.DownloadedSize = total
			d.mu.Unlock()

			// Persist in-progress chunk sizes outside the lock.
			for _, s := range snaps {
				d.emitProgress(s.index, s.downloaded, s.status)
			}
		}
	}
}

func (d *Downloader) GetProgress() DownloadProgress {
	d.mu.RLock()
	defer d.mu.RUnlock()

	elapsed := time.Since(d.progress.StartTime).Seconds()
	if elapsed < 1 {
		elapsed = 1
	}

	cp := make([]ChunkProgress, len(d.progress.Chunks))
	copy(cp, d.progress.Chunks)

	return DownloadProgress{
		TotalSize:      d.progress.TotalSize,
		DownloadedSize: d.progress.DownloadedSize,
		Chunks:         cp,
		StartTime:      d.progress.StartTime,
	}
}

func (d *Downloader) GetSpeed() float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.speedBuf) == 0 {
		return 0
	}

	var total int64
	for _, s := range d.speedBuf {
		total += s
	}
	return float64(total) / float64(len(d.speedBuf))
}

func (d *Downloader) ActiveWorkers() int32 {
	return atomic.LoadInt32(&d.active)
}

func (d *Downloader) Cancel() {
	d.mu.RLock()
	cancel := d.cancel
	d.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
}

func (d *Downloader) Done() <-chan struct{} {
	return d.done
}

type FileInfo struct {
	Size          int64
	SupportsRange bool
	Filename      string
}

func GetFileInfo(url string) (*FileInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create head request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("head request: %w", err)
	}
	defer resp.Body.Close()

	info := &FileInfo{}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		size, err := strconv.ParseInt(contentLength, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse content-length: %w", err)
		}
		info.Size = size
	}

	acceptRanges := resp.Header.Get("Accept-Ranges")
	contentRange := resp.Header.Get("Content-Range")

	info.SupportsRange = acceptRanges == "bytes" || contentRange != ""
	info.Filename = extractFilename(resp, url)

	return info, nil
}

func extractFilename(resp *http.Response, url string) string {
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		if idx := strings.Index(cd, "filename="); idx != -1 {
			f := cd[idx+9:]
			f = strings.Trim(f, `" `)
			if f != "" {
				return f
			}
		}
	}

	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if idx := strings.Index(name, "?"); idx != -1 {
			name = name[:idx]
		}
		if name != "" {
			return name
		}
	}

	return "download"
}

func VerifyDownload(filePath string, expectedSize int64) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	if info.Size() != expectedSize {
		return fmt.Errorf("size mismatch: expected %d, got %d", expectedSize, info.Size())
	}

	return nil
}

func CleanupFile(filePath string) error {
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove file: %w", err)
	}
	return nil
}

// MoveFile moves src to dst. It first tries an atomic os.Rename; if that fails
// because src and dst live on different filesystems (EXDEV), it falls back to
// a copy followed by removing the source.
func MoveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("rename %s -> %s: %w", src, dst, err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return fmt.Errorf("copy to dest: %w", err)
	}

	if err := out.Close(); err != nil {
		os.Remove(dst)
		return fmt.Errorf("close dest: %w", err)
	}

	if err := os.Remove(src); err != nil {
		return fmt.Errorf("remove source after copy: %w", err)
	}

	return nil
}

func CalculateChunks(fileSize int64, numChunks int) []Chunk {
	if numChunks <= 1 || fileSize <= 0 {
		return []Chunk{{
			Index: 0,
			Start: 0,
			End:   fileSize - 1,
		}}
	}

	if numChunks > int(fileSize) {
		numChunks = int(fileSize)
	}

	chunks := make([]Chunk, 0, numChunks)
	chunkSize := fileSize / int64(numChunks)
	remainder := fileSize % int64(numChunks)

	var start int64
	for i := 0; i < numChunks; i++ {
		size := chunkSize
		if int64(i) < remainder {
			size++
		}
		end := start + size - 1
		if i == numChunks-1 {
			end = fileSize - 1
		}
		chunks = append(chunks, Chunk{
			Index: i,
			Start: start,
			End:   end,
		})
		start = end + 1
	}

	return chunks
}
