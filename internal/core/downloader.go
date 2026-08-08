package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode"
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
	proxyURL   string   // "" = no proxy
	allowLocal bool     // true = permit LAN/loopback destinations
	etag       string
	modified   string
	client     *http.Client
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
		client:   newHTTPClient("", 0, false),
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

// SetProxy mengatur proxy URL untuk downloader ini.
// Format: "http://host:port", "socks5://host:port", atau "" untuk no proxy.
func (d *Downloader) SetProxy(proxyURL string) {
	d.mu.Lock()
	d.proxyURL = proxyURL
	d.client = newHTTPClient(proxyURL, 0, d.allowLocal)
	d.mu.Unlock()
}

// SetAllowLocalHosts controls whether LAN, loopback, and link-local
// destinations are reachable. Cloud metadata addresses stay blocked either way.
// Safe to call in any order relative to SetProxy.
func (d *Downloader) SetAllowLocalHosts(allow bool) {
	d.mu.Lock()
	d.allowLocal = allow
	d.client = newHTTPClient(d.proxyURL, 0, allow)
	d.mu.Unlock()
}

// SetHTTPClient replaces the HTTP client, primarily for callers that provide
// a custom transport. The default client enforces the destination policy.
func (d *Downloader) SetHTTPClient(client *http.Client) {
	if client == nil {
		return
	}
	d.mu.Lock()
	d.client = client
	d.mu.Unlock()
}

// SetValidators binds resumed range requests to the remote representation
// observed when the download was created.
func (d *Downloader) SetValidators(etag, lastModified string) {
	d.mu.Lock()
	d.etag = strings.TrimSpace(etag)
	d.modified = strings.TrimSpace(lastModified)
	d.mu.Unlock()
}

// metadataAddrs are cloud instance-metadata endpoints. Reaching them leaks
// credentials, so they stay blocked even when local destinations are allowed.
var metadataAddrs = []netip.Addr{
	netip.MustParseAddr("169.254.169.254"),
	netip.MustParseAddr("fd00:ec2::254"),
}

// addressPolicy decides which resolved addresses a download may reach.
//
// proxyHost is exempt from the policy: it is configured by the user, not by
// the remote URL, and loopback proxies (Tor, mitmproxy) are a normal setup.
// With a proxy in play the target host is still screened by safeRoundTripper.
type addressPolicy struct {
	allowLocal bool
	proxyHost  string
}

func (p addressPolicy) blocked(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, meta := range metadataAddrs {
		if addr == meta {
			return true
		}
	}
	if addr.IsUnspecified() || addr.IsMulticast() {
		return true
	}
	if p.allowLocal {
		return false
	}
	return addr.IsLoopback() || addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast()
}

func (p addressPolicy) exempt(host string) bool {
	return p.proxyHost != "" && strings.EqualFold(host, p.proxyHost)
}

// newHTTPClient membuat http.Client dengan proxy opsional dan kebijakan tujuan.
func newHTTPClient(proxyURL string, timeout time.Duration, allowLocal bool) *http.Client {
	policy := addressPolicy{allowLocal: allowLocal}

	transport := &http.Transport{
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
			policy.proxyHost = u.Hostname()
		}
	}
	transport.DialContext = policy.dialContext

	client := &http.Client{
		Timeout:   timeout,
		Transport: safeRoundTripper{transport: transport, policy: policy},
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return validateHTTPURL(req.Context(), req.URL, policy)
	}
	return client
}

type safeRoundTripper struct {
	transport http.RoundTripper
	policy    addressPolicy
}

func (t safeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := validateHTTPURL(req.Context(), req.URL, t.policy); err != nil {
		return nil, err
	}
	return t.transport.RoundTrip(req)
}

// dialContext resolves the host itself and connects only to addresses it has
// already screened, so a name cannot resolve to a public address during the
// check and a private one at connect time (DNS rebinding).
func (p addressPolicy) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid network address: %w", err)
	}
	dialer := &net.Dialer{}
	if p.exempt(host) {
		return dialer.DialContext(ctx, network, address)
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", host, err)
	}
	var allowed []netip.Addr
	for _, ip := range ips {
		if p.blocked(ip) {
			return nil, fmt.Errorf("blocked network destination %q", ip)
		}
		allowed = append(allowed, ip)
	}
	if len(allowed) == 0 {
		return nil, fmt.Errorf("resolve %q: no addresses", host)
	}

	var lastErr error
	for _, ip := range allowed {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("connect to %q: %w", host, lastErr)
}

func validateHTTPURL(ctx context.Context, u *url.URL, policy addressPolicy) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("invalid HTTP URL")
	}
	host := u.Hostname()
	if ip, err := netip.ParseAddr(host); err == nil {
		if policy.blocked(ip) {
			return fmt.Errorf("blocked network destination %q", host)
		}
		return nil
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("resolve %q: no addresses", host)
	}
	for _, ip := range ips {
		if policy.blocked(ip) {
			return fmt.Errorf("blocked network destination %q", host)
		}
	}
	return nil
}

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

	d.mu.RLock()
	client := d.client
	validator := d.etag
	if validator == "" || strings.HasPrefix(validator, "W/") {
		validator = d.modified
	}
	d.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, ch.End))
	if downloaded > 0 && validator != "" {
		req.Header.Set("If-Range", validator)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	expected := ch.End - start + 1
	if resp.StatusCode == http.StatusPartialContent {
		gotStart, gotEnd, gotTotal, err := parseContentRange(resp.Header.Get("Content-Range"))
		if err != nil {
			return err
		}
		if gotStart != start || gotEnd != ch.End || gotTotal != d.totalSize {
			return fmt.Errorf("content-range mismatch: got bytes %d-%d/%d, want %d-%d/%d", gotStart, gotEnd, gotTotal, start, ch.End, d.totalSize)
		}
	} else if resp.StatusCode == http.StatusOK {
		if start != 0 || ch.Start != 0 || ch.End != d.totalSize-1 || len(d.chunks) != 1 {
			return fmt.Errorf("server ignored range request for partial download")
		}
	} else {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	if resp.ContentLength >= 0 && resp.ContentLength != expected {
		return fmt.Errorf("content length mismatch: got %d, want %d", resp.ContentLength, expected)
	}

	buf := make([]byte, readBufSize)
	written := start
	chunkDownloaded := downloaded
	remaining := expected
	for remaining > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		readBuf := buf
		if int64(len(readBuf)) > remaining {
			readBuf = readBuf[:remaining]
		}
		n, err := resp.Body.Read(readBuf)
		if n > 0 {
			// Throttle jika limiter aktif. Tunggu global dulu lalu per-download;
			// karena keduanya harus lolos, yang paling ketat jadi bottleneck.
			d.mu.RLock()
			lim, glim := d.limiter, d.globalLim
			d.mu.RUnlock()
			if glim != nil {
				if werr := glim.Wait(ctx, int(n)); werr != nil {
					return werr
				}
			}
			if lim != nil {
				if werr := lim.Wait(ctx, int(n)); werr != nil {
					return werr
				}
			}

			if _, werr := d.file.WriteAt(buf[:n], written); werr != nil {
				return fmt.Errorf("write at offset %d: %w", written, werr)
			}
			written += int64(n)
			chunkDownloaded += int64(n)
			remaining -= int64(n)

			d.mu.Lock()
			d.progress.Chunks[ch.Index].DownloadedSize = chunkDownloaded
			d.progress.Chunks[ch.Index].Status = "downloading"
			d.mu.Unlock()
			// Progress is persisted periodically by trackSpeed rather than on
			// every read, to avoid hammering the single DB connection.
		}
		if err == io.EOF {
			if remaining != 0 {
				return fmt.Errorf("short response body: missing %d bytes", remaining)
			}
			break
		}
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
	}
	var extra [1]byte
	if n, err := resp.Body.Read(extra[:]); n != 0 || (err != nil && err != io.EOF) {
		if err != nil && err != io.EOF {
			return fmt.Errorf("check response boundary: %w", err)
		}
		return fmt.Errorf("response body exceeds requested range")
	}

	return nil
}

func parseContentRange(value string) (start, end, total int64, err error) {
	if !strings.HasPrefix(value, "bytes ") {
		return 0, 0, 0, fmt.Errorf("missing or invalid content-range")
	}
	parts := strings.Split(strings.TrimPrefix(value, "bytes "), "/")
	if len(parts) != 2 || parts[1] == "*" {
		return 0, 0, 0, fmt.Errorf("invalid content-range %q", value)
	}
	bounds := strings.Split(parts[0], "-")
	if len(bounds) != 2 {
		return 0, 0, 0, fmt.Errorf("invalid content-range %q", value)
	}
	start, err = strconv.ParseInt(bounds[0], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid content-range %q", value)
	}
	end, err = strconv.ParseInt(bounds[1], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid content-range %q", value)
	}
	total, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil || start < 0 || end < start || total <= end {
		return 0, 0, 0, fmt.Errorf("invalid content-range %q", value)
	}
	return start, end, total, nil
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
	ETag          string
	LastModified  string
}

func GetFileInfo(rawURL, proxyURL string, allowLocal bool) (*FileInfo, error) {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid HTTP URL")
	}
	client := newHTTPClient(proxyURL, 15*time.Second, allowLocal)
	policy := addressPolicy{allowLocal: allowLocal}
	if proxyURL != "" {
		if pu, perr := url.Parse(proxyURL); perr == nil {
			policy.proxyHost = pu.Hostname()
		}
	}
	if err := validateHTTPURL(context.Background(), u, policy); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodHead, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create head request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("head request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("head request returned %s", resp.Status)
	}

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
	info.ETag = strings.TrimSpace(resp.Header.Get("ETag"))
	info.LastModified = strings.TrimSpace(resp.Header.Get("Last-Modified"))
	info.Filename, err = extractFilename(resp, rawURL)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func extractFilename(resp *http.Response, rawURL string) (string, error) {
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		_, params, err := mime.ParseMediaType(cd)
		if err != nil {
			return "", fmt.Errorf("parse content-disposition: %w", err)
		}
		if f := params["filename"]; f != "" {
			return ValidateFilename(f)
		}
	}

	u, err := url.Parse(rawURL)
	if err == nil {
		name, unescapeErr := url.PathUnescape(path.Base(u.EscapedPath()))
		if unescapeErr == nil && name != "" && name != "." && name != "/" {
			return ValidateFilename(name)
		}
	}

	return "download", nil
}

// ValidateFilename accepts exactly one portable filesystem path component.
func ValidateFilename(name string) (string, error) {
	if name == "" || name == "." || name == ".." || len([]byte(name)) > 255 {
		return "", fmt.Errorf("invalid filename")
	}
	if strings.ContainsAny(name, `/\\`) || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return "", fmt.Errorf("filename must be a single path component")
	}
	for _, r := range name {
		if r == 0 || unicode.IsControl(r) || strings.ContainsRune(`<>:"|?*`, r) {
			return "", fmt.Errorf("filename contains invalid characters")
		}
	}
	base := strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name)))
	reserved := map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true}
	if reserved[base] || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
		return "", fmt.Errorf("filename is reserved by the operating system")
	}
	return name, nil
}

// SafeDownloadPath verifies that filename cannot escape root.
func SafeDownloadPath(root, filename string) (string, error) {
	name, err := ValidateFilename(filename)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve download directory: %w", err)
	}
	dst := filepath.Join(absRoot, name)
	rel, err := filepath.Rel(absRoot, dst)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("download path escapes destination directory")
	}
	return dst, nil
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

// ErrDestinationExists reports that the destination file is already present.
// Callers can offer the user Keep Both / Replace / Cancel instead of failing.
var ErrDestinationExists = errors.New("destination already exists")

// UniqueDownloadPath returns dst when it is free, otherwise the first available
// "name (n).ext" variant. It is advisory: the final create still uses O_EXCL,
// so a race loses safely rather than overwriting.
func UniqueDownloadPath(dst string) (string, error) {
	if _, err := os.Lstat(dst); errors.Is(err, os.ErrNotExist) {
		return dst, nil
	} else if err != nil {
		return "", fmt.Errorf("stat destination: %w", err)
	}

	dir := filepath.Dir(dst)
	base := filepath.Base(dst)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	for i := 1; i < 10000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("stat destination: %w", err)
		}
	}
	return "", fmt.Errorf("no available filename for %s", dst)
}

// ReplaceFile moves src onto dst, overwriting an existing destination.
// Only call this after the user has explicitly chosen to replace.
func ReplaceFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("replace %s: %w", dst, err)
	}
	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove existing destination: %w", err)
	}
	return MoveFile(src, dst)
}

// MoveFile moves src to dst without replacing an existing destination.
// It returns ErrDestinationExists when dst is already taken.
func MoveFile(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		if err := os.Remove(src); err != nil {
			return fmt.Errorf("remove source after link: %w", err)
		}
		return nil
	} else if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%w: %s", ErrDestinationExists, dst)
	} else if !errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("link %s -> %s: %w", src, dst, err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%w: %s", ErrDestinationExists, dst)
		}
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
