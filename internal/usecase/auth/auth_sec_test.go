package auth_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	domainauth "github.com/datdt/k8sselfhost/internal/domain/auth"
	"github.com/datdt/k8sselfhost/internal/domain/user"
	usecaseAuth "github.com/datdt/k8sselfhost/internal/usecase/auth"
)

type countingFetcher struct {
	fetchCount atomic.Int32
	blockCh    chan struct{}
}

func (f *countingFetcher) FetchJWKS(ctx context.Context, providerURL string) ([]*domainauth.JWKSKey, error) {
	f.fetchCount.Add(1)
	if f.blockCh != nil {
		<-f.blockCh
	}
	return []*domainauth.JWKSKey{
		{
			KID:       "key-1",
			Algorithm: "RS256",
			ExpiresAt: time.Now().Add(10 * time.Minute),
		},
	}, nil
}

func TestVerifyTOTP_SEC18_ReplayPrevention(t *testing.T) {
	setupTestEnv(t)
	repo := newMockUserRepo()
	repo.users["u-replay"] = &user.User{
		ID:    "u-replay",
		Email: "replay@example.com",
	}

	uc := usecaseAuth.NewUsecase(repo)
	setupRes, err := uc.SetupTOTP(context.Background(), "u-replay")
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}

	code, err := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generating totp code failed: %v", err)
	}

	_, err = uc.VerifyAndEnableTOTP(context.Background(), "u-replay", code)
	if err != nil {
		t.Fatalf("VerifyAndEnableTOTP failed: %v", err)
	}

	// First verification with valid code succeeds
	loginCode, err := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generating totp code failed: %v", err)
	}

	if err := uc.VerifyTOTP(context.Background(), "u-replay", loginCode); err != nil {
		t.Fatalf("first VerifyTOTP failed: %v", err)
	}

	// Immediate replay of the SAME valid code must be rejected
	err = uc.VerifyTOTP(context.Background(), "u-replay", loginCode)
	if err == nil {
		t.Fatal("expected replay of TOTP code to fail, got nil")
	}
}

func TestVerifyRecoveryCode_SEC13_RateLimiting(t *testing.T) {
	setupTestEnv(t)
	repo := newMockUserRepo()
	repo.users["u-rc-limit"] = &user.User{
		ID:    "u-rc-limit",
		Email: "rc@example.com",
	}

	uc := usecaseAuth.NewUsecase(repo)
	setupRes, _ := uc.SetupTOTP(context.Background(), "u-rc-limit")
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	recoveryCodes, err := uc.VerifyAndEnableTOTP(context.Background(), "u-rc-limit", code)
	if err != nil {
		t.Fatalf("VerifyAndEnableTOTP failed: %v", err)
	}

	// Attempt 5 invalid recovery codes
	for i := 0; i < 5; i++ {
		err := uc.VerifyRecoveryCode(context.Background(), "u-rc-limit", fmt.Sprintf("badcode%d", i))
		if err == nil {
			t.Fatalf("attempt %d: expected error for invalid recovery code, got nil", i+1)
		}
	}

	// 6th attempt should be blocked by rate limit, even with a valid recovery code
	err = uc.VerifyRecoveryCode(context.Background(), "u-rc-limit", recoveryCodes[0])
	if err == nil {
		t.Fatal("expected rate limit error on 6th recovery code attempt, got nil")
	}
	if err.Error() != "too many failed MFA attempts, please try again in 15 minutes" {
		t.Fatalf("unexpected rate limit message: %v", err)
	}
}

func TestJWKSShieldManager_PERF07_SingleflightDeduplication(t *testing.T) {
	fetcher := &countingFetcher{
		blockCh: make(chan struct{}),
	}

	mgr := usecaseAuth.NewJWKSShieldManager(
		usecaseAuth.WithProviderURL("https://idp.example.com/.well-known/jwks.json"),
		usecaseAuth.WithFetcher(fetcher),
	)

	const concurrency = 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_ = mgr.RefreshKeys(context.Background())
		}()
	}

	// Unblock all concurrent calls
	time.Sleep(50 * time.Millisecond)
	close(fetcher.blockCh)
	wg.Wait()

	if calls := fetcher.fetchCount.Load(); calls != 1 {
		t.Fatalf("expected singleflight to coalesce into 1 fetch, got %d", calls)
	}
}

func TestAuthenticate_SEC17_TimingSideChannelMitigation(t *testing.T) {
	pass := "secret123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)

	repo := &mockUserRepo{
		users: map[string]*user.User{
			"valid@example.com": {
				ID:           "u-1",
				Email:        "valid@example.com",
				PasswordHash: string(hash),
			},
		},
	}

	uc := usecaseAuth.NewUsecase(repo)

	// Measure non-existent user authentication time
	start := time.Now()
	_, err := uc.Authenticate(context.Background(), "nonexistent@example.com", pass)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// bcrypt.DefaultCost takes roughly > 30ms on modern CPUs, whereas a bare repo lookup without bcrypt takes < 1ms.
	// We verify that a non-existent user took at least 15ms due to dummy bcrypt hashing.
	if elapsed < 15*time.Millisecond {
		t.Errorf("Authenticate for non-existent user took only %v, indicates missing dummy bcrypt calculation", elapsed)
	}
}
