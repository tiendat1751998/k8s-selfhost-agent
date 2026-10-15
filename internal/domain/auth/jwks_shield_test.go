package auth_test

import (
	"sync"
	"testing"
	"time"

	domainauth "github.com/datdt/k8sselfhost/internal/domain/auth"
)

func TestJWKSKey_ExpirationAndStaleness(t *testing.T) {
	now := time.Now()
	key := &domainauth.JWKSKey{
		KID:       "key-1",
		Algorithm: "RS256",
		FetchedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-1 * time.Hour),
	}

	if !key.IsExpired(now) {
		t.Errorf("expected key to be expired at %v", now)
	}

	// Stale within 24h
	if key.IsStaleExhausted(now, 24*time.Hour) {
		t.Errorf("expected key to NOT be exhausted within 24h stale window")
	}

	// 25 hours later -> exhausted
	futureNow := now.Add(24 * time.Hour)
	if !key.IsStaleExhausted(futureNow, 24*time.Hour) {
		t.Errorf("expected key to be exhausted after 24h stale window")
	}
}

func TestJWKSMetrics_Operations(t *testing.T) {
	var metrics domainauth.JWKSMetrics
	if metrics.Hits.Load() != 0 {
		t.Fatalf("expected 0, got %d", metrics.Hits.Load())
	}

	// Verify concurrent atomic updates do not trigger race conditions or data corruption
	const workers = 32
	const iterations = 500
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if id%4 == 0 {
					metrics.Hits.Inc()
				} else if id%4 == 1 {
					metrics.Misses.Inc()
				} else if id%4 == 2 {
					metrics.StaleHits.Inc()
				} else {
					metrics.RefreshErrors.Inc()
				}
			}
		}(i)
	}
	wg.Wait()

	expectedPerGroup := uint64((workers / 4) * iterations)
	if metrics.Hits.Load() != expectedPerGroup {
		t.Errorf("hits counter expected %d, got %d", expectedPerGroup, metrics.Hits.Load())
	}
	if metrics.Misses.Load() != expectedPerGroup {
		t.Errorf("misses counter expected %d, got %d", expectedPerGroup, metrics.Misses.Load())
	}
	if metrics.StaleHits.Load() != expectedPerGroup {
		t.Errorf("staleHits counter expected %d, got %d", expectedPerGroup, metrics.StaleHits.Load())
	}
	if metrics.RefreshErrors.Load() != expectedPerGroup {
		t.Errorf("refreshErrors counter expected %d, got %d", expectedPerGroup, metrics.RefreshErrors.Load())
	}
}
