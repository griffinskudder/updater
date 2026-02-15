// Package ratelimit provides rate limiting for HTTP requests using the token
// bucket algorithm. It supports two-tier rate limits (anonymous vs authenticated)
// and includes HTTP middleware that sets standard rate limit response headers.
package ratelimit

import "time"

// Limiter defines the rate limiting contract. Implementations must be safe for
// concurrent use.
type Limiter interface {
	// Allow checks whether a request identified by key should be allowed.
	// Returns whether the request is allowed and rate information for
	// populating response headers.
	Allow(key string) (allowed bool, info Info)

	// Close stops background goroutines and releases resources.
	Close()
}

// Info contains rate limit state for populating response headers.
type Info struct {
	Limit      int           // Maximum requests per window
	Remaining  int           // Approximate tokens remaining
	ResetAt    time.Time     // When the bucket will be full again
	RetryAfter time.Duration // How long to wait (meaningful only when denied)
}
