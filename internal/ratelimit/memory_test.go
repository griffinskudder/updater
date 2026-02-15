package ratelimit

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryLimiter(t *testing.T) {
	limiter := NewMemoryLimiter(60, 10, 5*time.Minute)
	defer limiter.Close()

	assert.NotNil(t, limiter)
}

func TestMemoryLimiter_Allow_UnderLimit(t *testing.T) {
	limiter := NewMemoryLimiter(60, 10, 5*time.Minute)
	defer limiter.Close()

	allowed, info := limiter.Allow("192.168.1.1")
	assert.True(t, allowed)
	assert.Equal(t, 60, info.Limit)
	assert.True(t, info.Remaining >= 0)
	assert.False(t, info.ResetAt.IsZero())
}

func TestMemoryLimiter_Allow_ExceedsBurst(t *testing.T) {
	// Burst of 3, rate of 60/min -- 4th rapid request should be denied
	limiter := NewMemoryLimiter(60, 3, 5*time.Minute)
	defer limiter.Close()

	key := "192.168.1.1"

	for i := 0; i < 3; i++ {
		allowed, _ := limiter.Allow(key)
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}

	allowed, info := limiter.Allow(key)
	assert.False(t, allowed)
	assert.True(t, info.RetryAfter > 0)
}

func TestMemoryLimiter_Allow_DifferentKeys(t *testing.T) {
	limiter := NewMemoryLimiter(60, 2, 5*time.Minute)
	defer limiter.Close()

	// Exhaust key1's burst
	for i := 0; i < 2; i++ {
		limiter.Allow("key1")
	}
	allowed1, _ := limiter.Allow("key1")
	assert.False(t, allowed1, "key1 should be denied")

	// key2 should still be allowed
	allowed2, _ := limiter.Allow("key2")
	assert.True(t, allowed2, "key2 should be allowed")
}

func TestMemoryLimiter_Allow_InfoHeaders(t *testing.T) {
	limiter := NewMemoryLimiter(60, 5, 5*time.Minute)
	defer limiter.Close()

	_, info := limiter.Allow("test-key")
	assert.Equal(t, 60, info.Limit)
	assert.True(t, info.Remaining >= 0 && info.Remaining <= 5)
	assert.False(t, info.ResetAt.IsZero())
}

func TestMemoryLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewMemoryLimiter(1000, 100, 5*time.Minute)
	defer limiter.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("client-%d", id%5)
			for j := 0; j < 20; j++ {
				limiter.Allow(key)
			}
		}(i)
	}
	wg.Wait()
	// No panics or data races -- run with -race flag
}

func TestMemoryLimiter_Close(t *testing.T) {
	limiter := NewMemoryLimiter(60, 10, 100*time.Millisecond)
	limiter.Close()
	// Should not panic on double close or use after close
	limiter.Close()
}

func TestMemoryLimiter_Cleanup(t *testing.T) {
	// Use very short cleanup interval for testing
	limiter := NewMemoryLimiter(60, 10, 50*time.Millisecond)
	defer limiter.Close()

	limiter.Allow("ephemeral-key")

	// Verify the key exists
	limiter.mu.Lock()
	_, exists := limiter.entries["ephemeral-key"]
	limiter.mu.Unlock()
	require.True(t, exists, "key should exist before cleanup")

	// Wait for cleanup to run (2x cleanup interval for the staleness check)
	time.Sleep(200 * time.Millisecond)

	limiter.mu.Lock()
	_, exists = limiter.entries["ephemeral-key"]
	limiter.mu.Unlock()
	assert.False(t, exists, "key should be cleaned up after inactivity")
}
