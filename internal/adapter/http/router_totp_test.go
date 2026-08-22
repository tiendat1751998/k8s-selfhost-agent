package http_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/user"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

func TestRouter_TOTPRoutes(t *testing.T) {
	healthHandler := health.NewHandler(5 * time.Second)

	authHandler, userRepo, _, _ := setupAuthTest(t)
	userRepo.users["u1"] = &user.User{
		ID:           "u1",
		Email:        "totp-test@example.com",
		PlatformRole: "platform_admin",
		TenantID:     "tenant-1",
		TenantRole:   "admin",
		MFAEnabled:   false,
	}

	platform := &adapthttp.PlatformHandlers{
		Auth: authHandler,
	}

	router := adapthttp.NewRouterWithWS(healthHandler, nil, platform)

	token, err := middleware.GenerateJWT("u1", "viewer", "default-tenant")
	if err != nil {
		t.Fatalf("failed to generate JWT token: %v", err)
	}

	totpEndpoints := []struct {
		name         string
		method       string
		path         string
		body         string
		expectedCode int
	}{
		{
			name:         "Setup",
			method:       http.MethodPost,
			path:         "/api/v1/auth/totp/setup",
			body:         "",
			expectedCode: http.StatusOK,
		},
		{
			name:         "Status",
			method:       http.MethodGet,
			path:         "/api/v1/auth/totp/status",
			body:         "",
			expectedCode: http.StatusOK,
		},
	}

	for _, ep := range totpEndpoints {
		t.Run("Unauthenticated "+ep.method+" "+ep.path+" returns 401", func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(ep.body))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("[%s %s] expected 401 Unauthorized, got %d", ep.method, ep.path, rec.Code)
			}
		})

		t.Run("Authenticated "+ep.method+" "+ep.path+" returns "+http.StatusText(ep.expectedCode), func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(ep.body))
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != ep.expectedCode {
				t.Errorf("[%s %s] expected %d %s, got %d: %s", ep.method, ep.path, ep.expectedCode, http.StatusText(ep.expectedCode), rec.Code, rec.Body.String())
			}
		})
	}
}
