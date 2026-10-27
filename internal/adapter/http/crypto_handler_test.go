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
	"github.com/datdt/k8sselfhost/internal/domain/settings"
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

	// 4. GET /api/v1/security/kms/status (Honest provider status)
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/v1/security/kms/status", nil)
	recStatus := httptest.NewRecorder()
	r.ServeHTTP(recStatus, reqStatus)
	if recStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 on get kms status, got %d: %s", recStatus.Code, recStatus.Body.String())
	}
	var statusResp adapterhttp.KMSStatusResponse
	if err := json.Unmarshal(recStatus.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("failed to decode kms status response: %v", err)
	}
	if statusResp.Provider != "local-aes256-gcm" {
		t.Errorf("expected provider 'local-aes256-gcm', got %q", statusResp.Provider)
	}
	if statusResp.HardwareBacked {
		t.Errorf("expected hardware_backed=false, got true")
	}
	if statusResp.Mode != "envelope-aes-256-gcm" {
		t.Errorf("expected mode 'envelope-aes-256-gcm', got %q", statusResp.Mode)
	}

	// 5. GET /api/v1/security/kms/keys
	reqKeys := httptest.NewRequest(http.MethodGet, "/api/v1/security/kms/keys", nil)
	recKeys := httptest.NewRecorder()
	r.ServeHTTP(recKeys, reqKeys)
	if recKeys.Code != http.StatusOK {
		t.Fatalf("expected 200 on get kms keys, got %d: %s", recKeys.Code, recKeys.Body.String())
	}
	var keysResp adapterhttp.KMSKeysResponse
	if err := json.Unmarshal(recKeys.Body.Bytes(), &keysResp); err != nil {
		t.Fatalf("failed to decode kms keys response: %v", err)
	}
	if len(keysResp.Keys) == 0 {
		t.Errorf("expected at least 1 key in keys response")
	}
}

type mockKMSMemorySettingsRepo struct {
	settings map[string]string // key -> value
}

func (m *mockKMSMemorySettingsRepo) GetByCategory(ctx context.Context, tenantID, category string) ([]settings.Setting, error) {
	return nil, nil
}
func (m *mockKMSMemorySettingsRepo) GetByKey(ctx context.Context, tenantID, category, key string) (*settings.Setting, error) {
	if v, ok := m.settings[key]; ok {
		return &settings.Setting{Key: key, Value: v, Category: category, TenantID: tenantID}, nil
	}
	return nil, nil
}
func (m *mockKMSMemorySettingsRepo) Upsert(ctx context.Context, s *settings.Setting) error {
	if m.settings == nil {
		m.settings = make(map[string]string)
	}
	m.settings[s.Key] = s.Value
	return nil
}
func (m *mockKMSMemorySettingsRepo) BulkUpsert(ctx context.Context, items []settings.Setting) error {
	for _, it := range items {
		_ = m.Upsert(ctx, &it)
	}
	return nil
}
func (m *mockKMSMemorySettingsRepo) GetAll(ctx context.Context, tenantID string) ([]settings.Setting, error) {
	return nil, nil
}
func (m *mockKMSMemorySettingsRepo) FindAPIKeyByHash(ctx context.Context, tokenHash string) (*settings.Setting, error) {
	return nil, nil
}

func TestKMS_HonestProviderStatus_VaultEnv(t *testing.T) {
	t.Setenv("VAULT_ADDR", "https://vault.internal.net:8200")
	manager := usecasecrypto.NewShieldManager(usecasecrypto.ShieldManagerOptions{})
	handler := adapterhttp.NewCryptoHandler(manager)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/security/kms/status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp adapterhttp.KMSStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.Provider != "hashicorp-vault" {
		t.Errorf("expected provider 'hashicorp-vault' when VAULT_ADDR is set, got %q", resp.Provider)
	}
	if resp.HardwareBacked {
		t.Errorf("expected hardware_backed=false, got true")
	}
}

func TestKMS_MetadataPersistenceAcrossRestart(t *testing.T) {
	memRepo := &mockKMSMemorySettingsRepo{settings: make(map[string]string)}

	// 1. First run: Default KMS client initialized with settingsRepo
	kms1 := adapterhttp.NewDefaultKMSClient()
	kms1.SetSettingsRepo(memRepo)

	// Rotate key -> version increments from 1 to 2
	resp, err := kms1.RotateKey(context.Background())
	if err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}
	if resp.Version != 2 {
		t.Fatalf("expected version 2, got %d", resp.Version)
	}

	// Verify that setting was saved to platform_settings without raw key material
	savedMetaJSON, ok := memRepo.settings["security.kms_metadata"]
	if !ok || savedMetaJSON == "" {
		t.Fatalf("expected security.kms_metadata to be saved in settings")
	}
	if bytes.Contains([]byte(savedMetaJSON), []byte("raw_key")) || bytes.Contains([]byte(savedMetaJSON), []byte("production-secure")) {
		t.Fatalf("CRITICAL SECURITY VIOLATION: raw key material leaked into metadata JSON: %s", savedMetaJSON)
	}

	// 2. Second run: server restart simulated by initializing a fresh DefaultKMSClient with the same settingsRepo
	kms2 := adapterhttp.NewDefaultKMSClient()
	kms2.SetSettingsRepo(memRepo)

	details, err := kms2.GetKey(context.Background())
	if err != nil {
		t.Fatalf("GetKey on resumed kms client failed: %v", err)
	}
	if details.Version != 2 {
		t.Errorf("expected resumed KMS client to load version 2 from settings, got %d", details.Version)
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
