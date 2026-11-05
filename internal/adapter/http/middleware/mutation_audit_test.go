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
	readOnlyPosts := []string{
		"/api/v1/security/ssrf/validate-url",
		"/api/v1/system/test",
		"/api/v1/crypto/verify",
		"/api/v1/discover",
		"/api/v1/render",
		"/api/v1/cluster-search",
		"/api/v1/logs/search",
		"/api/v1/check-health-fallback",
		"/api/v1/audit-deps",
		"/api/v1/pod-preflight",
		"/api/v1/pfc-deadlock-check",
		"/api/v1/nccl-progress",
		"/api/v1/kv-cache-check",
		"/api/v1/resolve-digest",
		"/api/v1/fips/scan",
	}

	for _, p := range readOnlyPosts {
		if !IsReadOnlyPostPath(http.MethodPost, p) {
			t.Errorf("expected POST %q to be recognized as read-only POST path", p)
		}
	}

	// Methods other than POST MUST NEVER be recognized as read-only
	if IsReadOnlyPostPath(http.MethodDelete, "/api/v1/deployments/test") {
		t.Error("expected DELETE to NOT be recognized as read-only")
	}
	if IsReadOnlyPostPath(http.MethodPut, "/api/v1/organizations/test/quota") {
		t.Error("expected PUT to NOT be recognized as read-only")
	}
	if IsReadOnlyPostPath(http.MethodPatch, "/api/v1/nodes/worker/test") {
		t.Error("expected PATCH to NOT be recognized as read-only")
	}

	// Suffix that doesn't match action list (e.g. /test inside a path segment)
	if IsReadOnlyPostPath(http.MethodPost, "/api/v1/namespaces/test/deployments") {
		t.Error("expected POST with 'test' in path segment to NOT be recognized as read-only")
	}
}

func TestMutationAudit_NeverExemptsNonPostOrParamSuffix(t *testing.T) {
	recorder := &mockAuditRecorder{}
	mw := MutationAudit(recorder)

	// DELETE /api/v1/deployments/test
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/deployments/test", nil)
	recDel := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(recDel, reqDel)
	if !recorder.called {
		t.Error("expected DELETE /api/v1/deployments/test to be audited")
	}

	// PUT /api/v1/organizations/test/quota
	recorder.called = false
	reqPut := httptest.NewRequest(http.MethodPut, "/api/v1/organizations/test/quota", nil)
	recPut := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(recPut, reqPut)
	if !recorder.called {
		t.Error("expected PUT /api/v1/organizations/test/quota to be audited")
	}

	// POST /api/v1/namespaces/test/deployments
	recorder.called = false
	reqPost := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/test/deployments", nil)
	recPost := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(recPost, reqPost)
	if !recorder.called {
		t.Error("expected POST /api/v1/namespaces/test/deployments to be audited")
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
