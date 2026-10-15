package spiffeguard

import (
	"sync"
	"testing"
	"time"
)

func TestPaddedCounterInitialization(t *testing.T) {
	counter := PaddedCounter{}
	if counter.Load() != 0 {
		t.Fatalf("expected 0, got %d", counter.Load())
	}
}

func TestPaddedCounterConcurrentAccess(t *testing.T) {
	var counter PaddedCounter
	const numGoroutines = 100
	const incrementsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				counter.Inc()
			}
		}()
	}

	wg.Wait()

	expected := uint64(numGoroutines * incrementsPerGoroutine)
	if actual := counter.Load(); actual != expected {
		t.Fatalf("expected counter value %d, got %d", expected, actual)
	}
}

func TestIdentityMetricsConcurrentSafety(t *testing.T) {
	metrics := NewIdentityMetrics()
	const numWorkers = 50
	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(numWorkers * 6)

	// Worker 1: SVID Renewals
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.SVIDRenewalsTotal.Inc()
			}
		}()
	}

	// Worker 2: SVID Backlog Drops
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.SVIDBacklogDropsTotal.Inc()
			}
		}()
	}

	// Worker 3: Token Reloads
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.TokenReloadsTotal.Inc()
			}
		}()
	}

	// Worker 4: Token Emergency Requests
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.TokenEmergencyRequestsTotal.Inc()
			}
		}()
	}

	// Worker 5: Clock Skew Violations
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.ClockSkewViolationsTotal.Inc()
			}
		}()
	}

	// Worker 6: Active Tokens Watched
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.ActiveTokensWatched.Inc()
			}
		}()
	}

	wg.Wait()

	expected := uint64(numWorkers * iterations)
	if metrics.SVIDRenewalsTotal.Load() != expected {
		t.Errorf("SVIDRenewalsTotal: expected %d, got %d", expected, metrics.SVIDRenewalsTotal.Load())
	}
	if metrics.SVIDBacklogDropsTotal.Load() != expected {
		t.Errorf("SVIDBacklogDropsTotal: expected %d, got %d", expected, metrics.SVIDBacklogDropsTotal.Load())
	}
	if metrics.TokenReloadsTotal.Load() != expected {
		t.Errorf("TokenReloadsTotal: expected %d, got %d", expected, metrics.TokenReloadsTotal.Load())
	}
	if metrics.TokenEmergencyRequestsTotal.Load() != expected {
		t.Errorf("TokenEmergencyRequestsTotal: expected %d, got %d", expected, metrics.TokenEmergencyRequestsTotal.Load())
	}
	if metrics.ClockSkewViolationsTotal.Load() != expected {
		t.Errorf("ClockSkewViolationsTotal: expected %d, got %d", expected, metrics.ClockSkewViolationsTotal.Load())
	}
	if metrics.ActiveTokensWatched.Load() != expected {
		t.Errorf("ActiveTokensWatched: expected %d, got %d", expected, metrics.ActiveTokensWatched.Load())
	}
}

func TestSVIDStatusNearExpiryEvaluation(t *testing.T) {
	now := time.Now()
	totalLifetime := 1 * time.Hour

	// Case 1: TTL is 40 minutes (66.7% > 50%) -> NOT near expiry
	statusHealthy := NewSVIDStatus("spiffe://cluster.local/ns/default/sa/app", now.Add(40*time.Minute), totalLifetime, "01-A2-B3")
	if statusHealthy.IsNearExpiry {
		t.Errorf("expected IsNearExpiry to be false when TTL is 40m/60m, got true")
	}

	// Case 2: TTL is 20 minutes (33.3% <= 50%) -> Near expiry
	statusNearExpiry := NewSVIDStatus("spiffe://cluster.local/ns/default/sa/app", now.Add(20*time.Minute), totalLifetime, "01-A2-B3")
	if !statusNearExpiry.IsNearExpiry {
		t.Errorf("expected IsNearExpiry to be true when TTL is 20m/60m, got false")
	}

	// Case 3: TTL is negative (expired) -> Near expiry
	statusExpired := NewSVIDStatus("spiffe://cluster.local/ns/default/sa/app", now.Add(-5*time.Minute), totalLifetime, "01-A2-B3")
	if !statusExpired.IsNearExpiry {
		t.Errorf("expected IsNearExpiry to be true when expired, got false")
	}
}

func TestTokenWatchStatusStaleness(t *testing.T) {
	now := time.Now()

	// Case 1: Token expires in 10 minutes, last refreshed 1 minute ago -> Not stale
	tokenFresh := NewTokenWatchStatus("/var/run/secrets/token", now.Add(-1*time.Minute), now.Add(10*time.Minute), 0.5)
	if tokenFresh.IsStale {
		t.Errorf("expected fresh token to not be stale")
	}

	// Case 2: Token expired 10 seconds ago -> Stale
	tokenExpired := NewTokenWatchStatus("/var/run/secrets/token", now.Add(-1*time.Hour), now.Add(-10*time.Second), 0.0)
	if !tokenExpired.IsStale {
		t.Errorf("expected expired token to be marked stale")
	}
}

func TestStandardDomainErrors(t *testing.T) {
	if ErrSVIDBacklogSaturated == nil {
		t.Fatal("ErrSVIDBacklogSaturated must not be nil")
	}
	if ErrTokenClockSkewExceeded == nil {
		t.Fatal("ErrTokenClockSkewExceeded must not be nil")
	}
	if ErrTokenStaleExpired == nil {
		t.Fatal("ErrTokenStaleExpired must not be nil")
	}
}
