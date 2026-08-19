package http_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/user"
	usecaseAuth "github.com/datdt/k8sselfhost/internal/usecase/auth"
)

type mockUserRepo struct {
	users map[string]*user.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[string]*user.User),
	}
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			copied := *u
			return &copied, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*user.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, errors.New("user not found")
	}
	copied := *u
	return &copied, nil
}

func (m *mockUserRepo) UpdateMFA(ctx context.Context, userID, encryptedSecret string, enabled bool) error {
	u, ok := m.users[userID]
	if !ok {
		return errors.New("user not found")
	}
	if enabled {
		if encryptedSecret != "" {
			u.MFASecret = encryptedSecret
		}
		u.MFAEnabled = true
		now := time.Now()
		u.MFAVerifiedAt = &now
	} else {
		if encryptedSecret != "" {
			u.MFASecret = encryptedSecret
			u.MFAEnabled = false
		} else {
			u.MFASecret = ""
			u.MFAEnabled = false
			u.MFARecoveryCodes = ""
			u.MFAVerifiedAt = nil
		}
	}
	return nil
}

func (m *mockUserRepo) SetRecoveryCodes(ctx context.Context, userID, encryptedCodes string) error {
	u, ok := m.users[userID]
	if !ok {
		return errors.New("user not found")
	}
	u.MFARecoveryCodes = encryptedCodes
	return nil
}

func (m *mockUserRepo) GetMFASecret(ctx context.Context, userID string) (string, error) {
	u, ok := m.users[userID]
	if !ok {
		return "", errors.New("user not found")
	}
	return u.MFASecret, nil
}

type mockRefreshTokenRepo struct {
	tokens map[string]*user.RefreshToken
}

func newMockRefreshTokenRepo() *mockRefreshTokenRepo {
	return &mockRefreshTokenRepo{
		tokens: make(map[string]*user.RefreshToken),
	}
}

func (m *mockRefreshTokenRepo) Create(ctx context.Context, rt *user.RefreshToken) error {
	if rt.ID == "" {
		rt.ID = "rt-" + rt.TokenHash[:8]
	}
	rt.CreatedAt = time.Now()
	copied := *rt
	m.tokens[rt.TokenHash] = &copied
	return nil
}

func (m *mockRefreshTokenRepo) GetByHash(ctx context.Context, hash string) (*user.RefreshToken, error) {
	rt, ok := m.tokens[hash]
	if !ok {
		return nil, errors.New("refresh token not found")
	}
	copied := *rt
	return &copied, nil
}

func (m *mockRefreshTokenRepo) Revoke(ctx context.Context, id string) error {
	for _, rt := range m.tokens {
		if rt.ID == id {
			now := time.Now()
			rt.RevokedAt = &now
			return nil
		}
	}
	return nil
}

func (m *mockRefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID string) error {
	for _, rt := range m.tokens {
		if rt.UserID == userID {
			now := time.Now()
			rt.RevokedAt = &now
		}
	}
	return nil
}

func (m *mockRefreshTokenRepo) DeleteExpired(ctx context.Context) (int64, error) {
	var count int64
	for k, rt := range m.tokens {
		if time.Now().After(rt.ExpiresAt) {
			delete(m.tokens, k)
			count++
		}
	}
	return count, nil
}

func setupAuthTest(t *testing.T) (*adapthttp.AuthHandler, *mockUserRepo, *mockRefreshTokenRepo, *usecaseAuth.Usecase) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")

	userRepo := newMockUserRepo()
	tokenRepo := newMockRefreshTokenRepo()
	uc := usecaseAuth.NewUsecase(userRepo)
	handler := adapthttp.NewAuthHandler(uc, userRepo, tokenRepo)

	return handler, userRepo, tokenRepo, uc
}

func TestLogin_WithoutMFA_ReturnsTokenDirectly(t *testing.T) {
	handler, userRepo, _, _ := setupAuthTest(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("secretpass123"), bcrypt.DefaultCost)
	userRepo.users["u1"] = &user.User{
		ID:           "u1",
		Email:        "test@example.com",
		PasswordHash: string(hash),
		PlatformRole: "platform_admin",
		TenantID:     "tenant-1",
		TenantRole:   "admin",
		MFAEnabled:   false,
	}

	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "secretpass123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["token"] == nil || resp["token"] == "" {
		t.Fatal("expected access token in response")
	}

	// Verify refresh_token cookie was set
	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil || refreshCookie.Value == "" {
		t.Fatal("expected refresh_token cookie to be set")
	}
	if !refreshCookie.HttpOnly {
		t.Fatal("expected refresh_token cookie to be HttpOnly")
	}
}

func TestLogin_WithMFA_ReturnsPartialToken(t *testing.T) {
	handler, userRepo, _, _ := setupAuthTest(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("secretpass123"), bcrypt.DefaultCost)
	userRepo.users["u1"] = &user.User{
		ID:           "u1",
		Email:        "mfa@example.com",
		PasswordHash: string(hash),
		PlatformRole: "viewer",
		TenantID:     "tenant-1",
		TenantRole:   "viewer",
		MFAEnabled:   true,
	}

	body, _ := json.Marshal(map[string]string{
		"email":    "mfa@example.com",
		"password": "secretpass123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["mfa_required"] != true {
		t.Fatal("expected mfa_required to be true")
	}
	if resp["partial_token"] == nil || resp["partial_token"] == "" {
		t.Fatal("expected partial_token in response")
	}
	if resp["token"] != nil {
		t.Fatal("expected access token to NOT be present when MFA required")
	}
}

func TestVerifyMFA_ValidCode_ReturnsFullToken(t *testing.T) {
	handler, userRepo, _, uc := setupAuthTest(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("secretpass123"), bcrypt.DefaultCost)
	userRepo.users["u1"] = &user.User{
		ID:           "u1",
		Email:        "mfa@example.com",
		PasswordHash: string(hash),
		PlatformRole: "platform_admin",
		TenantID:     "tenant-1",
		TenantRole:   "admin",
	}

	setupRes, err := uc.SetupTOTP(context.Background(), "u1")
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	_, _ = uc.VerifyAndEnableTOTP(context.Background(), "u1", code)

	partialToken, err := middleware.GeneratePartialToken("u1")
	if err != nil {
		t.Fatalf("GeneratePartialToken failed: %v", err)
	}

	validCode, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	body, _ := json.Marshal(map[string]string{
		"partial_token": partialToken,
		"code":          validCode,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-mfa", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.VerifyMFA(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["token"] == nil || resp["token"] == "" {
		t.Fatal("expected full access token in response")
	}

	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil || refreshCookie.Value == "" {
		t.Fatal("expected refresh_token cookie")
	}
}

func TestVerifyMFA_InvalidCode_Returns401(t *testing.T) {
	handler, userRepo, _, uc := setupAuthTest(t)
	userRepo.users["u1"] = &user.User{
		ID:         "u1",
		Email:      "mfa@example.com",
		MFAEnabled: true,
	}

	setupRes, _ := uc.SetupTOTP(context.Background(), "u1")
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	_, _ = uc.VerifyAndEnableTOTP(context.Background(), "u1", code)

	partialToken, _ := middleware.GeneratePartialToken("u1")

	body, _ := json.Marshal(map[string]string{
		"partial_token": partialToken,
		"code":          "000000",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-mfa", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.VerifyMFA(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}
}

func TestVerifyMFA_ExpiredPartialToken_Returns401(t *testing.T) {
	handler, userRepo, _, uc := setupAuthTest(t)
	userRepo.users["u1"] = &user.User{
		ID:         "u1",
		Email:      "mfa@example.com",
		MFAEnabled: true,
	}
	setupRes, _ := uc.SetupTOTP(context.Background(), "u1")
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	_, _ = uc.VerifyAndEnableTOTP(context.Background(), "u1", code)

	// Craft an expired partial token
	header := `{"alg":"HS256","typ":"JWT"}`
	expiredClaims := middleware.JWTClaims{
		Sub:  "u1",
		Type: middleware.TokenTypePartial,
		Exp:  time.Now().Add(-10 * time.Minute).Unix(),
		Iat:  time.Now().Add(-15 * time.Minute).Unix(),
		Iss:  middleware.JWTIssuer,
	}
	payloadBytes, _ := json.Marshal(expiredClaims)
	encH := base64.RawURLEncoding.EncodeToString([]byte(header))
	encP := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingStr := encH + "." + encP
	h := hmac.New(sha256.New, []byte("k8sselfhost-jwt-test-secret-key-32bytes-secure!"))
	h.Write([]byte(signingStr))
	expiredToken := signingStr + "." + base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	validCode, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	body, _ := json.Marshal(map[string]string{
		"partial_token": expiredToken,
		"code":          validCode,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-mfa", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.VerifyMFA(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for expired token, got %d", w.Code)
	}
}

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

func TestTOTPSetup_ReturnsQRCode(t *testing.T) {
	handler, userRepo, _, _ := setupAuthTest(t)
	userRepo.users["u1"] = &user.User{
		ID:    "u1",
		Email: "setup@example.com",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/setup", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "u1")
	w := httptest.NewRecorder()

	handler.SetupTOTP(w, req.WithContext(ctx))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["secret"] == nil || resp["secret"] == "" {
		t.Fatal("expected secret in setup response")
	}
	if resp["qr_uri"] == nil || resp["qr_uri"] == "" {
		t.Fatal("expected qr_uri in setup response")
	}
}

func TestTOTPVerifySetup_EnablesMFA(t *testing.T) {
	handler, userRepo, _, uc := setupAuthTest(t)
	userRepo.users["u1"] = &user.User{
		ID:    "u1",
		Email: "verify-setup@example.com",
	}

	setupRes, err := uc.SetupTOTP(context.Background(), "u1")
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}

	validCode, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())

	body, _ := json.Marshal(map[string]string{
		"code": validCode,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/verify-setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "u1")
	w := httptest.NewRecorder()

	handler.VerifyTOTPSetup(w, req.WithContext(ctx))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	recoveryCodes, ok := resp["recovery_codes"].([]interface{})
	if !ok || len(recoveryCodes) != 10 {
		t.Fatalf("expected 10 recovery codes in response, got %v", resp["recovery_codes"])
	}

	if !userRepo.users["u1"].MFAEnabled {
		t.Fatal("expected user MFAEnabled to be true in repository")
	}
}

func TestTOTPDisable_RequiresAuth(t *testing.T) {
	handler, userRepo, _, uc := setupAuthTest(t)
	pwdHash, _ := bcrypt.GenerateFromPassword([]byte("mypassword"), bcrypt.DefaultCost)
	userRepo.users["u1"] = &user.User{
		ID:           "u1",
		Email:        "disable@example.com",
		PasswordHash: string(pwdHash),
	}

	setupRes, _ := uc.SetupTOTP(context.Background(), "u1")
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	_, _ = uc.VerifyAndEnableTOTP(context.Background(), "u1", code)

	// 1. Unauthenticated request
	body, _ := json.Marshal(map[string]string{
		"password": "mypassword",
		"code":     code,
	})
	reqUnauth := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/disable", bytes.NewReader(body))
	reqUnauth.Header.Set("Content-Type", "application/json")
	wUnauth := httptest.NewRecorder()

	handler.DisableTOTP(wUnauth, reqUnauth)

	if wUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 for unauthenticated request, got %d", wUnauth.Code)
	}

	// 2. Authenticated request with valid credentials
	currentCode, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	bodyAuth, _ := json.Marshal(map[string]string{
		"password": "mypassword",
		"code":     currentCode,
	})
	reqAuth := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/disable", bytes.NewReader(bodyAuth))
	reqAuth.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(reqAuth.Context(), middleware.UserIDKey, "u1")
	wAuth := httptest.NewRecorder()

	handler.DisableTOTP(wAuth, reqAuth.WithContext(ctx))

	if wAuth.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", wAuth.Code, wAuth.Body.String())
	}

	if userRepo.users["u1"].MFAEnabled {
		t.Fatal("expected MFA to be disabled")
	}
}
