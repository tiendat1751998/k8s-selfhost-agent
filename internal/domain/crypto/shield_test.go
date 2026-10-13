package crypto_test

import (
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/datdt/k8sselfhost/internal/domain/crypto"
)

func TestPaddedCounter_64ByteCacheLineAlignment(t *testing.T) {
	var c crypto.PaddedCounter
	size := unsafe.Sizeof(c)
	if size != 64 {
		t.Fatalf("expected PaddedCounter size to be 64 bytes for CPU cache-line alignment, got %d", size)
	}

	c.Store(10)
	if val := c.Load(); val != 10 {
		t.Fatalf("expected loaded value 10, got %d", val)
	}

	if val := c.Inc(); val != 11 {
		t.Fatalf("expected incremented value 11, got %d", val)
	}

	if val := c.Add(5); val != 16 {
		t.Fatalf("expected added value 16, got %d", val)
	}
}

func TestPaddedCounter_HighConcurrencyMESISafety(t *testing.T) {
	var c crypto.PaddedCounter
	const workers = 32
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				c.Inc()
			}
		}()
	}
	wg.Wait()

	expected := uint64(workers * iterations)
	if c.Load() != expected {
		t.Fatalf("expected total %d, got %d", expected, c.Load())
	}
}

func TestOCSPStapleStatus_PrefetchAndGraceWindows(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	prefetchWindow := 4 * time.Hour
	graceWindow := 24 * time.Hour

	// NextUpdate is 3 hours away -> within 4h prefetch window
	status := &crypto.OCSPStapleStatus{
		Domain:       "api.enterprise.internal",
		HasStaple:    true,
		NextUpdate:   now.Add(3 * time.Hour),
		ResponderURL: "http://ocsp.pki.enterprise.internal",
	}

	if !status.NeedsPrefetch(now, prefetchWindow) {
		t.Errorf("expected NeedsPrefetch=true when within 4h of NextUpdate")
	}

	// NextUpdate is 10 hours away -> outside prefetch window
	status.NextUpdate = now.Add(10 * time.Hour)
	if status.NeedsPrefetch(now, prefetchWindow) {
		t.Errorf("expected NeedsPrefetch=false when 10h before NextUpdate")
	}

	// Expired 5 hours ago -> within 24h grace window
	status.NextUpdate = now.Add(-5 * time.Hour)
	if !status.IsWithinGrace(now, graceWindow) {
		t.Errorf("expected IsWithinGrace=true within 24h grace window")
	}

	// Expired 25 hours ago -> grace window exhausted
	status.NextUpdate = now.Add(-25 * time.Hour)
	if status.IsWithinGrace(now, graceWindow) {
		t.Errorf("expected IsWithinGrace=false when expired >24h")
	}
}

func TestEnvelopeCacheEntry_Lifecycle(t *testing.T) {
	now := time.Now()
	entry := &crypto.EnvelopeCacheEntry{
		KeyID:        "dek-key-vault-01",
		PlaintextKey: []byte("0123456789abcdef0123456789abcdef"),
		CachedAt:     now.Add(-30 * time.Minute),
		TTL:          1 * time.Hour,
		GracePeriod:  4 * time.Hour,
		IsStaleValid: false,
	}

	if !entry.IsFresh(now) {
		t.Errorf("expected entry cached 30m ago with 1h TTL to be fresh")
	}

	// Entry cached 2 hours ago (TTL expired, within 4h grace window)
	entry.CachedAt = now.Add(-2 * time.Hour)
	if entry.IsFresh(now) {
		t.Errorf("expected entry cached 2h ago with 1h TTL to NOT be fresh")
	}
	if !entry.IsWithinGrace(now) {
		t.Errorf("expected entry within 4h grace window to be valid")
	}

	// Entry cached 6 hours ago (TTL 1h + Grace 4h = 5h expired)
	entry.CachedAt = now.Add(-6 * time.Hour)
	if entry.IsWithinGrace(now) {
		t.Errorf("expected entry cached 6h ago to be grace exhausted")
	}
}

func TestHSMSessionPoolStatus_ValidationAndSaturation(t *testing.T) {
	status := crypto.HSMSessionPoolStatus{
		MaxSessions:       256,
		ActiveSessions:    256,
		AvailableSessions: 0,
		AcquireWaitTime:   500 * time.Millisecond,
		IsSaturated:       true,
	}

	if !status.IsSaturated {
		t.Errorf("expected pool to be saturated")
	}

	if err := status.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	invalid := crypto.HSMSessionPoolStatus{
		MaxSessions:    0,
		ActiveSessions: 1,
	}
	if err := invalid.Validate(); err == nil {
		t.Fatalf("expected validation error on MaxSessions <= 0")
	}
}
