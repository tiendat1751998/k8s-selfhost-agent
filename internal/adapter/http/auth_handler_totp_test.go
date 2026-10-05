package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/user"
)

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
