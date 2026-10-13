package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	domain "github.com/datdt/k8sselfhost/internal/domain/secdefense"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/secdefense"
)

func setupTestSecDefenseRouter() (*chi.Mux, *SecDefenseHandler) {
	mgr := usecase.NewDefenseManager(nil)
	handler := NewSecDefenseHandler(mgr)

	r := chi.NewRouter()
	handler.Mount(r)

	return r, handler
}

func TestSSRFValidateURL_MetadataForbidden_IPv4(t *testing.T) {
	r, _ := setupTestSecDefenseRouter()

	reqBody := `{"url": "http://169.254.169.254/latest/meta-data/"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/ssrf/validate-url", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden, got %d: %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %s", contentType)
	}

	var prob ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem details JSON: %v", err)
	}
	if prob.Code != "ERR_SEC_SSRF_METADATA_ATTEMPT" {
		t.Errorf("expected code ERR_SEC_SSRF_METADATA_ATTEMPT, got %s", prob.Code)
	}
}

func TestSSRFValidateURL_MetadataForbidden_IPv6(t *testing.T) {
	r, _ := setupTestSecDefenseRouter()

	reqBody := `{"url": "http://[fd00:ec2::254]/latest/meta-data/"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/ssrf/validate-url", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden for IPv6 metadata, got %d: %s", rec.Code, rec.Body.String())
	}

	var prob ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to parse problem details: %v", err)
	}
	if prob.Code != "ERR_SEC_SSRF_METADATA_ATTEMPT" {
		t.Errorf("expected code ERR_SEC_SSRF_METADATA_ATTEMPT, got %s", prob.Code)
	}
}

func TestSSRFValidateURL_PublicAllowed(t *testing.T) {
	r, _ := setupTestSecDefenseRouter()

	reqBody := `{"url": "https://1.1.1.1/dns-query"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/ssrf/validate-url", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res domain.SSRFValidationResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}
	if res.IsBlocked {
		t.Errorf("expected URL not to be blocked, got reason: %s", res.BlockReason)
	}
}

func TestSSRFValidateURL_EmptyURL(t *testing.T) {
	r, _ := setupTestSecDefenseRouter()

	reqBody := `{"url": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/ssrf/validate-url", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 Bad Request, got %d", rec.Code)
	}
}

func TestAuditDependencies_ConfusionDetected(t *testing.T) {
	r, _ := setupTestSecDefenseRouter()

	tempDir := t.TempDir()
	lockfilePath := filepath.Join(tempDir, "package-lock.json")
	maliciousLockfile := `{
  "name": "enterprise-app",
  "version": "1.0.0",
  "lockfileVersion": 2,
  "packages": {
    "node_modules/@internal/crypto-vault": {
      "version": "99.0.0",
      "resolved": "https://registry.npmjs.org/@internal/crypto-vault/-/crypto-vault-99.0.0.tgz"
    }
  }
}`
	if err := os.WriteFile(lockfilePath, []byte(maliciousLockfile), 0600); err != nil {
		t.Fatalf("failed to write temp lockfile: %v", err)
	}

	reqPayload := map[string]interface{}{
		"lockfile_path":    lockfilePath,
		"private_prefixes": []string{"@internal/"},
	}
	body, _ := json.Marshal(reqPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/supplychain/audit-deps", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected HTTP 422 Unprocessable Entity, got %d: %s", rec.Code, rec.Body.String())
	}

	var prob ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}
	if prob.Code != "ERR_SEC_DEPENDENCY_CONFUSION" {
		t.Errorf("expected ERR_SEC_DEPENDENCY_CONFUSION, got %s", prob.Code)
	}
}

func TestAuditDependencies_Clean(t *testing.T) {
	r, _ := setupTestSecDefenseRouter()

	tempDir := t.TempDir()
	lockfilePath := filepath.Join(tempDir, "package-lock.json")
	cleanLockfile := `{
  "name": "enterprise-app",
  "version": "1.0.0",
  "lockfileVersion": 2,
  "packages": {
    "node_modules/@internal/crypto-vault": {
      "version": "1.2.0",
      "resolved": "https://artifactory.corp.internal/@internal/crypto-vault/-/crypto-vault-1.2.0.tgz"
    }
  }
}`
	if err := os.WriteFile(lockfilePath, []byte(cleanLockfile), 0600); err != nil {
		t.Fatalf("failed to write temp lockfile: %v", err)
	}

	reqPayload := map[string]interface{}{
		"lockfile_path":    lockfilePath,
		"private_prefixes": []string{"@internal/"},
	}
	body, _ := json.Marshal(reqPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/supplychain/audit-deps", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var report domain.DependencyAuditReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("failed to unmarshal report: %v", err)
	}
	if !report.IsClean {
		t.Errorf("expected report to be clean")
	}
}

func TestValidateSidecarOrder_InsecureOrder(t *testing.T) {
	r, _ := setupTestSecDefenseRouter()

	insecureYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: payment-service
spec:
  containers:
    - name: payment-app
      image: app:1.0.0
    - name: envoy-proxy
      image: envoyproxy/envoy:v1.28.0
`
	reqPayload := map[string]string{
		"pod_spec_yaml": insecureYAML,
	}
	body, _ := json.Marshal(reqPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/sidecar/validate-order", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected HTTP 422 Unprocessable Entity, got %d: %s", rec.Code, rec.Body.String())
	}

	var prob ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to unmarshal problem details: %v", err)
	}
	if prob.Code != "ERR_INVALID_SIDECAR_ORDER" {
		t.Errorf("expected ERR_INVALID_SIDECAR_ORDER, got %s", prob.Code)
	}
}

func TestValidateSidecarOrder_CompliantNativeSidecar(t *testing.T) {
	r, _ := setupTestSecDefenseRouter()

	compliantYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: payment-service
spec:
  initContainers:
    - name: envoy-proxy
      image: envoyproxy/envoy:v1.28.0
      restartPolicy: Always
  containers:
    - name: payment-app
      image: app:1.0.0
`
	reqPayload := map[string]string{
		"pod_spec_yaml": compliantYAML,
	}
	body, _ := json.Marshal(reqPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/sidecar/validate-order", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnforceIMDSv2_SuccessAndFailure(t *testing.T) {
	r, _ := setupTestSecDefenseRouter()

	// Failure case: missing instance_id
	badReqBody := `{"instance_id": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/imdsv2/enforce", bytes.NewBufferString(badReqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 Bad Request, got %d", rec.Code)
	}

	// Success case: valid instance_id
	goodReqBody := `{"instance_id": "i-0987654321fedcba0", "hop_limit": 1, "require_tokens": true}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/security/imdsv2/enforce", bytes.NewBufferString(goodReqBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()

	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK, got %d: %s", rec2.Code, rec2.Body.String())
	}
}
