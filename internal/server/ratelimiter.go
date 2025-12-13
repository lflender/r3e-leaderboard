package server

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements simple token bucket rate limiting per IP
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int           // requests per window
	window   time.Duration // time window
}

type visitor struct {
	tokens     int
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter
// rate: number of requests allowed per window
// window: time window (e.g., 1 minute)
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		window:   window,
	}

	// Cleanup old visitors every 5 minutes
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given IP should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[ip]

	if !exists {
		// New visitor - give them full tokens
		rl.visitors[ip] = &visitor{
			tokens:     rl.rate - 1,
			lastRefill: now,
		}
		return true
	}

	// Refill tokens based on time elapsed
	elapsed := now.Sub(v.lastRefill)
	if elapsed >= rl.window {
		// Full refill
		v.tokens = rl.rate - 1
		v.lastRefill = now
		return true
	}

	// Check if tokens available
	if v.tokens > 0 {
		v.tokens--
		return true
	}

	return false
}

// cleanup removes old visitors to prevent memory leak
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, v := range rl.visitors {
			if now.Sub(v.lastRefill) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware creates HTTP middleware for rate limiting
func (rl *RateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)

		if !rl.Allow(ip) {
			writeErrorResponse(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}

// getIP extracts the real IP address from request
func getIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}
