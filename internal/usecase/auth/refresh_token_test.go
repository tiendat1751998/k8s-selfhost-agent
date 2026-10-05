package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domainAuth "github.com/datdt/k8sselfhost/internal/domain/auth"
	"github.com/datdt/k8sselfhost/internal/domain/user"
)

type mockTxManager struct {
	mu sync.Mutex
}

func (m *mockTxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(ctx)
}

type mockConcurrentTokenRepo struct {
	mu     sync.Mutex
	tokens map[string]*user.RefreshToken
}

func newMockConcurrentTokenRepo() *mockConcurrentTokenRepo {
	return &mockConcurrentTokenRepo{
		tokens: make(map[string]*user.RefreshToken),
	}
}

func (m *mockConcurrentTokenRepo) Create(ctx context.Context, rt *user.RefreshToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rt.ID == "" {
		rt.ID = "rt-" + rt.TokenHash[:8]
	}
	rt.CreatedAt = time.Now()
	cp := *rt
	m.tokens[rt.TokenHash] = &cp
	return nil
}

func (m *mockConcurrentTokenRepo) GetByHash(ctx context.Context, hash string) (*user.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rt, ok := m.tokens[hash]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *rt
	return &cp, nil
}

func (m *mockConcurrentTokenRepo) GetByHashForUpdate(ctx context.Context, hash string) (*user.RefreshToken, error) {
	return m.GetByHash(ctx, hash)
}

func (m *mockConcurrentTokenRepo) Revoke(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rt := range m.tokens {
		if rt.ID == id {
			now := time.Now()
			rt.RevokedAt = &now
			return nil
		}
	}
	return nil
}

func (m *mockConcurrentTokenRepo) RevokeAllForUser(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rt := range m.tokens {
		if rt.UserID == userID {
			now := time.Now()
			rt.RevokedAt = &now
		}
	}
	return nil
}

func (m *mockConcurrentTokenRepo) DeleteExpired(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	for k, rt := range m.tokens {
		if time.Now().After(rt.ExpiresAt) {
			delete(m.tokens, k)
			count++
		}
	}
	return count, nil
}

func TestRotateRefreshToken_ConcurrentRaceCondition(t *testing.T) {
	userRepo := &mockUserRepo{
		users: map[string]*user.User{
			"u-1": {
				ID:           "u-1",
				Email:        "concurrent@example.com",
				PlatformRole: "viewer",
				TenantID:     "t-1",
				TenantRole:   "viewer",
			},
		},
	}

	tokenRepo := newMockConcurrentTokenRepo()
	txMgr := &mockTxManager{}
	uc := NewUsecase(userRepo, WithRefreshTokenRepo(tokenRepo), WithTxManager(txMgr))

	rawToken := "race-condition-test-token-123456"
	tokenHash := uc.tokenIssuer.HashToken(rawToken)

	tokenRepo.tokens[tokenHash] = &user.RefreshToken{
		ID:        "rt-seed-1",
		UserID:    "u-1",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	const concurrency = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	var successCount int
	var failCount int
	var errorsList []error

	startGate := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startGate // Synchronize all goroutines to fire simultaneously

			res, err := uc.RotateRefreshToken(context.Background(), RotateTokenInput{
				RefreshToken: rawToken,
				UserAgent:    "Go-Test-Agent",
				IPAddress:    "127.0.0.1",
			})

			mu.Lock()
			defer mu.Unlock()
			if err == nil && res != nil {
				successCount++
			} else {
				failCount++
				errorsList = append(errorsList, err)
			}
		}()
	}

	// Release all goroutines at once
	close(startGate)
	wg.Wait()

	// Guarantee that EXACTLY 1 succeeds and duplicates are rejected with reuse detection
	if successCount != 1 {
		t.Fatalf("expected exactly 1 success under concurrency, got %d. Failures: %d, Errors: %v",
			successCount, failCount, errorsList)
	}

	if failCount != concurrency-1 {
		t.Fatalf("expected %d failures, got %d", concurrency-1, failCount)
	}

	for _, err := range errorsList {
		if !errors.Is(err, domainAuth.ErrTokenRevoked) {
			t.Errorf("expected rejection with ErrTokenRevoked, got: %v", err)
		}
	}
}

func TestRotateRefreshToken_ValidSingle(t *testing.T) {
	userRepo := &mockUserRepo{
		users: map[string]*user.User{
			"u-1": {
				ID:           "u-1",
				Email:        "single@example.com",
				PlatformRole: "platform_admin",
				TenantID:     "t-admin",
				TenantRole:   "admin",
			},
		},
	}

	tokenRepo := newMockConcurrentTokenRepo()
	txMgr := &mockTxManager{}
	uc := NewUsecase(userRepo, WithRefreshTokenRepo(tokenRepo), WithTxManager(txMgr))

	rawToken := "valid-single-token-123"
	tokenHash := uc.tokenIssuer.HashToken(rawToken)

	tokenRepo.tokens[tokenHash] = &user.RefreshToken{
		ID:        "rt-single-1",
		UserID:    "u-1",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	res, err := uc.RotateRefreshToken(context.Background(), RotateTokenInput{
		RefreshToken: rawToken,
		UserAgent:    "TestAgent/1.0",
		IPAddress:    "10.0.0.1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if res.RawRefreshToken == "" || res.RawRefreshToken == rawToken {
		t.Errorf("expected newly generated refresh token, got %q", res.RawRefreshToken)
	}

	// Verify old token was revoked in repository
	oldRT := tokenRepo.tokens[tokenHash]
	if oldRT.RevokedAt == nil {
		t.Error("expected old token to be marked revoked")
	}

	// Verify new token exists in repository
	newHash := uc.tokenIssuer.HashToken(res.RawRefreshToken)
	newRT, ok := tokenRepo.tokens[newHash]
	if !ok {
		t.Fatalf("expected new refresh token in repo with hash %s", newHash)
	}
	if newRT.RevokedAt != nil {
		t.Error("new token must not be revoked")
	}
}

func TestRotateRefreshToken_ExpiredToken(t *testing.T) {
	userRepo := &mockUserRepo{
		users: map[string]*user.User{
			"u-1": {ID: "u-1", Email: "expired@example.com"},
		},
	}
	tokenRepo := newMockConcurrentTokenRepo()
	uc := NewUsecase(userRepo, WithRefreshTokenRepo(tokenRepo))

	rawToken := "expired-token-123"
	tokenHash := uc.tokenIssuer.HashToken(rawToken)
	tokenRepo.tokens[tokenHash] = &user.RefreshToken{
		ID:        "rt-exp-1",
		UserID:    "u-1",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	_, err := uc.RotateRefreshToken(context.Background(), RotateTokenInput{RefreshToken: rawToken})
	if !errors.Is(err, domainAuth.ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestRotateRefreshToken_AlreadyRevoked_ReuseDetected(t *testing.T) {
	userRepo := &mockUserRepo{
		users: map[string]*user.User{
			"u-1": {ID: "u-1", Email: "reused@example.com"},
		},
	}
	tokenRepo := newMockConcurrentTokenRepo()
	uc := NewUsecase(userRepo, WithRefreshTokenRepo(tokenRepo))

	now := time.Now()
	rawToken := "reused-token-123"
	tokenHash := uc.tokenIssuer.HashToken(rawToken)
	tokenRepo.tokens[tokenHash] = &user.RefreshToken{
		ID:        "rt-reuse-1",
		UserID:    "u-1",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		RevokedAt: &now,
	}

	_, err := uc.RotateRefreshToken(context.Background(), RotateTokenInput{RefreshToken: rawToken})
	if !errors.Is(err, domainAuth.ErrTokenRevoked) {
		t.Fatalf("expected ErrTokenRevoked for reuse, got %v", err)
	}
}

func TestRotateRefreshToken_InvalidInput(t *testing.T) {
	uc := NewUsecase(&mockUserRepo{})

	_, err := uc.RotateRefreshToken(context.Background(), RotateTokenInput{RefreshToken: "  "})
	if !errors.Is(err, domainAuth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for empty string, got %v", err)
	}

	_, err = uc.RotateRefreshToken(context.Background(), RotateTokenInput{RefreshToken: "non-existent"})
	if err == nil {
		t.Fatal("expected error when repo is nil or token not found")
	}
}

func TestRevokeRefreshToken(t *testing.T) {
	tokenRepo := newMockConcurrentTokenRepo()
	uc := NewUsecase(&mockUserRepo{}, WithRefreshTokenRepo(tokenRepo))

	rawToken := "logout-token-123"
	tokenHash := uc.tokenIssuer.HashToken(rawToken)
	tokenRepo.tokens[tokenHash] = &user.RefreshToken{
		ID:        "rt-logout-1",
		UserID:    "u-1",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := uc.RevokeRefreshToken(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("unexpected error revoking token: %v", err)
	}
	if tokenRepo.tokens[tokenHash].RevokedAt == nil {
		t.Error("expected token to be marked revoked")
	}
}
