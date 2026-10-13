package http_test

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	adapterhttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domainauth "github.com/datdt/k8sselfhost/internal/domain/auth"
)

type stubJWKSShield struct {
	status     domainauth.JWKSCacheStatus
	refreshErr error
}

func (s *stubJWKSShield) GetValidationKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	return nil, nil
}

func (s *stubJWKSShield) RefreshKeys(ctx context.Context) error {
	return s.refreshErr
}

func (s *stubJWKSShield) GetCacheStatus() domainauth.JWKSCacheStatus {
	return s.status
}

type stubVaultProvider struct {
	status domainauth.VaultUnsealStatus
	err    error
}

func (p *stubVaultProvider) GetVaultUnsealStatus(ctx context.Context) (domainauth.VaultUnsealStatus, error) {
	if p.err != nil {
		return domainauth.VaultUnsealStatus{}, p.err
	}
	return p.status, nil
}

func setupJWKSTestRouter(handler *adapterhttp.JWKSHandler) *chi.Mux {
	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r
}

func TestJWKSHandler_GetJWKSStatus(t *testing.T) {
	stub := &stubJWKSShield{
		status: domainauth.JWKSCacheStatus{
			TotalKeys:        3,
			IsUsingStaleKey:  true,
			StaleKeyAge:      15 * time.Minute,
			StaleKeyAgeHuman: "15m0s",
			ProviderURL:      "https://idp.example.com/.well-known/jwks.json",
			Hits:             150,
			Misses:           2,
			StaleHits:        25,
		},
	}

	handler := adapterhttp.NewJWKSHandler(stub)
	router := setupJWKSTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/jwks/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domainauth.JWKSCacheStatus
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if resp.TotalKeys != 3 {
		t.Errorf("expected TotalKeys = 3, got %d", resp.TotalKeys)
	}
	if !resp.IsUsingStaleKey {
		t.Errorf("expected IsUsingStaleKey = true")
	}
	if resp.ProviderURL != "https://idp.example.com/.well-known/jwks.json" {
		t.Errorf("unexpected provider URL: %s", resp.ProviderURL)
	}
}

func TestJWKSHandler_RefreshJWKS_SuccessAndFailure(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		stub := &stubJWKSShield{
			status: domainauth.JWKSCacheStatus{
				TotalKeys:   4,
				ProviderURL: "https://idp.example.com/.well-known/jwks.json",
			},
		}

		handler := adapterhttp.NewJWKSHandler(stub)
		router := setupJWKSTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/jwks/refresh", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200 on refresh, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Failure", func(t *testing.T) {
		stub := &stubJWKSShield{
			refreshErr: errors.New("IdP connection timeout"),
		}

		handler := adapterhttp.NewJWKSHandler(stub)
		router := setupJWKSTestRouter(handler)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/jwks/refresh", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected HTTP 502 on refresh failure, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestJWKSHandler_GetVaultUnsealStatus(t *testing.T) {
	t.Run("Unsealed status", func(t *testing.T) {
		vaultStub := &stubVaultProvider{
			status: domainauth.VaultUnsealStatus{
				IsSealed:       false,
				Threshold:      3,
				Progress:       0,
				AutoUnsealMode: "tpm2",
				LastUnsealedAt: time.Now(),
			},
		}

		handler := adapterhttp.NewJWKSHandler(
			&stubJWKSShield{},
			adapterhttp.WithVaultStatusProvider(vaultStub),
		)
		router := setupJWKSTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/vault/unseal-status", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var res domainauth.VaultUnsealStatus
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if res.IsSealed {
			t.Errorf("expected is_sealed to be false")
		}
		if res.AutoUnsealMode != "tpm2" {
			t.Errorf("expected auto_unseal_mode 'tpm2', got %s", res.AutoUnsealMode)
		}
	})

	t.Run("Provider error", func(t *testing.T) {
		vaultStub := &stubVaultProvider{
			err: errors.New("vault API unreachable"),
		}

		handler := adapterhttp.NewJWKSHandler(
			&stubJWKSShield{},
			adapterhttp.WithVaultStatusProvider(vaultStub),
		)
		router := setupJWKSTestRouter(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/vault/unseal-status", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 5xx status on vault provider error, got %d", rec.Code)
		}
	})
}
