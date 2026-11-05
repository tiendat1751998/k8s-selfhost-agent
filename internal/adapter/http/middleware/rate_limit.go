package middleware

import (
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

const maxTrackedKeys = 50000

// clientEntry tracks timestamps of recent requests for an IP.
type clientEntry struct {
	timestamps []time.Time
	lastSeen   time.Time
}

// IPRateLimiter is a thread-safe in-memory sliding window rate limiter.
type IPRateLimiter struct {
	mu              sync.RWMutex
	clients         map[string]*clientEntry
	limit           int
	window          time.Duration
	cleanupInterval time.Duration
	stopCh          chan struct{}
	stopOnce        sync.Once
}

// NewIPRateLimiter constructs a new IPRateLimiter and starts its background janitor.
func NewIPRateLimiter(limit int, window time.Duration, cleanupInterval time.Duration) *IPRateLimiter {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}

	l := &IPRateLimiter{
		clients:         make(map[string]*clientEntry),
		limit:           limit,
		window:          window,
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
	}

	go l.janitor()
	return l
}

// Close stops the background janitor goroutine.
func (l *IPRateLimiter) Close() {
	l.stopOnce.Do(func() {
		close(l.stopCh)
	})
}

// janitor periodically runs cleanup to purge stale client entries.
func (l *IPRateLimiter) janitor() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.cleanup()
		}
	}
}

// cleanup removes clients that haven't been seen within the stale threshold.
func (l *IPRateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	staleThreshold := l.window * 2
	if staleThreshold < l.cleanupInterval {
		staleThreshold = l.cleanupInterval
	}

	for ip, entry := range l.clients {
		if now.Sub(entry.lastSeen) > staleThreshold {
			delete(l.clients, ip)
		}
	}
}

// ClientCount returns the number of tracked IP addresses.
func (l *IPRateLimiter) ClientCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.clients)
}

// RateLimitDecision holds rate limit decision metadata.
type RateLimitDecision struct {
	Allowed    bool
	Limit      int
	Remaining  int
	Used       int
	Reset      int64
	RetryAfter int
}

// Check checks rate limit status and returns detailed decision metadata.
func (l *IPRateLimiter) Check(ip string) RateLimitDecision {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, exists := l.clients[ip]
	if !exists {
		if len(l.clients) >= maxTrackedKeys {
			staleThreshold := l.window * 2
			for k, e := range l.clients {
				if now.Sub(e.lastSeen) > staleThreshold {
					delete(l.clients, k)
				}
			}
		}
		if len(l.clients) >= maxTrackedKeys {
			for k := range l.clients {
				delete(l.clients, k)
				break
			}
		}
		entry = &clientEntry{
			timestamps: make([]time.Time, 0, l.limit),
			lastSeen:   now,
		}
		l.clients[ip] = entry
	}
	entry.lastSeen = now

	// Prune timestamps older than the sliding window
	cutoff := now.Add(-l.window)
	valid := entry.timestamps[:0]
	for _, ts := range entry.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	entry.timestamps = valid

	used := len(entry.timestamps)
	reset := now.Add(l.window).Unix()
	if used > 0 {
		reset = entry.timestamps[0].Add(l.window).Unix()
	}

	if used < l.limit {
		entry.timestamps = append(entry.timestamps, now)
		used++
		remaining := l.limit - used
		if remaining < 0 {
			remaining = 0
		}
		return RateLimitDecision{
			Allowed:    true,
			Limit:      l.limit,
			Remaining:  remaining,
			Used:       used,
			Reset:      reset,
			RetryAfter: 0,
		}
	}

	// Calculate remaining time until oldest request in window expires
	oldest := entry.timestamps[0]
	retryAfter := int(math.Ceil(oldest.Add(l.window).Sub(now).Seconds()))
	if retryAfter < 1 {
		retryAfter = 1
	}
	return RateLimitDecision{
		Allowed:    false,
		Limit:      l.limit,
		Remaining:  0,
		Used:       l.limit,
		Reset:      reset,
		RetryAfter: retryAfter,
	}
}

// Allow checks if a request from the given IP is allowed under the rate limit.
func (l *IPRateLimiter) Allow(ip string) (bool, int) {
	d := l.Check(ip)
	return d.Allowed, d.RetryAfter
}

// extractClientIP extracts the client IP from RemoteAddr.
func extractClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

// Middleware returns an HTTP middleware enforcing the rate limiter.
func (l *IPRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r)
			decision := l.Check(ip)

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(decision.Remaining))
			w.Header().Set("X-RateLimit-Used", strconv.Itoa(decision.Used))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.Reset, 10))
			w.Header().Set("X-RateLimit-Resource", "auth")

			if !decision.Allowed {
				w.Header().Set("Retry-After", strconv.Itoa(decision.RetryAfter))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"error":       "too many requests",
					"retry_after": decision.RetryAfter,
				}); err != nil {
					zap.L().Warn("failed to encode rate limit json response", zap.Error(err))
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit returns a standard rate-limiting middleware with a 5-minute janitor cleanup.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	limiter := NewIPRateLimiter(limit, window, 5*time.Minute)
	return limiter.Middleware()
}
