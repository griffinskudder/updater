package ratelimit

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"updater/internal/models"
)

// Middleware returns HTTP middleware that enforces rate limits. It takes two
// limiters: one for anonymous requests (keyed by IP) and one for authenticated
// requests (keyed by API key name). The middleware reads the "api_key" context
// value set by the auth middleware to determine which limiter to use.
func Middleware(anonymous Limiter, authenticated Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, limiter := resolveKeyAndLimiter(r, anonymous, authenticated)

			allowed, info := limiter.Allow(key)

			// Always set rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", info.Limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", info.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", info.ResetAt.Unix()))

			if !allowed {
				retryAfterSecs := int(info.RetryAfter.Seconds()) + 1
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSecs))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)

				errorResp := models.NewErrorResponse("Rate limit exceeded", "RATE_LIMIT_EXCEEDED")
				json.NewEncoder(w).Encode(errorResp)

				slog.Warn("Rate limit exceeded",
					"key", key,
					"limit", info.Limit,
					"retry_after", retryAfterSecs,
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// resolveKeyAndLimiter determines the rate limit key and which limiter to use
// based on the request's authentication context.
func resolveKeyAndLimiter(r *http.Request, anonymous Limiter, authenticated Limiter) (string, Limiter) {
	if apiKey, ok := r.Context().Value("api_key").(*models.APIKey); ok && apiKey != nil {
		return "auth:" + apiKey.Name, authenticated
	}
	return getClientIP(r), anonymous
}

// getClientIP extracts the client IP from the request, checking proxy headers.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	return r.RemoteAddr
}
