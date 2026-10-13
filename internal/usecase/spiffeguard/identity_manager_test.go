package spiffeguard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/spiffeguard"
)

// helper to create a mock JWT token with given iat and exp
func createTestJWT(t *testing.T, sub string, iat, exp time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payloadData := map[string]interface{}{
		"iss": "https://kubernetes.default.svc.cluster.local",
		"sub": sub,
		"aud": []string{"https://kubernetes.default.svc"},
		"iat": iat.Unix(),
		"exp": exp.Unix(),
	}
	payloadBytes, err := json.Marshal(payloadData)
	if err != nil {
		t.Fatalf("failed to marshal jwt payload: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := base64.RawURLEncoding.EncodeToString([]byte("signature-bytes"))
	return fmt.Sprintf("%s.%s.%s", header, payload, sig)
}

func TestSVIDRenewalJitterStaggering(t *testing.T) {
	mgr := NewIdentityManager(IdentityManagerConfig{
		DefaultAllowedSkew: 60 * time.Second,
	})

	const numWorkloads = 200
	ttl := 30 * time.Minute
	maxJitter := 5 * time.Minute

	jitters := make(map[time.Duration]bool)
	var mu sync.Mutex

	var wg sync.WaitGroup
	wg.Add(numWorkloads)

	for i := 0; i < numWorkloads; i++ {
		go func() {
			defer wg.Done()
			j := mgr.CalculateJitter(ttl, maxJitter)
			mu.Lock()
			jitters[j] = true
			mu.Unlock()
		}()
	}

	wg.Wait()

	// In high-scale zero-trust environments, random nanosecond-resolution jitter ensures
	// that no two renewal attempts collide on the exact same microsecond timestamp.
	if len(jitters) < numWorkloads*95/100 {
		t.Fatalf("expected high diversity in calculated jitter (got %d unique out of %d)", len(jitters), numWorkloads)
	}
}

func TestSVIDRenewalBacklogSaturationAndRetry(t *testing.T) {
	metrics := domain.NewIdentityMetrics()
	dialAttempts := 0
	var mu sync.Mutex

	// Mock dialer simulating a saturated SPIRE Agent UDS backlog queue
	failingDialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		mu.Lock()
		dialAttempts++
		mu.Unlock()
		return nil, errors.New("connect: connection refused (backlog queue saturated)")
	}

	mgr := NewIdentityManager(IdentityManagerConfig{
		MaxRetries:  3,
		BaseBackoff: 1 * time.Millisecond,
	}, WithMetrics(metrics), WithUDSDialer(failingDialer))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := mgr.RenewSVIDWithJitter(ctx, "/tmp/spire-agent/public/api.sock", 10*time.Millisecond)
	if !errors.Is(err, domain.ErrSVIDBacklogSaturated) {
		t.Fatalf("expected ErrSVIDBacklogSaturated, got %v", err)
	}

	mu.Lock()
	attempts := dialAttempts
	mu.Unlock()

	// Should have retried 3 times (1 initial + 3 retries = 4 attempts)
	if attempts < 4 {
		t.Fatalf("expected at least 4 dial attempts before exhausting retries, got %d", attempts)
	}

	if metrics.SVIDBacklogDropsTotal.Load() != 1 {
		t.Fatalf("expected SVIDBacklogDropsTotal to be 1, got %d", metrics.SVIDBacklogDropsTotal.Load())
	}
}

func TestProjectedTokenWatcherFreshAndReload(t *testing.T) {
	tempDir := t.TempDir()
	tokenPath := filepath.Join(tempDir, "token")

	now := time.Now()
	initialToken := createTestJWT(t, "system:serviceaccount:default:payment-svc", now, now.Add(2*time.Hour))
	if err := os.WriteFile(tokenPath, []byte(initialToken), 0600); err != nil {
		t.Fatalf("failed to write initial token: %v", err)
	}

	mgr := NewIdentityManager(IdentityManagerConfig{})

	ctx := context.Background()
	status, err := mgr.WatchProjectedToken(ctx, tokenPath, 60*time.Second)
	if err != nil {
		t.Fatalf("WatchProjectedToken failed: %v", err)
	}

	if status.IsStale {
		t.Errorf("expected initial token to not be stale")
	}

	// Update token file with new expiration
	time.Sleep(10 * time.Millisecond) // ensure mtime difference
	updatedToken := createTestJWT(t, "system:serviceaccount:default:payment-svc", now, now.Add(4*time.Hour))
	if err := os.WriteFile(tokenPath, []byte(updatedToken), 0600); err != nil {
		t.Fatalf("failed to write updated token: %v", err)
	}

	statusUpdated, err := mgr.WatchProjectedToken(ctx, tokenPath, 60*time.Second)
	if err != nil {
		t.Fatalf("WatchProjectedToken updated failed: %v", err)
	}

	if statusUpdated.ExpiresAt.Before(status.ExpiresAt) {
		t.Errorf("expected updated token expiry to be after initial token")
	}

	if mgr.Metrics().TokenReloadsTotal.Load() < 1 {
		t.Errorf("expected TokenReloadsTotal >= 1, got %d", mgr.Metrics().TokenReloadsTotal.Load())
	}
}

func TestProjectedTokenEmergencyFallback(t *testing.T) {
	tempDir := t.TempDir()
	tokenPath := filepath.Join(tempDir, "expiring_token")

	now := time.Now()
	// Token expires in 2 minutes (< 5m threshold)
	nearExpiryToken := createTestJWT(t, "system:serviceaccount:default:order-svc", now.Add(-58*time.Minute), now.Add(2*time.Minute))
	if err := os.WriteFile(tokenPath, []byte(nearExpiryToken), 0600); err != nil {
		t.Fatalf("failed to write near expiry token: %v", err)
	}

	emergencyTriggered := false
	var mu sync.Mutex

	mgr := NewIdentityManager(IdentityManagerConfig{
		FallbackExpiry: 1 * time.Hour,
	}, WithTokenRequester(func(ctx context.Context, aud string, dur time.Duration) (*StoredToken, error) {
		mu.Lock()
		emergencyTriggered = true
		mu.Unlock()
		if dur > 1*time.Hour {
			return nil, errors.New("emergency token duration exceeds 1 hour ceiling")
		}
		return &StoredToken{
			RawToken:    "emergency-scoped-token-data",
			ParsedIAT:   time.Now(),
			ParsedEXP:   time.Now().Add(dur),
			Subject:     "system:serviceaccount:default:order-svc",
			Audience:    []string{aud},
			IsEmergency: true,
		}, nil
	}))

	ctx := context.Background()
	status, err := mgr.WatchProjectedToken(ctx, tokenPath, 60*time.Second)
	if err != nil {
		t.Fatalf("WatchProjectedToken emergency check failed: %v", err)
	}

	mu.Lock()
	triggered := emergencyTriggered
	mu.Unlock()

	if !triggered {
		t.Errorf("expected emergency fallback token request to be triggered when TTL < 5m")
	}

	if status.IsStale {
		t.Errorf("expected token not to be stale after emergency renewal")
	}

	if mgr.Metrics().TokenEmergencyRequestsTotal.Load() != 1 {
		t.Errorf("expected TokenEmergencyRequestsTotal to be 1, got %d", mgr.Metrics().TokenEmergencyRequestsTotal.Load())
	}
}

func TestTokenClockSkewValidation(t *testing.T) {
	mgr := NewIdentityManager(IdentityManagerConfig{})
	ctx := context.Background()

	fixedNow := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	mgr.SetNowFunc(func() time.Time {
		return fixedNow
	})

	allowedSkew := 60 * time.Second

	// Case 1: Token issued in past (iat = 11:59:50) -> Valid
	validPastIAT := fixedNow.Add(-10 * time.Second)
	if err := mgr.ValidateTokenClockSkew(ctx, validPastIAT, allowedSkew); err != nil {
		t.Errorf("expected valid past iat to pass, got: %v", err)
	}

	// Case 2: Token issued slightly in future within leeway (iat = 12:00:30, leeway = 60s) -> Valid
	validFutureIAT := fixedNow.Add(30 * time.Second)
	if err := mgr.ValidateTokenClockSkew(ctx, validFutureIAT, allowedSkew); err != nil {
		t.Errorf("expected valid future iat within leeway to pass, got: %v", err)
	}

	// Case 3: Token issued excessively in future (iat = 12:02:00, leeway = 60s) -> Clock Skew Exceeded
	invalidFutureIAT := fixedNow.Add(120 * time.Second)
	err := mgr.ValidateTokenClockSkew(ctx, invalidFutureIAT, allowedSkew)
	if !errors.Is(err, domain.ErrTokenClockSkewExceeded) {
		t.Errorf("expected ErrTokenClockSkewExceeded, got: %v", err)
	}

	if mgr.Metrics().ClockSkewViolationsTotal.Load() != 1 {
		t.Errorf("expected ClockSkewViolationsTotal to be 1, got %d", mgr.Metrics().ClockSkewViolationsTotal.Load())
	}
}

func TestConcurrentManagerLoad(t *testing.T) {
	mgr := NewIdentityManager(IdentityManagerConfig{
		DefaultAllowedSkew: 60 * time.Second,
	})

	const numWorkers = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(numWorkers * 2)

	now := time.Now()

	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = mgr.ValidateTokenClockSkew(context.Background(), now.Add(-5*time.Second), 60*time.Second)
			}
		}()
	}

	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = mgr.CalculateJitter(10*time.Minute, 1*time.Minute)
			}
		}()
	}

	wg.Wait()
}
