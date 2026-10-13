package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domainmesh "github.com/datdt/k8sselfhost/internal/domain/mesh"
	usecasemesh "github.com/datdt/k8sselfhost/internal/usecase/mesh"
)

func setupTestMeshRouter() *chi.Mux {
	r := chi.NewRouter()
	svc := usecasemesh.NewLifecycleService()
	handler := adapthttp.NewMeshHandler(svc)

	r.Route("/api/v1/mesh", handler.RegisterRoutes)
	return r
}

func TestMeshHandler_Endpoints(t *testing.T) {
	router := setupTestMeshRouter()

	t.Run("POST /api/v1/mesh/sidecar/inject-guard - Success", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cart-service",
				Namespace: "staging",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "cart-app", Image: "cart:v1"},
					{Name: "istio-proxy", Image: "istio/proxyv2:1.20"},
				},
			},
		}
		body, _ := json.Marshal(pod)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/sidecar/inject-guard", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "initContainers") {
			t.Fatalf("expected patch to contain initContainers, got: %s", rr.Body.String())
		}
	})

	t.Run("POST /api/v1/mesh/sidecar/inject-guard - Empty Body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/sidecar/inject-guard", bytes.NewReader([]byte{}))
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for empty body, got %d", rr.Code)
		}
		var prob adapthttp.ProblemDetails
		if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
			t.Fatalf("unmarshaling problem details: %v", err)
		}
		if prob.Code != "VALIDATION_ERROR" {
			t.Errorf("expected code VALIDATION_ERROR, got %s", prob.Code)
		}
	})

	t.Run("POST /api/v1/mesh/waf/verify-regex - Approved Linear", func(t *testing.T) {
		payload := map[string]string{
			"pattern": `^[a-zA-Z0-9_\-\.]+@[a-zA-Z0-9\-]+\.[a-zA-Z0-9\-\.]+$`,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/waf/verify-regex", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var report domainmesh.RegexVerificationReport
		if err := json.Unmarshal(rr.Body.Bytes(), &report); err != nil {
			t.Fatalf("unmarshaling report: %v", err)
		}
		if report.Verdict != "APPROVED" {
			t.Errorf("expected APPROVED verdict, got %s", report.Verdict)
		}
		if !report.IsLinearDFA {
			t.Error("expected IsLinearDFA=true")
		}
	})

	t.Run("POST /api/v1/mesh/waf/verify-regex - Rejected Catastrophic ReDoS (RFC 7807)", func(t *testing.T) {
		payload := map[string]string{
			"pattern": `(a+)+$`,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/waf/verify-regex", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status 422 Unprocessable Entity, got %d: %s", rr.Code, rr.Body.String())
		}
		if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
			t.Errorf("expected Content-Type application/problem+json, got %s", ct)
		}

		var prob adapthttp.ProblemDetails
		if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
			t.Fatalf("unmarshaling problem details: %v", err)
		}
		if prob.Code != domainmesh.CodeWAFNonLinearRegex {
			t.Errorf("expected code %s, got %s", domainmesh.CodeWAFNonLinearRegex, prob.Code)
		}
		if prob.Status != http.StatusUnprocessableEntity {
			t.Errorf("expected status 422, got %d", prob.Status)
		}
	})

	t.Run("POST /api/v1/mesh/tls/drain - Success", func(t *testing.T) {
		payload := map[string]interface{}{
			"namespace":              "production",
			"deployment":             "ingress-gateway",
			"drain_duration_seconds": 45,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/tls/drain", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshaling response: %v", err)
		}
		if resp["status"] != "configured" {
			t.Errorf("expected status configured, got %v", resp["status"])
		}
	})

	t.Run("POST /api/v1/mesh/tls/drain - Missing Deployment", func(t *testing.T) {
		payload := map[string]interface{}{
			"namespace": "production",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/mesh/tls/drain", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for missing deployment, got %d", rr.Code)
		}
	})
}
