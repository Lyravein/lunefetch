package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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
		{0, 4, 1},    // zero size collapses to a single chunk
		{3, 4, 3},    // more chunks than bytes clamps to bytes
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

func TestMoveFileDoesNotReplaceExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	if err := os.WriteFile(src, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := MoveFile(src, dst); err == nil {
		t.Fatal("MoveFile replaced an existing destination")
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "existing" {
		t.Fatalf("destination changed to %q", got)
	}
}

func TestValidateFilename(t *testing.T) {
	valid := []string{"archive.tar.gz", "normal file.pdf", "日本語.zip"}
	for _, name := range valid {
		if _, err := ValidateFilename(name); err != nil {
			t.Errorf("ValidateFilename(%q): %v", name, err)
		}
	}
	invalid := []string{"", ".", "..", "../escape", `dir\\escape`, "bad\x00name", "CON.txt", "trailing."}
	for _, name := range invalid {
		if _, err := ValidateFilename(name); err == nil {
			t.Errorf("ValidateFilename(%q) succeeded", name)
		}
	}
}

func TestSafeDownloadPathRejectsTraversal(t *testing.T) {
	if _, err := SafeDownloadPath(t.TempDir(), "../../escape"); err == nil {
		t.Fatal("SafeDownloadPath accepted traversal")
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
	d.client = httptestClient()

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

func TestDownloadRejectsServerIgnoringRanges(t *testing.T) {
	payload := makePayload(1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	d := NewDownloader(srv.URL, dst, int64(len(payload)), CalculateChunks(int64(len(payload)), 2), 2, 0)
	d.client = httptestClient()
	if err := d.Start(context.Background()); err == nil {
		t.Fatal("download succeeded when server ignored multi-chunk ranges")
	}
}

func TestDownloadRejectsMismatchedContentRange(t *testing.T) {
	payload := makePayload(1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 1-%d/%d", len(payload)-1, len(payload)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(payload[1:])
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	d := NewDownloader(srv.URL, dst, int64(len(payload)), CalculateChunks(int64(len(payload)), 1), 1, 0)
	d.client = httptestClient()
	if err := d.Start(context.Background()); err == nil {
		t.Fatal("download succeeded with mismatched Content-Range")
	}
}

func TestDownloadRejectsShortRange(t *testing.T) {
	payload := makePayload(1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(payload)-1, len(payload)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(payload[:len(payload)/2])
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	d := NewDownloader(srv.URL, dst, int64(len(payload)), CalculateChunks(int64(len(payload)), 1), 1, 0)
	d.client = httptestClient()
	if err := d.Start(context.Background()); err == nil {
		t.Fatal("download succeeded with a short range body")
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
	d.client = httptestClient()
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

func TestResumeSendsStrongETagWithIfRange(t *testing.T) {
	payload := makePayload(1024)
	const etag = `"version-1"`
	var gotIfRange string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIfRange = r.Header.Get("If-Range")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 512-1023/%d", len(payload)))
		w.Header().Set("Content-Length", "512")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(payload[512:])
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	if err := os.WriteFile(dst, payload[:512], 0o644); err != nil {
		t.Fatal(err)
	}
	d := NewDownloader(srv.URL, dst, int64(len(payload)), []Chunk{{Index: 0, Start: 0, End: 1023, Downloaded: 512}}, 1, 0)
	d.SetHTTPClient(httptestClient())
	d.SetValidators(etag, "Wed, 21 Oct 2015 07:28:00 GMT")
	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if gotIfRange != etag {
		t.Fatalf("If-Range = %q, want %q", gotIfRange, etag)
	}
}

func TestResumeUsesLastModifiedForWeakETag(t *testing.T) {
	payload := makePayload(1024)
	const modified = "Wed, 21 Oct 2015 07:28:00 GMT"
	var gotIfRange string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIfRange = r.Header.Get("If-Range")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 512-1023/%d", len(payload)))
		w.Header().Set("Content-Length", "512")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(payload[512:])
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	if err := os.WriteFile(dst, payload[:512], 0o644); err != nil {
		t.Fatal(err)
	}
	d := NewDownloader(srv.URL, dst, int64(len(payload)), []Chunk{{Index: 0, Start: 0, End: 1023, Downloaded: 512}}, 1, 0)
	d.SetHTTPClient(httptestClient())
	d.SetValidators(`W/"version-1"`, modified)
	if err := d.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if gotIfRange != modified {
		t.Fatalf("If-Range = %q, want Last-Modified %q", gotIfRange, modified)
	}
}

func TestResumeRejectsChangedRepresentation(t *testing.T) {
	payload := makePayload(1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Range") == "" {
			t.Error("resumed request omitted If-Range")
		}
		// Per RFC 7233, a failed If-Range comparison returns the full 200 body.
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	original := bytes.Repeat([]byte{0x7f}, 512)
	if err := os.WriteFile(dst, original, 0o644); err != nil {
		t.Fatal(err)
	}
	d := NewDownloader(srv.URL, dst, int64(len(payload)), []Chunk{{Index: 0, Start: 0, End: 1023, Downloaded: 512}}, 1, 0)
	d.SetHTTPClient(httptestClient())
	d.SetValidators(`"old-version"`, "")
	if err := d.Start(context.Background()); err == nil {
		t.Fatal("resume succeeded after the remote representation changed")
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got[:512], original) {
		t.Fatal("persisted prefix was overwritten after validator mismatch")
	}
}

func TestMoveFileReportsDestinationExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := MoveFile(src, dst)
	if !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("MoveFile error = %v, want ErrDestinationExists", err)
	}
	// The source must survive so the user can still choose Keep Both/Replace.
	if _, statErr := os.Stat(src); statErr != nil {
		t.Errorf("source was lost on conflict: %v", statErr)
	}
	if got, _ := os.ReadFile(dst); string(got) != "old" {
		t.Errorf("destination was modified: %q", got)
	}
}

func TestUniqueDownloadPathSuffixes(t *testing.T) {
	dir := t.TempDir()
	free := filepath.Join(dir, "file.tar.gz")
	got, err := UniqueDownloadPath(free)
	if err != nil {
		t.Fatal(err)
	}
	if got != free {
		t.Errorf("free path changed: got %q want %q", got, free)
	}

	if err := os.WriteFile(free, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = UniqueDownloadPath(free)
	if err != nil {
		t.Fatal(err)
	}
	// Only the final extension is preserved, matching common file managers.
	if want := filepath.Join(dir, "file.tar (1).gz"); got != want {
		t.Errorf("got %q want %q", got, want)
	}

	if err := os.WriteFile(got, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = UniqueDownloadPath(free)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "file.tar (2).gz"); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestUniqueDownloadPathNoExtension(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "README")
	if err := os.WriteFile(p, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := UniqueDownloadPath(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "README (1)"); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestReplaceFileOverwrites(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceFile(src, dst); err != nil {
		t.Fatalf("ReplaceFile: %v", err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "new" {
		t.Errorf("destination content = %q, want %q", got, "new")
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Error("source still present after replace")
	}
}

func httptestClient() *http.Client {
	return &http.Client{Transport: http.DefaultTransport}
}

func TestValidateHTTPURLRejectsBlockedDestinations(t *testing.T) {
	strict := addressPolicy{}
	for _, host := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "[::1]", "192.168.1.10", "0.0.0.0"} {
		u := mustURL(t, "http://"+host+"/")
		if err := validateHTTPURL(context.Background(), u, strict); err == nil {
			t.Errorf("validateHTTPURL accepted blocked host %q", host)
		}
	}
}

// With allow_local_hosts on, a NAS or localhost server must be reachable.
func TestAllowLocalHostsPermitsPrivateDestinations(t *testing.T) {
	relaxed := addressPolicy{allowLocal: true}
	for _, host := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.10", "[::1]"} {
		u := mustURL(t, "http://"+host+"/")
		if err := validateHTTPURL(context.Background(), u, relaxed); err != nil {
			t.Errorf("allowLocal rejected %q: %v", host, err)
		}
	}
}

// Cloud metadata stays blocked even when local destinations are allowed,
// because reaching it leaks instance credentials.
func TestMetadataBlockedEvenWhenLocalAllowed(t *testing.T) {
	relaxed := addressPolicy{allowLocal: true}
	for _, host := range []string{"169.254.169.254", "[fd00:ec2::254]"} {
		u := mustURL(t, "http://"+host+"/latest/meta-data/")
		if err := validateHTTPURL(context.Background(), u, relaxed); err == nil {
			t.Errorf("metadata endpoint %q was allowed", host)
		}
	}
}

// IPv4-mapped IPv6 must not bypass the policy.
func TestMappedIPv4IsNotABypass(t *testing.T) {
	strict := addressPolicy{}
	if !strict.blocked(netip.MustParseAddr("::ffff:127.0.0.1")) {
		t.Error("mapped loopback ::ffff:127.0.0.1 was not blocked")
	}
	if !strict.blocked(netip.MustParseAddr("::ffff:169.254.169.254")) {
		t.Error("mapped metadata address was not blocked")
	}
}

func TestDialContextRejectsBlockedIP(t *testing.T) {
	strict := addressPolicy{}
	addr := net.JoinHostPort(netip.MustParseAddr("127.0.0.1").String(), "80")
	if _, err := strict.dialContext(context.Background(), "tcp", addr); err == nil {
		t.Fatal("dialContext accepted loopback destination")
	}
}

// A loopback proxy (Tor, mitmproxy) is user-configured, so it must stay
// dialable even under the strict policy.
func TestProxyHostExemptFromPolicy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newHTTPClient(srv.URL, 5*time.Second, false)
	resp, err := client.Get("http://example.com/file.bin")
	if err != nil {
		t.Fatalf("request through loopback proxy failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("proxy returned %s", resp.Status)
	}
}

// The proxy exemption must not become a general hole: a blocked *target*
// is still rejected even while a proxy is configured.
func TestProxyDoesNotExemptBlockedTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newHTTPClient(srv.URL, 5*time.Second, false)
	resp, err := client.Get("http://169.254.169.254/latest/meta-data/")
	if err == nil {
		resp.Body.Close()
		t.Fatal("metadata target was allowed through a proxy")
	}
}

func TestHTTPClientRejectsBlockedRedirect(t *testing.T) {
	client := newHTTPClient("", 0, false)
	redirect := &http.Request{URL: mustURL(t, "http://127.0.0.1/")}
	if err := client.CheckRedirect(redirect, nil); err == nil {
		t.Fatal("redirect policy accepted loopback destination")
	}
}

func TestHTTPClientLimitsRedirects(t *testing.T) {
	client := newHTTPClient("", 0, false)
	redirect := &http.Request{URL: mustURL(t, "https://example.com/")}
	via := make([]*http.Request, 10)
	if err := client.CheckRedirect(redirect, via); err == nil {
		t.Fatal("redirect policy accepted more than 10 redirects")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
