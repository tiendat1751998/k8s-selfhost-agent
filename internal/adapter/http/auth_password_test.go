package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/user"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

func TestAuth_RequiresPasswordChange_And_ChangePassword(t *testing.T) {
	_ = middleware.SetJWTSecret("test-secret-at-least-32-characters-long-key")
	handler, userRepo, tokenRepo, _ := setupAuthTest(t)

	// Create user with requires_password_change = true
	hash, _ := bcrypt.GenerateFromPassword([]byte("InitialPassword123!"), bcrypt.DefaultCost)
	u := &user.User{
		ID:                     "user-pwd-change-1",
		Email:                  "admin@k8sselfhost.local",
		PasswordHash:           string(hash),
		PlatformRole:           "platform_admin",
		TenantID:               "default-tenant",
		TenantRole:             "admin",
		RequiresPasswordChange: true,
	}
	userRepo.users[u.ID] = u

	healthHandler := health.NewHandler(5 * time.Second)
	router := adapthttp.NewRouterWithPlatform(healthHandler, nil, &adapthttp.PlatformHandlers{
		Auth: handler,
	})

	// 1. Login returns requires_password_change = true
	loginBody := `{"email":"admin@k8sselfhost.local","password":"InitialPassword123!"}`
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	router.ServeHTTP(wLogin, reqLogin)

	if wLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d: %s", wLogin.Code, wLogin.Body.String())
	}

	var loginResp struct {
		Token                  string `json:"token"`
		RequiresPasswordChange bool   `json:"requires_password_change"`
		User                   struct {
			RequiresPasswordChange bool `json:"requires_password_change"`
		} `json:"user"`
	}
	if err := json.Unmarshal(wLogin.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("failed to decode login resp: %v", err)
	}
	if !loginResp.RequiresPasswordChange || !loginResp.User.RequiresPasswordChange {
		t.Fatalf("expected requires_password_change=true on login response, got: %+v", loginResp)
	}
	accessToken := loginResp.Token

	// 2. GET /api/v1/auth/me returns requires_password_change = true
	reqMe := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+accessToken)
	wMe := httptest.NewRecorder()
	router.ServeHTTP(wMe, reqMe)

	if wMe.Code != http.StatusOK {
		t.Fatalf("expected 200 on /auth/me, got %d: %s", wMe.Code, wMe.Body.String())
	}
	var meResp struct {
		ID                     string `json:"id"`
		Email                  string `json:"email"`
		RequiresPasswordChange bool   `json:"requires_password_change"`
	}
	if err := json.Unmarshal(wMe.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("failed to decode me resp: %v", err)
	}
	if !meResp.RequiresPasswordChange {
		t.Fatalf("expected /auth/me to return requires_password_change=true")
	}

	// 3. POST /api/v1/auth/change-password with weak password fails (< 10 chars)
	weakBody := `{"current_password":"InitialPassword123!","new_password":"short"}`
	reqWeak := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(weakBody))
	reqWeak.Header.Set("Content-Type", "application/json")
	reqWeak.Header.Set("Authorization", "Bearer "+accessToken)
	wWeak := httptest.NewRecorder()
	router.ServeHTTP(wWeak, reqWeak)

	if wWeak.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for password < 10 chars, got %d", wWeak.Code)
	}

	// 4. POST /api/v1/auth/change-password with invalid current password fails
	wrongCurBody := `{"current_password":"WrongCurrentPassword!","new_password":"NewStrongPassword2026!"}`
	reqWrong := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(wrongCurBody))
	reqWrong.Header.Set("Content-Type", "application/json")
	reqWrong.Header.Set("Authorization", "Bearer "+accessToken)
	wWrong := httptest.NewRecorder()
	router.ServeHTTP(wWrong, reqWrong)

	if wWrong.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for wrong current password, got %d", wWrong.Code)
	}

	// 5. POST /api/v1/auth/change-password with valid passwords succeeds
	validChangeBody := `{"current_password":"InitialPassword123!","new_password":"NewStrongPassword2026!"}`
	reqChange := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(validChangeBody))
	reqChange.Header.Set("Content-Type", "application/json")
	reqChange.Header.Set("Authorization", "Bearer "+accessToken)
	wChange := httptest.NewRecorder()
	router.ServeHTTP(wChange, reqChange)

	if wChange.Code != http.StatusOK {
		t.Fatalf("expected 200 on change-password, got %d: %s", wChange.Code, wChange.Body.String())
	}

	// Verify user requires_password_change is now false and password_hash changed
	usrAfter, _ := userRepo.GetByID(context.Background(), u.ID)
	if usrAfter.RequiresPasswordChange {
		t.Fatal("expected requires_password_change to be false after password change")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(usrAfter.PasswordHash), []byte("NewStrongPassword2026!")); err != nil {
		t.Fatalf("expected new password hash to match NewStrongPassword2026!, got err: %v", err)
	}

	// 6. Old access token used to change password was revoked upon change-password
	reqOldToken := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	reqOldToken.Header.Set("Authorization", "Bearer "+accessToken)
	wOldToken := httptest.NewRecorder()
	router.ServeHTTP(wOldToken, reqOldToken)

	if wOldToken.Code != http.StatusUnauthorized {
		t.Fatalf("expected old access token to be revoked (401), got %d", wOldToken.Code)
	}

	// 7. Login with new password works and returns requires_password_change = false
	newLoginBody := `{"email":"admin@k8sselfhost.local","password":"NewStrongPassword2026!"}`
	reqNewLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(newLoginBody))
	reqNewLogin.Header.Set("Content-Type", "application/json")
	wNewLogin := httptest.NewRecorder()
	router.ServeHTTP(wNewLogin, reqNewLogin)

	if wNewLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 on new login, got %d: %s", wNewLogin.Code, wNewLogin.Body.String())
	}
	var newLoginResp struct {
		Token                  string `json:"token"`
		RequiresPasswordChange bool   `json:"requires_password_change"`
	}
	_ = json.Unmarshal(wNewLogin.Body.Bytes(), &newLoginResp)
	if newLoginResp.RequiresPasswordChange {
		t.Fatal("expected requires_password_change=false on subsequent login")
	}

	// 8. Test Logout revokes access token
	newAccessToken := newLoginResp.Token
	reqLogout := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	reqLogout.Header.Set("Authorization", "Bearer "+newAccessToken)
	wLogout := httptest.NewRecorder()
	router.ServeHTTP(wLogout, reqLogout)

	if wLogout.Code != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", wLogout.Code)
	}

	// After logout, token is immediately revoked (401)
	reqAfterLogout := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	reqAfterLogout.Header.Set("Authorization", "Bearer "+newAccessToken)
	wAfterLogout := httptest.NewRecorder()
	router.ServeHTTP(wAfterLogout, reqAfterLogout)

	if wAfterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", wAfterLogout.Code)
	}
	_ = tokenRepo
}
