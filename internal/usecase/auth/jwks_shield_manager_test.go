package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"sync"
	"testing"
	"time"

	domainauth "github.com/datdt/k8sselfhost/internal/domain/auth"
	usecaseauth "github.com/datdt/k8sselfhost/internal/usecase/auth"
)

// generateTestRSAKey creates an in-memory RSA key pair for testing without external deps.
func generateTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test RSA key: %v", err)
	}
	return privKey
}

func TestJWKSShieldManager_FastPathValidKey(t *testing.T) {
	privKey := generateTestRSAKey(t)
	now := time.Now()

	mgr := usecaseauth.NewJWKSShieldManager(
		usecaseauth.WithProviderURL("https://idp.example.com/.well-known/jwks.json"),
		usecaseauth.WithDefaultTTL(10*time.Minute),
	)

	mgr.InjectKey(&domainauth.JWKSKey{
		KID:       "key-active",
		PublicKey: &privKey.PublicKey,
		Algorithm: "RS256",
		FetchedAt: now.Add(-1 * time.Minute),
		ExpiresAt: now.Add(9 * time.Minute),
	})

	ctx := context.Background()
	pubKey, err := mgr.GetValidationKey(ctx, "key-active")
	if err != nil {
		t.Fatalf("unexpected error on fast path: %v", err)
	}
	if pubKey == nil || pubKey.N.Cmp(privKey.PublicKey.N) != 0 {
		t.Fatalf("retrieved public key does not match injected key")
	}

	status := mgr.GetCacheStatus()
	if status.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", status.Hits)
	}
	if status.IsUsingStaleKey {
		t.Errorf("expected is_using_stale_key to be false")
	}
}

func TestJWKSShieldManager_StaleWhileRevalidate_ServingStaleKeyOnIdPFailure(t *testing.T) {
	privKey := generateTestRSAKey(t)
	now := time.Now()

	// Mock fetcher that fails with 500 error simulating IdP outage
	mockFetcher := &mockJWKSFetcher{
		err: errors.New("HTTP 500: IdP internal server error"),
	}

	mgr := usecaseauth.NewJWKSShieldManager(
		usecaseauth.WithProviderURL("https://idp.example.com/.well-known/jwks.json"),
		usecaseauth.WithFetcher(mockFetcher),
		usecaseauth.WithClock(func() time.Time { return now }),
	)

	// Inject key that expired 10 minutes ago, but within 24h grace window
	mgr.InjectKey(&domainauth.JWKSKey{
		KID:       "key-stale",
		PublicKey: &privKey.PublicKey,
		Algorithm: "RS256",
		FetchedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-10 * time.Minute),
	})

	ctx := context.Background()
	pubKey, err := mgr.GetValidationKey(ctx, "key-stale")
	if err != nil {
		t.Fatalf("expected stale-while-revalidate to return stale key, got error: %v", err)
	}
	if pubKey == nil {
		t.Fatal("expected non-nil public key")
	}

	// Verify status reflects stale key serving and fetch error
	status := mgr.GetCacheStatus()
	if !status.IsUsingStaleKey {
		t.Errorf("expected is_using_stale_key to be true")
	}
	if status.StaleHits < 1 {
		t.Errorf("expected at least 1 stale hit, got %d", status.StaleHits)
	}
}

func TestJWKSShieldManager_RejectionAfter24hStaleWindow(t *testing.T) {
	privKey := generateTestRSAKey(t)
	now := time.Now()

	mockFetcher := &mockJWKSFetcher{
		err: errors.New("network timeout reaching IdP"),
	}

	mgr := usecaseauth.NewJWKSShieldManager(
		usecaseauth.WithFetcher(mockFetcher),
		usecaseauth.WithClock(func() time.Time { return now }),
	)

	// Key expired 25 hours ago (> 24h max stale window)
	mgr.InjectKey(&domainauth.JWKSKey{
		KID:       "key-exhausted",
		PublicKey: &privKey.PublicKey,
		Algorithm: "RS256",
		FetchedAt: now.Add(-30 * time.Hour),
		ExpiresAt: now.Add(-25 * time.Hour),
	})

	ctx := context.Background()
	_, err := mgr.GetValidationKey(ctx, "key-exhausted")
	if !errors.Is(err, domainauth.ErrJWKSStaleKeyExhausted) {
		t.Fatalf("expected ErrJWKSStaleKeyExhausted, got %v", err)
	}
}

func TestJWKSShieldManager_StaticBackupKeysetFallback(t *testing.T) {
	backupPrivKey := generateTestRSAKey(t)
	now := time.Now()

	mockFetcher := &mockJWKSFetcher{
		err: errors.New("IdP completely unreachable"),
	}

	mgr := usecaseauth.NewJWKSShieldManager(
		usecaseauth.WithFetcher(mockFetcher),
		usecaseauth.WithClock(func() time.Time { return now }),
	)

	// Configure static backup keyset (e.g. from K8s ConfigMap/Secret)
	backupKey := &domainauth.JWKSKey{
		KID:       "static-k8s-backup-key",
		PublicKey: &backupPrivKey.PublicKey,
		Algorithm: "RS256",
		FetchedAt: now,
		ExpiresAt: now.Add(365 * 24 * time.Hour),
	}
	mgr.SetStaticBackupKeys(map[string]*domainauth.JWKSKey{
		"static-k8s-backup-key": backupKey,
	})

	ctx := context.Background()
	pubKey, err := mgr.GetValidationKey(ctx, "static-k8s-backup-key")
	if err != nil {
		t.Fatalf("expected fallback to static backup key, got error: %v", err)
	}
	if pubKey == nil || pubKey.N.Cmp(backupPrivKey.PublicKey.N) != 0 {
		t.Fatalf("returned key did not match static backup key")
	}
}

func TestJWKSShieldManager_JitterRenewalDistribution(t *testing.T) {
	mgr := usecaseauth.NewJWKSShieldManager()
	ttl := 1 * time.Hour

	minAllowed := time.Duration(float64(ttl) * 0.60)
	maxAllowed := time.Duration(float64(ttl) * 0.90)

	const iterations = 1000
	for i := 0; i < iterations; i++ {
		renewTime := mgr.CalculateLeaseRenewal(ttl)
		if renewTime < minAllowed || renewTime > maxAllowed {
			t.Fatalf("iteration %d: renewal time %v outside 0.60-0.90 TTL window [%v, %v]",
				i, renewTime, minAllowed, maxAllowed)
		}
	}
}

func TestJWKSShieldManager_VaultAutoUnsealStatusEvaluator(t *testing.T) {
	mgr := usecaseauth.NewJWKSShieldManager()
	ctx := context.Background()

	// Case 1: Vault is unsealed -> OK
	unsealed := domainauth.VaultUnsealStatus{
		IsSealed:       false,
		Threshold:      3,
		Progress:       0,
		AutoUnsealMode: "tpm2",
		LastUnsealedAt: time.Now(),
	}
	res, err := mgr.EvaluateVaultSealStatus(ctx, unsealed, 0)
	if err != nil {
		t.Fatalf("expected no error for unsealed vault, got %v", err)
	}
	if res.IsSealed {
		t.Errorf("expected res.IsSealed to be false")
	}

	// Case 2: Vault sealed for 30s (< 60s grace threshold) -> OK (still pending startup)
	sealedGrace := domainauth.VaultUnsealStatus{
		IsSealed:       true,
		Threshold:      3,
		Progress:       1,
		AutoUnsealMode: "tpm2",
	}
	res, err = mgr.EvaluateVaultSealStatus(ctx, sealedGrace, 30*time.Second)
	if err != nil {
		t.Fatalf("expected no error within 60s seal grace period, got %v", err)
	}

	// Case 3: Vault sealed for 65s (> 60s) with no executor -> Deadlock error
	res, err = mgr.EvaluateVaultSealStatus(ctx, sealedGrace, 65*time.Second)
	if !errors.Is(err, domainauth.ErrVaultSealedDeadlock) {
		t.Fatalf("expected ErrVaultSealedDeadlock when sealed > 60s, got %v", err)
	}

	// Case 4: Vault sealed > 60s with successful unseal executor -> Returns unsealed status
	mockExecutor := &mockVaultExecutor{
		unsealStatus: domainauth.VaultUnsealStatus{
			IsSealed:       false,
			Threshold:      3,
			Progress:       3,
			AutoUnsealMode: "tpm2",
			LastUnsealedAt: time.Now(),
		},
	}
	mgrWithExecutor := usecaseauth.NewJWKSShieldManager(
		usecaseauth.WithVaultUnsealExecutor(mockExecutor),
	)
	res, err = mgrWithExecutor.EvaluateVaultSealStatus(ctx, sealedGrace, 70*time.Second)
	if err != nil {
		t.Fatalf("expected executor to recover from sealed state, got error: %v", err)
	}
	if res.IsSealed {
		t.Errorf("expected unsealed status after executor ran")
	}
}

func TestJWKSShieldManager_HighConcurrencyRaceSafety(t *testing.T) {
	privKey := generateTestRSAKey(t)
	now := time.Now()

	mgr := usecaseauth.NewJWKSShieldManager()
	mgr.InjectKey(&domainauth.JWKSKey{
		KID:       "conc-key",
		PublicKey: &privKey.PublicKey,
		Algorithm: "RS256",
		FetchedAt: now,
		ExpiresAt: now.Add(1 * time.Hour),
	})

	const goroutines = 50
	const ops = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				_, _ = mgr.GetValidationKey(context.Background(), "conc-key")
				_ = mgr.CalculateLeaseRenewal(10 * time.Minute)
				_ = mgr.GetCacheStatus()
			}
		}()
	}
	wg.Wait()

	status := mgr.GetCacheStatus()
	if status.Hits != uint64(goroutines*ops) {
		t.Errorf("expected %d hits, got %d", goroutines*ops, status.Hits)
	}
}

// Helpers for test mocks
type mockJWKSFetcher struct {
	keys []*domainauth.JWKSKey
	err  error
}

func (m *mockJWKSFetcher) FetchJWKS(ctx context.Context, providerURL string) ([]*domainauth.JWKSKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.keys, nil
}

type mockVaultExecutor struct {
	unsealStatus domainauth.VaultUnsealStatus
	err          error
}

func (m *mockVaultExecutor) ExecuteUnseal(ctx context.Context) (domainauth.VaultUnsealStatus, error) {
	if m.err != nil {
		return domainauth.VaultUnsealStatus{IsSealed: true}, m.err
	}
	return m.unsealStatus, nil
}
