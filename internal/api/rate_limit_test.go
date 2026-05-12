package api

import (
	"testing"
	"time"
)

func TestSlidingMinuteLimiter_unlimited(t *testing.T) {
	var l *slidingMinuteLimiter
	if !l.take() {
		t.Fatal("nil limiter should allow")
	}
	lim := newSlidingMinuteLimiter(0)
	if lim != nil {
		t.Fatal("max 0 should return nil")
	}
}

func TestSlidingMinuteLimiter_blocksAfterMax(t *testing.T) {
	lim := newSlidingMinuteLimiter(2)
	if lim == nil {
		t.Fatal("expected limiter")
	}
	first := lim.take()
	second := lim.take()
	if !first || !second {
		t.Fatal("expected first two allowed")
	}
	if lim.take() {
		t.Fatal("expected third blocked")
	}
}

func TestSlidingMinuteLimiter_resetsNewMinute(t *testing.T) {
	lim := &slidingMinuteLimiter{maxPerMin: 1, bucketStart: 0, count: 1}
	lim.bucketStart = time.Now().UTC().Unix()/60 - 1
	if !lim.take() {
		t.Fatal("expected allow after minute roll")
	}
}
