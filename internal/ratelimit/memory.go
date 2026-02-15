package ratelimit

import (
	"math"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// entry holds a rate limiter and its last access time for cleanup.
type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// MemoryLimiter is an in-memory rate limiter backed by golang.org/x/time/rate.
// Each unique key gets its own token bucket. A background goroutine periodically
// evicts stale entries that have not been accessed within 2x the cleanup interval.
type MemoryLimiter struct {
	rate            rate.Limit
	burst           int
	limit           int // requests per minute, for Info.Limit
	cleanupInterval time.Duration

	mu      sync.Mutex
	entries map[string]*entry
	done    chan struct{}
	closed  bool
}

// NewMemoryLimiter creates a rate limiter with the given requests-per-minute rate,
// burst size, and cleanup interval. It starts a background goroutine for eviction.
func NewMemoryLimiter(requestsPerMinute int, burst int, cleanupInterval time.Duration) *MemoryLimiter {
	m := &MemoryLimiter{
		rate:            rate.Every(time.Minute / time.Duration(requestsPerMinute)),
		burst:           burst,
		limit:           requestsPerMinute,
		cleanupInterval: cleanupInterval,
		entries:         make(map[string]*entry),
		done:            make(chan struct{}),
	}
	go m.cleanup()
	return m
}

// Allow checks whether a request from the given key should be allowed.
func (m *MemoryLimiter) Allow(key string) (bool, Info) {
	m.mu.Lock()
	e, exists := m.entries[key]
	if !exists {
		e = &entry{
			limiter: rate.NewLimiter(m.rate, m.burst),
		}
		m.entries[key] = e
	}
	e.lastSeen = time.Now()
	m.mu.Unlock()

	allowed := e.limiter.Allow()

	now := time.Now()
	tokens := e.limiter.TokensAt(now)
	remaining := int(math.Max(0, math.Floor(tokens)))

	// Calculate reset time: how long until the bucket is full again
	tokensNeeded := float64(m.burst) - tokens
	var resetAt time.Time
	if tokensNeeded > 0 {
		resetDuration := time.Duration(tokensNeeded / float64(m.rate) * float64(time.Second))
		resetAt = now.Add(resetDuration)
	} else {
		resetAt = now
	}

	info := Info{
		Limit:     m.limit,
		Remaining: remaining,
		ResetAt:   resetAt,
	}

	if !allowed {
		// Calculate retry-after: time until the next token is available
		reservation := e.limiter.Reserve()
		delay := reservation.Delay()
		reservation.Cancel()
		info.RetryAfter = delay
	}

	return allowed, info
}

// Close stops the background cleanup goroutine.
func (m *MemoryLimiter) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.done)
	}
}

// cleanup periodically evicts entries that have not been accessed within
// 2x the cleanup interval.
func (m *MemoryLimiter) cleanup() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.evictStale()
		}
	}
}

// evictStale removes entries older than 2x the cleanup interval.
func (m *MemoryLimiter) evictStale() {
	cutoff := time.Now().Add(-2 * m.cleanupInterval)
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, e := range m.entries {
		if e.lastSeen.Before(cutoff) {
			delete(m.entries, key)
		}
	}
}
