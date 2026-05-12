package api

import (
	"sync"
	"time"
)

// slidingMinuteLimiter enforces a maximum number of "tokens" per UTC minute.
// A zero-value limiter or maxPerMinute <= 0 means unlimited.
type slidingMinuteLimiter struct {
	mu          sync.Mutex
	bucketStart int64 // Unix time truncated to minute
	count       int
	maxPerMin   int
}

func newSlidingMinuteLimiter(maxPerMinute int) *slidingMinuteLimiter {
	if maxPerMinute <= 0 {
		return nil
	}
	return &slidingMinuteLimiter{maxPerMin: maxPerMinute}
}

// take returns true if the request is allowed under the limit.
func (l *slidingMinuteLimiter) take() bool {
	if l == nil {
		return true
	}
	nowMin := time.Now().UTC().Unix() / 60
	l.mu.Lock()
	defer l.mu.Unlock()
	if nowMin != l.bucketStart {
		l.bucketStart = nowMin
		l.count = 0
	}
	if l.count >= l.maxPerMin {
		return false
	}
	l.count++
	return true
}
