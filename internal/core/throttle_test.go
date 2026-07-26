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

func TestLimiterThrottles(t *testing.T) {
	// 64 KiB/s: burst di-clamp ke minimal 32 KiB, jadi 128 KiB butuh
	// setidaknya ~1.5s setelah burst awal terpakai.
	lim := NewLimiter(64 * 1024)

	// Habiskan burst dulu supaya pengukuran tidak terdistorsi.
	if err := lim.Wait(context.Background(), 128*1024); err != nil {
		t.Fatalf("priming wait: %v", err)
	}

	start := time.Now()
	if err := lim.Wait(context.Background(), 64*1024); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 500*time.Millisecond {
		t.Errorf("64 KiB at 64 KiB/s took %v, expected to be throttled", elapsed)
	}
}

func TestLimiterWaitLargerThanBurst(t *testing.T) {
	// n lebih besar dari burst harus dipecah, bukan error.
	lim := NewLimiter(1 << 20) // burst 2 MiB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := lim.Wait(ctx, 5<<20); err != nil {
		t.Fatalf("wait larger than burst failed: %v", err)
	}
}

func TestLimiterRespectsContextCancel(t *testing.T) {
	lim := NewLimiter(1024) // sangat lambat
	// Habiskan burst.
	lim.Wait(context.Background(), 32*1024)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := lim.Wait(ctx, 32*1024); err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}

func TestLimiterSetRateToUnlimited(t *testing.T) {
	lim := NewLimiter(1024)
	lim.Wait(context.Background(), 32*1024) // habiskan burst

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
	lim.SetRate(64 * 1024)

	// Habiskan burst.
	if err := lim.Wait(context.Background(), 128*1024); err != nil {
		t.Fatalf("priming wait: %v", err)
	}

	start := time.Now()
	if err := lim.Wait(context.Background(), 64*1024); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 500*time.Millisecond {
		t.Errorf("after SetRate(64k) wait took %v, expected throttling", elapsed)
	}
}
