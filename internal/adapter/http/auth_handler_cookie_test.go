package http_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/datdt/k8sselfhost/internal/domain/user"
)

func TestLogin_CookieSecureOverride(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "false")
	handler, userRepo, _, _ := setupAuthTest(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("secretpass123"), bcrypt.DefaultCost)
	userRepo.users["u1"] = &user.User{
		ID:           "u1",
		Email:        "user@example.com",
		PasswordHash: string(hash),
		PlatformRole: "operator",
		TenantID:     "tenant-1",
		TenantRole:   "admin",
	}

	body := `{"email":"user@example.com","password":"secretpass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatal("expected refresh_token cookie")
	}
	if refreshCookie.Secure {
		t.Fatal("expected refresh_token cookie to have Secure=false when COOKIE_SECURE=false")
	}
}
