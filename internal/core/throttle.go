package core

import (
	"context"
	"math"

	"golang.org/x/time/rate"
)

// Limiter wraps rate.Limiter dan support dynamic update via SetRate.
// Jika limit == 0, throttle dinonaktifkan.
type Limiter struct {
	l *rate.Limiter
}

// NewLimiter membuat limiter baru. bytesPerSec == 0 berarti unlimited.
func NewLimiter(bytesPerSec int64) *Limiter {
	return &Limiter{l: makeLimiter(bytesPerSec)}
}

// SetRate mengubah rate limit secara dinamis. Thread-safe.
// bytesPerSec == 0 berarti unlimited.
func (lim *Limiter) SetRate(bytesPerSec int64) {
	if bytesPerSec <= 0 {
		lim.l.SetLimit(rate.Inf)
		lim.l.SetBurst(math.MaxInt32)
	} else {
		lim.l.SetLimit(rate.Limit(bytesPerSec))
		lim.l.SetBurst(burstFor(bytesPerSec))
	}
}

// Wait memblokir sampai limiter mengizinkan n bytes lewat.
// Jika ctx dibatalkan, langsung return ctx.Err().
func (lim *Limiter) Wait(ctx context.Context, n int) error {
	if lim.l.Limit() == rate.Inf {
		return nil
	}
	// WaitN membutuhkan n <= burst; clamp kalau perlu.
	burst := lim.l.Burst()
	for n > 0 {
		take := n
		if take > burst {
			take = burst
		}
		if err := lim.l.WaitN(ctx, take); err != nil {
			return err
		}
		n -= take
	}
	return nil
}

func makeLimiter(bytesPerSec int64) *rate.Limiter {
	if bytesPerSec <= 0 {
		return rate.NewLimiter(rate.Inf, math.MaxInt32)
	}
	return rate.NewLimiter(rate.Limit(bytesPerSec), burstFor(bytesPerSec))
}

// burstFor menentukan burst size. Burst harus kecil — sebesar read buffer atau
// rate itu sendiri, mana yang lebih kecil. Burst besar (misal 2× rate) bikin
// download pendek lewat sepenuhnya tanpa kena throttle karena seluruh file
// masuk ke dalam burst awal.
func burstFor(bytesPerSec int64) int {
	burst := int64(readBufSize)
	if bytesPerSec < burst {
		burst = bytesPerSec
	}
	if burst < 1 {
		burst = 1
	}
	return int(burst)
}
