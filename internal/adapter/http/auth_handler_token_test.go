package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/user"
)

func TestRefreshToken_ValidCookie_ReturnsNewToken(t *testing.T) {
	handler, userRepo, tokenRepo, _ := setupAuthTest(t)
	userRepo.users["u1"] = &user.User{
		ID:           "u1",
		Email:        "refresh@example.com",
		PlatformRole: "viewer",
		TenantID:     "tenant-1",
		TenantRole:   "viewer",
	}

	rawToken, _ := middleware.GenerateRefreshToken()
	tokenRepo.tokens[middleware.HashToken(rawToken)] = &user.RefreshToken{
		ID:        "rt-1",
		UserID:    "u1",
		TokenHash: middleware.HashToken(rawToken),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: rawToken,
	})
	w := httptest.NewRecorder()

	handler.RefreshToken(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["token"] == nil || resp["token"] == "" {
		t.Fatal("expected new access token in response")
	}

	// Verify old token was revoked
	oldToken := tokenRepo.tokens[middleware.HashToken(rawToken)]
	if oldToken.RevokedAt == nil {
		t.Fatal("expected old refresh token to be revoked")
	}
}

func TestRefreshToken_ExpiredCookie_Returns401(t *testing.T) {
	handler, userRepo, tokenRepo, _ := setupAuthTest(t)
	userRepo.users["u1"] = &user.User{
		ID:    "u1",
		Email: "refresh@example.com",
	}

	rawToken, _ := middleware.GenerateRefreshToken()
	tokenRepo.tokens[middleware.HashToken(rawToken)] = &user.RefreshToken{
		ID:        "rt-1",
		UserID:    "u1",
		TokenHash: middleware.HashToken(rawToken),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // expired
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: rawToken,
	})
	w := httptest.NewRecorder()

	handler.RefreshToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for expired refresh token, got %d", w.Code)
	}
}

func TestRefreshToken_RevokedToken_Returns401(t *testing.T) {
	handler, userRepo, tokenRepo, _ := setupAuthTest(t)
	userRepo.users["u1"] = &user.User{
		ID:    "u1",
		Email: "refresh@example.com",
	}

	now := time.Now()
	rawToken, _ := middleware.GenerateRefreshToken()
	tokenRepo.tokens[middleware.HashToken(rawToken)] = &user.RefreshToken{
		ID:        "rt-1",
		UserID:    "u1",
		TokenHash: middleware.HashToken(rawToken),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		RevokedAt: &now, // revoked
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: rawToken,
	})
	w := httptest.NewRecorder()

	handler.RefreshToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for revoked token, got %d", w.Code)
	}
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	handler, _, tokenRepo, _ := setupAuthTest(t)

	rawToken, _ := middleware.GenerateRefreshToken()
	tokenRepo.tokens[middleware.HashToken(rawToken)] = &user.RefreshToken{
		ID:        "rt-1",
		UserID:    "u1",
		TokenHash: middleware.HashToken(rawToken),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: rawToken,
	})
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Verify token revoked in repo
	if tokenRepo.tokens[middleware.HashToken(rawToken)].RevokedAt == nil {
		t.Fatal("expected refresh token to be revoked upon logout")
	}

	// Verify cookie was cleared
	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil || refreshCookie.MaxAge != -1 {
		t.Fatal("expected refresh_token cookie to be expired/cleared")
	}
}

func TestRefreshToken_ConcurrentRaceCondition_OnlyOneSucceeds(t *testing.T) {
	handler, userRepo, tokenRepo, _ := setupAuthTest(t)
	userRepo.users["u1"] = &user.User{
		ID:           "u1",
		Email:        "refresh-concurrent@example.com",
		PlatformRole: "viewer",
		TenantID:     "tenant-1",
		TenantRole:   "viewer",
	}

	rawToken, _ := middleware.GenerateRefreshToken()
	tokenRepo.tokens[middleware.HashToken(rawToken)] = &user.RefreshToken{
		ID:        "rt-concurrent-1",
		UserID:    "u1",
		TokenHash: middleware.HashToken(rawToken),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	const concurrency = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	var statusCodes []int

	startGate := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startGate

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
			req.AddCookie(&http.Cookie{
				Name:  "refresh_token",
				Value: rawToken,
			})
			w := httptest.NewRecorder()

			handler.RefreshToken(w, req)

			mu.Lock()
			statusCodes = append(statusCodes, w.Code)
			mu.Unlock()
		}()
	}

	close(startGate)
	wg.Wait()

	var okCount, unauthorizedCount int
	for _, code := range statusCodes {
		switch code {
		case http.StatusOK:
			okCount++
		case http.StatusUnauthorized:
			unauthorizedCount++
		default:
			t.Errorf("unexpected status code: %d", code)
		}
	}

	if okCount != 1 {
		t.Fatalf("expected exactly 1 HTTP 200 OK under concurrency, got %d", okCount)
	}
	if unauthorizedCount != concurrency-1 {
		t.Fatalf("expected %d HTTP 401 Unauthorized, got %d", concurrency-1, unauthorizedCount)
	}
}
