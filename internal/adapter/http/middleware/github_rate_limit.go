package middleware

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

const maxGitHubTrackedKeys = 50000

// GitHubRateLimitConfig defines operational limits modeled after GitHub's REST API rate limiting.
type GitHubRateLimitConfig struct {
	PrimaryLimit    int
	PrimaryWindow   time.Duration
	MutationLimit   int
	MutationWindow  time.Duration
	MaxConcurrent   int
	CleanupInterval time.Duration
}

// DefaultGitHubRateLimitConfig returns GitHub's standard rate limiting parameters.
func DefaultGitHubRateLimitConfig() GitHubRateLimitConfig {
	return GitHubRateLimitConfig{
		PrimaryLimit:    5000,
		PrimaryWindow:   1 * time.Hour,
		MutationLimit:   80,
		MutationWindow:  1 * time.Minute,
		MaxConcurrent:   100,
		CleanupInterval: 5 * time.Minute,
	}
}

type slidingWindowCounter struct {
	windowStart time.Time
	currCount   int
	prevCount   int
}

func (sc *slidingWindowCounter) record(now time.Time, window time.Duration, limit int) (allowed bool, remaining, used int, reset int64, retryAfter int) {
	if sc.windowStart.IsZero() {
		sc.windowStart = now
	}
	if now.Sub(sc.windowStart) >= window {
		if now.Sub(sc.windowStart) >= 2*window {
			sc.prevCount = 0
			sc.currCount = 0
			sc.windowStart = now
		} else {
			sc.prevCount = sc.currCount
			sc.currCount = 0
			sc.windowStart = sc.windowStart.Add(window)
		}
	}

	elapsed := now.Sub(sc.windowStart)
	weight := 1.0 - float64(elapsed)/float64(window)
	if weight < 0 {
		weight = 0
	}
	estimated := sc.currCount + int(float64(sc.prevCount)*weight)
	reset = sc.windowStart.Add(window).Unix()

	if estimated < limit {
		sc.currCount++
		estimated++
		remaining = limit - estimated
		if remaining < 0 {
			remaining = 0
		}
		return true, remaining, estimated, reset, 0
	}

	retryAfter = int(math.Ceil(sc.windowStart.Add(window).Sub(now).Seconds()))
	if retryAfter < 1 {
		retryAfter = 1
	}
	return false, 0, limit, reset, retryAfter
}

type identityEntry struct {
	inFlight atomic.Int64
	mu       sync.Mutex
	lastSeen time.Time
	primary  slidingWindowCounter
	mutation slidingWindowCounter
}

// GitHubRateLimiter enforces GitHub-grade primary, secondary mutation, and concurrency limits.
type GitHubRateLimiter struct {
	mu         sync.RWMutex
	identities map[string]*identityEntry
	cfg        GitHubRateLimitConfig
	stopCh     chan struct{}
	stopOnce   sync.Once
}

// NewGitHubRateLimiter constructs a new GitHubRateLimiter and launches its janitor.
func NewGitHubRateLimiter(cfg GitHubRateLimitConfig) *GitHubRateLimiter {
	if cfg.PrimaryLimit <= 0 {
		cfg.PrimaryLimit = 5000
	}
	if cfg.PrimaryWindow <= 0 {
		cfg.PrimaryWindow = time.Hour
	}
	if cfg.MutationLimit <= 0 {
		cfg.MutationLimit = 80
	}
	if cfg.MutationWindow <= 0 {
		cfg.MutationWindow = time.Minute
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 100
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}

	l := &GitHubRateLimiter{
		identities: make(map[string]*identityEntry),
		cfg:        cfg,
		stopCh:     make(chan struct{}),
	}
	go l.janitor()
	return l
}

// Close stops the background janitor.
func (l *GitHubRateLimiter) Close() {
	l.stopOnce.Do(func() {
		close(l.stopCh)
	})
}

func (l *GitHubRateLimiter) janitor() {
	ticker := time.NewTicker(l.cfg.CleanupInterval)
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

func (l *GitHubRateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	threshold := l.cfg.PrimaryWindow * 2
	if threshold < l.cfg.CleanupInterval {
		threshold = l.cfg.CleanupInterval
	}

	for k, entry := range l.identities {
		entry.mu.Lock()
		lastSeen := entry.lastSeen
		inFlight := entry.inFlight.Load()
		entry.mu.Unlock()

		if inFlight == 0 && now.Sub(lastSeen) > threshold {
			delete(l.identities, k)
		}
	}
}

func (l *GitHubRateLimiter) getOrCreateIdentity(key string, now time.Time) *identityEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, exists := l.identities[key]
	if !exists {
		if len(l.identities) >= maxGitHubTrackedKeys {
			for k, e := range l.identities {
				if e.inFlight.Load() == 0 {
					delete(l.identities, k)
					break
				}
			}
		}
		entry = &identityEntry{lastSeen: now}
		l.identities[key] = entry
	}
	entry.lastSeen = now
	return entry
}

func (l *GitHubRateLimiter) extractIdentity(r *http.Request) string {
	if meta := tenancy.RequestMetaFromContext(r.Context()); meta != nil {
		tid := meta.TenantID()
		uid := meta.UserID()
		if tid != "" && uid != "" {
			return tid + ":" + uid
		}
		if uid != "" {
			return uid
		}
	}
	return extractClientIP(r)
}

func isMutatingRequest(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}

	if IsStreamingRequest(r) ||
		strings.HasPrefix(r.URL.Path, "/api/v1/logs") ||
		strings.HasPrefix(r.URL.Path, "/api/v1/agents/register") ||
		IsReadOnlyPostPath(r.URL.Path) {
		return false
	}

	return true
}

// Middleware returns the HTTP middleware enforcing GitHub rate limits.
func (l *GitHubRateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			key := l.extractIdentity(r)
			entry := l.getOrCreateIdentity(key, now)

			// 1. Secondary Concurrency check
			isStreaming := IsStreamingRequest(r)
			if !isStreaming {
				currInFlight := entry.inFlight.Add(1)
				if currInFlight > int64(l.cfg.MaxConcurrent) {
					entry.inFlight.Add(-1)
					w.Header().Set("Retry-After", "1")
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusTooManyRequests)
					if err := json.NewEncoder(w).Encode(map[string]interface{}{
						"error":       "secondary rate limit exceeded: too many concurrent requests",
						"retry_after": 1,
					}); err != nil {
						zap.L().Warn("failed to encode concurrent rate limit response", zap.Error(err))
					}
					return
				}
				defer entry.inFlight.Add(-1)
			}

			// 2. Secondary Mutation check
			if isMutatingRequest(r) {
				entry.mu.Lock()
				mutAllowed, _, _, mutReset, mutRetry := entry.mutation.record(now, l.cfg.MutationWindow, l.cfg.MutationLimit)
				entry.mu.Unlock()

				if !mutAllowed {
					w.Header().Set("X-RateLimit-Resource", "mutation")
					w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.cfg.MutationLimit))
					w.Header().Set("X-RateLimit-Remaining", "0")
					w.Header().Set("X-RateLimit-Used", strconv.Itoa(l.cfg.MutationLimit))
					w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(mutReset, 10))
					w.Header().Set("Retry-After", strconv.Itoa(mutRetry))
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusTooManyRequests)
					if err := json.NewEncoder(w).Encode(map[string]interface{}{
						"error":       "secondary rate limit exceeded: too many mutating requests",
						"retry_after": mutRetry,
					}); err != nil {
						zap.L().Warn("failed to encode mutation rate limit response", zap.Error(err))
					}
					return
				}
			}

			// 3. Primary Core rate limit check
			entry.mu.Lock()
			coreAllowed, coreRemaining, coreUsed, coreReset, coreRetry := entry.primary.record(now, l.cfg.PrimaryWindow, l.cfg.PrimaryLimit)
			entry.mu.Unlock()

			w.Header().Set("X-RateLimit-Resource", "core")
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(l.cfg.PrimaryLimit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(coreRemaining))
			w.Header().Set("X-RateLimit-Used", strconv.Itoa(coreUsed))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(coreReset, 10))

			if !coreAllowed {
				w.Header().Set("Retry-After", strconv.Itoa(coreRetry))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"error":       "rate limit exceeded",
					"retry_after": coreRetry,
				}); err != nil {
					zap.L().Warn("failed to encode primary rate limit response", zap.Error(err))
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GitHubRateLimit returns a middleware configured with given config.
func GitHubRateLimit(cfg GitHubRateLimitConfig) func(http.Handler) http.Handler {
	limiter := NewGitHubRateLimiter(cfg)
	return limiter.Middleware()
}
