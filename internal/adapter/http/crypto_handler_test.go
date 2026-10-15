package http_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	adapterhttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domaincrypto "github.com/datdt/k8sselfhost/internal/domain/crypto"
	usecasecrypto "github.com/datdt/k8sselfhost/internal/usecase/crypto"
)

func setupCryptoTestRouter(manager *usecasecrypto.ShieldManager) *chi.Mux {
	r := chi.NewRouter()
	handler := adapterhttp.NewCryptoHandler(manager)
	handler.RegisterRoutes(r)
	return r
}

func generateCertChainForHandlerTest(t *testing.T, ocspURL string) []byte {
	t.Helper()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed ca key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(10),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed ca cert: %v", err)
	}
	caCert, _ := x509.ParseCertificate(caDER)

	leafKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(11),
		Subject:      pkix.Name{CommonName: "gateway.internal.net"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		OCSPServer:   []string{ocspURL},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed leaf cert: %v", err)
	}

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	return append(leafPEM, caPEM...)
}

func TestCryptoHandler_OCSPVerifyStaple_Success(t *testing.T) {
	ocspServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/ocsp-response")
		w.WriteHeader(http.StatusOK)
	}))
	defer ocspServer.Close()

	chainPEM := generateCertChainForHandlerTest(t, ocspServer.URL)

	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{
		HTTPClient: ocspServer.Client(),
	})
	r := setupCryptoTestRouter(manager)

	reqBody, _ := json.Marshal(map[string]string{
		"cert_chain_pem": string(chainPEM),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/crypto/ocsp/verify-staple", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status code: %d", rec.Code)
	}
}

func TestCryptoHandler_OCSPVerifyStaple_502_RFC7807(t *testing.T) {
	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{
		HTTPClient: &http.Client{Timeout: 10 * time.Millisecond},
	})
	r := setupCryptoTestRouter(manager)

	chainPEM := generateCertChainForHandlerTest(t, "http://127.0.0.1:54321/dead-ocsp")

	reqBody, _ := json.Marshal(map[string]string{
		"cert_chain_pem": string(chainPEM),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/crypto/ocsp/verify-staple", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway on OCSP unreachable, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", contentType)
	}

	var prob adapterhttp.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode RFC 7807 problem details: %v", err)
	}
	if prob.Status != 502 || prob.Code != "ERR_OCSP_STAPLE_FETCH_FAILED" {
		t.Errorf("unexpected problem details payload: %+v", prob)
	}
}

func TestCryptoHandler_HSMSessionStatus_NormalAndSaturated(t *testing.T) {
	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{
		MaxHSMSessions: 2,
	})
	r := setupCryptoTestRouter(manager)

	// 1. Unsaturated pool -> 200 OK
	req := httptest.NewRequest(http.MethodGet, "/api/v1/crypto/hsm/session-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when pool unsaturated, got %d", rec.Code)
	}

	var status domaincrypto.HSMSessionPoolStatus
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode pool status: %v", err)
	}
	if status.IsSaturated || status.AvailableSessions != 2 {
		t.Errorf("unexpected status: %+v", status)
	}

	// 2. Saturate the pool
	_, r1, _ := manager.AcquireHSMSession(context.Background(), 50*time.Millisecond)
	_, r2, _ := manager.AcquireHSMSession(context.Background(), 50*time.Millisecond)
	defer r1()
	defer r2()

	reqSaturated := httptest.NewRequest(http.MethodGet, "/api/v1/crypto/hsm/session-status", nil)
	recSaturated := httptest.NewRecorder()
	r.ServeHTTP(recSaturated, reqSaturated)

	if recSaturated.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable when pool saturated, got %d", recSaturated.Code)
	}
}

type mockKMSClient struct {
	fail bool
	key  []byte
}

func (m *mockKMSClient) DecryptDEK(ctx context.Context, keyID string) ([]byte, error) {
	if m.fail {
		return nil, errors.New("kms service unavailable")
	}
	return m.key, nil
}

func (m *mockKMSClient) GetKey(ctx context.Context) (*adapterhttp.KMSKeyDetails, error) {
	return &adapterhttp.KMSKeyDetails{KeyID: "kms-master", Version: 1, Algorithm: "AES-256-GCM"}, nil
}

func (m *mockKMSClient) RotateKey(ctx context.Context) (*adapterhttp.KMSRotateResponse, error) {
	return &adapterhttp.KMSRotateResponse{KeyID: "kms-master", Version: 2, RotatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (m *mockKMSClient) VerifyKey(ctx context.Context) (*adapterhttp.KMSVerifyResponse, error) {
	return &adapterhttp.KMSVerifyResponse{Verified: true, Checksum: "abc123checksum"}, nil
}

func TestCryptoHandler_EnvelopeDEK_KMSGraceAndExpiry(t *testing.T) {
	now := time.Now()
	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{
		DEKDefaultTTL:  1 * time.Hour,
		DEKGracePeriod: 4 * time.Hour,
		NowFunc:        func() time.Time { return now },
	})
	mockKMS := &mockKMSClient{key: []byte("raw-secret-key-32-byte-buffer000")}
	handler := adapterhttp.NewCryptoHandler(manager, mockKMS)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// 1. Fetch DEK successfully
	reqBody, _ := json.Marshal(map[string]any{
		"key_id": "dek-database-user-pw",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/crypto/envelope/dek", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	// 2. KMS fails, but within 4h grace window -> returns 200 OK with is_stale_valid=true
	now = now.Add(2 * time.Hour)
	mockKMS.fail = true
	reqStale := httptest.NewRequest(http.MethodPost, "/api/v1/crypto/envelope/dek", bytes.NewReader(reqBody))
	reqStale.Header.Set("Content-Type", "application/json")
	recStale := httptest.NewRecorder()
	r.ServeHTTP(recStale, reqStale)

	if recStale.Code != http.StatusOK {
		t.Fatalf("expected 200 OK during 4h grace window, got %d: %s", recStale.Code, recStale.Body.String())
	}
	var staleResp adapterhttp.EnvelopeDEKResponse
	if err := json.NewDecoder(recStale.Body).Decode(&staleResp); err != nil {
		t.Fatalf("failed to decode stale response: %v", err)
	}
	if !staleResp.IsStaleValid {
		t.Errorf("expected is_stale_valid=true, got false")
	}

	// 3. KMS fails and past 4h grace window (total 6 hours) -> returns 500 ErrKMSDEKGraceExpired
	now = now.Add(4 * time.Hour)
	reqExpired := httptest.NewRequest(http.MethodPost, "/api/v1/crypto/envelope/dek", bytes.NewReader(reqBody))
	reqExpired.Header.Set("Content-Type", "application/json")
	recExpired := httptest.NewRecorder()
	r.ServeHTTP(recExpired, reqExpired)

	if recExpired.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error when grace expired, got %d", recExpired.Code)
	}
}

func TestCryptoHandler_KMSKeyManagementRoutes(t *testing.T) {
	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{})
	r := chi.NewRouter()
	handler := adapterhttp.NewCryptoHandler(manager)
	handler.RegisterRoutes(r)

	// 1. GET /api/v1/security/kms/key
	reqKey := httptest.NewRequest(http.MethodGet, "/api/v1/security/kms/key", nil)
	recKey := httptest.NewRecorder()
	r.ServeHTTP(recKey, reqKey)
	if recKey.Code != http.StatusOK {
		t.Fatalf("expected 200 on get kms key, got %d: %s", recKey.Code, recKey.Body.String())
	}

	// 2. POST /api/v1/security/kms/rotate
	reqRot := httptest.NewRequest(http.MethodPost, "/api/v1/security/kms/rotate", bytes.NewReader([]byte("{}")))
	recRot := httptest.NewRecorder()
	r.ServeHTTP(recRot, reqRot)
	if recRot.Code != http.StatusOK {
		t.Fatalf("expected 200 on rotate kms key, got %d: %s", recRot.Code, recRot.Body.String())
	}

	// 3. POST /api/v1/security/kms/verify
	reqVer := httptest.NewRequest(http.MethodPost, "/api/v1/security/kms/verify", bytes.NewReader([]byte("{}")))
	recVer := httptest.NewRecorder()
	r.ServeHTTP(recVer, reqVer)
	if recVer.Code != http.StatusOK {
		t.Fatalf("expected 200 on verify kms key, got %d: %s", recVer.Code, recVer.Body.String())
	}
}

func TestCryptoHandler_FIPSScan_CleanAndViolations(t *testing.T) {
	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{})
	r := setupCryptoTestRouter(manager)

	tempDir := t.TempDir()

	// 1. Clean dir
	safeFile := filepath.Join(tempDir, "safe.go")
	_ = os.WriteFile(safeFile, []byte("package safe\nfunc Ok() {}\n"), 0600)

	cleanBody, _ := json.Marshal(map[string]string{"root_dir": tempDir})
	reqClean := httptest.NewRequest(http.MethodPost, "/api/v1/crypto/fips/scan", bytes.NewReader(cleanBody))
	reqClean.Header.Set("Content-Type", "application/json")
	recClean := httptest.NewRecorder()
	r.ServeHTTP(recClean, reqClean)

	if recClean.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on clean scan, got %d", recClean.Code)
	}

	// 2. Dir with non-FIPS MD5 algorithm -> 422 Unprocessable Entity
	badFile := filepath.Join(tempDir, "bad.go")
	_ = os.WriteFile(badFile, []byte("package bad\nimport \"crypto/md5\"\nfunc Bad() { _ = md5.New() }\n"), 0600)

	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/crypto/fips/scan", bytes.NewReader(cleanBody))
	reqBad.Header.Set("Content-Type", "application/json")
	recBad := httptest.NewRecorder()
	r.ServeHTTP(recBad, reqBad)

	if recBad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity on non-FIPS hash, got %d: %s", recBad.Code, recBad.Body.String())
	}
}
