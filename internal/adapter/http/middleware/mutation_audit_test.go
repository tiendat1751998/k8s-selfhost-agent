package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockAuditRecorder struct {
	called     bool
	actor      string
	action     string
	targetType string
	targetID   string
	targetName string
	result     string
	details    map[string]interface{}
	ipAddress  string
	userAgent  string
}

func (m *mockAuditRecorder) RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error {
	m.called = true
	m.actor = actor
	m.action = action
	m.targetType = targetType
	m.targetID = targetID
	m.targetName = targetName
	m.result = result
	m.details = details
	m.ipAddress = ipAddress
	m.userAgent = userAgent
	return nil
}

func TestIsReadOnlyPostPath(t *testing.T) {
	readOnlyPaths := []string{
		"/api/v1/security/ssrf/validate-url",
		"/api/v1/system/test",
		"/api/v1/verify",
		"/api/v1/discover",
		"/api/v1/render",
		"/api/v1/cluster-search",
		"/api/v1/search",
		"/api/v1/check-health",
		"/api/v1/audit-check",
		"/api/v1/pod-preflight",
		"/api/v1/pfc-deadlock-check",
		"/api/v1/nccl-progress",
		"/api/v1/kv-cache-check",
		"/api/v1/resolve-digest",
		"/api/v1/fips/scan",
	}

	for _, p := range readOnlyPaths {
		if !IsReadOnlyPostPath(p) {
			t.Errorf("expected %q to be recognized as read-only POST path", p)
		}
	}

	mutatingPaths := []string{
		"/api/v1/deployments",
		"/api/v1/nodes/worker-1/drain",
		"/api/v1/organizations/org-1/quota",
	}

	for _, p := range mutatingPaths {
		if IsReadOnlyPostPath(p) {
			t.Errorf("expected %q to NOT be recognized as read-only POST path", p)
		}
	}
}

func TestMutationAudit_RecordsMutation(t *testing.T) {
	recorder := &mockAuditRecorder{}
	mw := MutationAudit(recorder)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/restart", nil)
	req = setTestIdentity(req, "admin-user", "tenant-alpha")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !recorder.called {
		t.Fatal("expected mutation audit to record action")
	}
	if recorder.actor != "admin-user" {
		t.Errorf("expected actor 'admin-user', got %q", recorder.actor)
	}
	if recorder.result != "success" {
		t.Errorf("expected result 'success', got %q", recorder.result)
	}
}

func TestMutationAudit_SkipsGETAndReadOnlyPOST(t *testing.T) {
	recorder := &mockAuditRecorder{}
	mw := MutationAudit(recorder)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// GET request
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/deployments", nil)
	recGet := httptest.NewRecorder()
	handler.ServeHTTP(recGet, reqGet)
	if recorder.called {
		t.Fatal("expected GET to be skipped from mutation audit")
	}

	// Read-only POST request
	reqPost := httptest.NewRequest(http.MethodPost, "/api/v1/cluster-search", nil)
	recPost := httptest.NewRecorder()
	handler.ServeHTTP(recPost, reqPost)
	if recorder.called {
		t.Fatal("expected read-only POST to be skipped from mutation audit")
	}
}

func TestMutationAudit_SkipsWhenAlreadyRecorded(t *testing.T) {
	recorder := &mockAuditRecorder{}
	mw := MutationAudit(recorder)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handler marks audit recorded explicitly
		tenancy.MarkAuditRecorded(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/scale", nil)
	req = setTestIdentity(req, "admin-user", "tenant-alpha")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if recorder.called {
		t.Fatal("expected mutation audit to skip recording when tenancy.IsAuditRecorded is true")
	}
}
