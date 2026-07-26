package core

import (
	"context"
	"testing"
	"time"
)

func TestLimiterUnlimitedIsInstant(t *testing.T) {
	lim := NewLimiter(0)
	start := time.Now()
	for i := 0; i < 100; i++ {
		if err := lim.Wait(context.Background(), 1<<20); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("unlimited limiter blocked for %v", elapsed)
	}
}

// TestLimiterBurstIsSmall menjaga invariant utama: burst tidak boleh besar,
// kalau tidak download pendek lolos tanpa throttle sama sekali.
func TestLimiterBurstIsSmall(t *testing.T) {
	for _, rate := range []int64{1024, 64 * 1024, 1 << 20, 100 << 20} {
		if got := burstFor(rate); got > readBufSize {
			t.Errorf("burstFor(%d) = %d, must not exceed read buffer %d",
				rate, got, readBufSize)
		}
	}
	// Rate di bawah buffer harus dibatasi ke rate itu sendiri.
	if got := burstFor(1024); got != 1024 {
		t.Errorf("burstFor(1024) = %d, want 1024", got)
	}
}

func TestLimiterThrottles(t *testing.T) {
	// 128 KiB/s, minta 128 KiB. Burst hanya 32 KiB, jadi sisa 96 KiB
	// harus menunggu ~0.75s.
	lim := NewLimiter(128 * 1024)

	start := time.Now()
	if err := lim.Wait(context.Background(), 128*1024); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 500*time.Millisecond {
		t.Errorf("128 KiB at 128 KiB/s took %v, expected throttling", elapsed)
	}
}

func TestLimiterWaitLargerThanBurst(t *testing.T) {
	// n jauh lebih besar dari burst harus dipecah, bukan error.
	lim := NewLimiter(1 << 20) // burst 32 KiB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := lim.Wait(ctx, 1<<20); err != nil {
		t.Fatalf("wait larger than burst failed: %v", err)
	}
}

func TestLimiterRespectsContextCancel(t *testing.T) {
	lim := NewLimiter(1024)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := lim.Wait(ctx, 8*1024); err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}

func TestLimiterSetRateToUnlimited(t *testing.T) {
	lim := NewLimiter(64 * 1024)
	lim.SetRate(0) // jadi unlimited

	start := time.Now()
	if err := lim.Wait(context.Background(), 10<<20); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("after SetRate(0) limiter still blocked for %v", elapsed)
	}
}

func TestLimiterSetRateTightens(t *testing.T) {
	lim := NewLimiter(0) // unlimited
	lim.SetRate(128 * 1024)

	start := time.Now()
	if err := lim.Wait(context.Background(), 128*1024); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 500*time.Millisecond {
		t.Errorf("after SetRate(128k) wait took %v, expected throttling", elapsed)
	}
}

// TestLimiterSustainedRate memverifikasi throughput aktual mendekati rate yang
// diminta, bukan hanya "lebih lambat dari instan".
func TestLimiterSustainedRate(t *testing.T) {
	const rate = 256 * 1024
	const total = 512 * 1024

	lim := NewLimiter(rate)
	start := time.Now()
	sent := 0
	for sent < total {
		n := readBufSize
		if total-sent < n {
			n = total - sent
		}
		if err := lim.Wait(context.Background(), n); err != nil {
			t.Fatalf("wait: %v", err)
		}
		sent += n
	}
	elapsed := time.Since(start)

	actual := float64(total) / elapsed.Seconds()
	// Toleransi 35% di atas rate — burst awal mempercepat sedikit.
	if actual > rate*1.35 {
		t.Errorf("sustained rate %.0f B/s exceeds limit %d B/s by too much",
			actual, rate)
	}
	t.Logf("sustained %.0f B/s against limit %d B/s (%v)", actual, rate, elapsed)
}
