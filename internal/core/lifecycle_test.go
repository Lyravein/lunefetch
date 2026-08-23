package core

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// A .part file that does not hold the bytes its chunk bookkeeping claims must
// never be reported as a successful download: the caller publishes it verbatim.
func TestStartRejectsSparseFileClaimedComplete(t *testing.T) {
	dir := t.TempDir()
	part := filepath.Join(dir, "claimed.part")
	const total = 4096

	chunks := []Chunk{{Index: 0, Start: 0, End: total - 1, Downloaded: total}}
	d := NewDownloader("http://127.0.0.1:1/never", part, total, chunks, 1, 0)

	err := d.Start(context.Background())
	if err == nil {
		t.Fatal("Start reported success for a file that was never downloaded")
	}
}

// A short .part file must also be rejected rather than published.
func TestStartRejectsTruncatedFile(t *testing.T) {
	dir := t.TempDir()
	part := filepath.Join(dir, "short.part")
	if err := os.WriteFile(part, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	const total = 1024
	chunks := []Chunk{{Index: 0, Start: 0, End: total - 1, Downloaded: total}}
	d := NewDownloader("http://127.0.0.1:1/never", part, total, chunks, 1, 0)

	if err := d.Start(context.Background()); err == nil {
		t.Fatal("Start reported success for a truncated file")
	}
}

// Done must be closed on every Start return path, including early failures,
// otherwise callers waiting on it block forever.
func TestDoneIsClosedOnEarlyFailure(t *testing.T) {
	dir := t.TempDir()
	missingParent := filepath.Join(dir, "no-such-dir", "f.part")
	chunks := []Chunk{{Index: 0, Start: 0, End: 9}}
	d := NewDownloader("http://127.0.0.1:1/never", missingParent, 10, chunks, 1, 0)

	if err := d.Start(context.Background()); err == nil {
		t.Fatal("expected Start to fail when the parent directory is missing")
	}
	select {
	case <-d.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Done was never closed after Start failed early")
	}
}

// Cancelling while chunks are still waiting for a slot must still wait for
// running workers, persist their progress, and close Done. Previously this path
// returned immediately, losing progress and leaving a worker writing to a file
// the deferred Close had already closed.
func TestCancelDuringChunkSpawnFlushesProgressAndClosesDone(t *testing.T) {
	dir := t.TempDir()
	part := filepath.Join(dir, "cancel.part")

	// More chunks than concurrent slots, so the spawn loop blocks on the
	// semaphore and observes the cancellation.
	const chunkSize = 64
	chunks := make([]Chunk, 8)
	for i := range chunks {
		chunks[i] = Chunk{Index: i, Start: int64(i) * chunkSize, End: int64(i+1)*chunkSize - 1}
	}
	total := int64(len(chunks)) * chunkSize

	d := NewDownloader("http://127.0.0.1:1/never", part, total, chunks, 1, 0)
	flushed := make(chan struct{}, len(chunks)*4)
	d.SetProgressCallback(func(int, int64, string) {
		select {
		case flushed <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := d.Start(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Start error = %v, want context.Canceled", err)
	}
	select {
	case <-d.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Done was never closed after cancellation")
	}
	if got := d.ActiveWorkers(); got != 0 {
		t.Fatalf("active workers after cancel = %d, want 0", got)
	}
	if len(flushed) == 0 {
		t.Fatal("cancellation did not flush chunk progress")
	}
}

// Done must be safe to read while Start is writing it.
func TestDoneIsRaceFreeWithStart(t *testing.T) {
	dir := t.TempDir()
	part := filepath.Join(dir, "race.part")
	chunks := []Chunk{{Index: 0, Start: 0, End: 63, Downloaded: 64}}
	d := NewDownloader("http://127.0.0.1:1/never", part, 64, chunks, 1, 0)

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = d.Done()
			}
		}
	}()
	_ = d.Start(context.Background())
	close(stop)
}

func TestBackoffHonoursConfiguredBaseAndCap(t *testing.T) {
	d := NewDownloader("http://example.invalid/f", "f", 1, []Chunk{{Index: 0, End: 0}}, 1, 3)

	if got := d.backoffFor(0); got != time.Second {
		t.Fatalf("default first backoff = %v, want 1s", got)
	}

	d.SetRetryBackoff(2 * time.Second)
	if got := d.backoffFor(0); got != 2*time.Second {
		t.Fatalf("configured first backoff = %v, want 2s", got)
	}
	if got := d.backoffFor(2); got != 8*time.Second {
		t.Fatalf("third backoff = %v, want 8s", got)
	}
	if got := d.backoffFor(19); got != maxRetryBackoff {
		t.Fatalf("late backoff = %v, want the %v cap", got, maxRetryBackoff)
	}
	d.SetRetryBackoff(0)
	if got := d.backoffFor(0); got != 2*time.Second {
		t.Fatalf("non-positive backoff overwrote the configured value: %v", got)
	}
}

func TestAddressPolicyBlocksReservedRanges(t *testing.T) {
	policy := addressPolicy{}
	blocked := []string{
		"100.64.0.1",      // CGNAT / Tailscale tailnet
		"100.102.238.113", // a real tailnet peer address shape
		"192.0.0.1",
		"198.18.0.1",
		"198.51.100.7",
		"203.0.113.9",
		"240.0.0.1",
		"2001:db8::1",
		"::ffff:100.64.0.1", // mapped CGNAT must not bypass the check
	}
	for _, raw := range blocked {
		if !policy.blocked(mustAddr(t, raw)) {
			t.Errorf("%s was allowed, want blocked", raw)
		}
	}

	allowed := []string{"1.1.1.1", "8.8.8.8", "93.184.216.34", "2606:4700::1111"}
	for _, raw := range allowed {
		if policy.blocked(mustAddr(t, raw)) {
			t.Errorf("%s was blocked, want allowed", raw)
		}
	}

	local := addressPolicy{allowLocal: true}
	if !local.blocked(mustAddr(t, "169.254.169.254")) {
		t.Error("cloud metadata must stay blocked even with local access allowed")
	}
	if local.blocked(mustAddr(t, "100.64.0.1")) {
		t.Error("CGNAT should be reachable when local destinations are allowed")
	}
}

func TestSafeSaveDirConfinesDestinationToRoot(t *testing.T) {
	root := t.TempDir()

	got, err := SafeSaveDir(root, "")
	if err != nil || got != root {
		t.Fatalf("empty dir = (%q, %v), want the root", got, err)
	}

	nested := filepath.Join(root, "Media", "clips")
	if got, err := SafeSaveDir(root, nested); err != nil || got != nested {
		t.Fatalf("nested dir = (%q, %v), want it accepted", got, err)
	}

	escapes := []string{
		filepath.Join(root, ".."),
		filepath.Join(root, "..", "elsewhere"),
		filepath.Dir(root),
	}
	if runtime.GOOS != "windows" {
		escapes = append(escapes, "/etc", filepath.Join(os.Getenv("HOME"), ".config", "autostart"))
	}
	for _, dir := range escapes {
		if _, err := SafeSaveDir(root, dir); err == nil {
			t.Errorf("SafeSaveDir(%q) was accepted, want rejection", dir)
		}
	}

	if _, err := SafeSaveDir(root, filepath.Join(root, "with\x00null")); err == nil {
		t.Error("a NUL byte in the destination was accepted")
	}
}

func mustAddr(t *testing.T, raw string) netip.Addr {
	t.Helper()
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return addr
}

// numChunks was compared against int(fileSize), which truncates on 32-bit
// builds and could produce numChunks = 0 and a divide by zero.
func TestCalculateChunksHandlesLargeAndTinySizes(t *testing.T) {
	const fourGiB = int64(4) << 30
	chunks := CalculateChunks(fourGiB, 8)
	if len(chunks) != 8 {
		t.Fatalf("chunk count = %d, want 8", len(chunks))
	}
	if chunks[0].Start != 0 {
		t.Fatalf("first chunk starts at %d, want 0", chunks[0].Start)
	}
	if last := chunks[len(chunks)-1]; last.End != fourGiB-1 {
		t.Fatalf("last chunk ends at %d, want %d", last.End, fourGiB-1)
	}

	var covered int64
	for i, c := range chunks {
		if c.End < c.Start {
			t.Fatalf("chunk %d is inverted: %d-%d", i, c.Start, c.End)
		}
		covered += c.End - c.Start + 1
	}
	if covered != fourGiB {
		t.Fatalf("chunks cover %d bytes, want %d", covered, fourGiB)
	}

	// More chunks than bytes must collapse to one chunk per byte, never zero.
	if got := CalculateChunks(3, 16); len(got) != 3 {
		t.Fatalf("chunk count for a 3-byte file = %d, want 3", len(got))
	}
	if got := CalculateChunks(1, 8); len(got) != 1 {
		t.Fatalf("chunk count for a 1-byte file = %d, want 1", len(got))
	}
}
