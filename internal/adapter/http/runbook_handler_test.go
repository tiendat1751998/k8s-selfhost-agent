package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/domain/runbook"
	pkgErrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type mockRunbookRepo struct {
	items     map[string]runbook.Runbook
	listErr   error
	getErr    error
	createErr error
	updateErr error
	deleteErr error
}

func newMockRunbookRepo() *mockRunbookRepo {
	return &mockRunbookRepo{
		items: make(map[string]runbook.Runbook),
	}
}

func (m *mockRunbookRepo) List(ctx context.Context, category string, limit, offset int) ([]runbook.Runbook, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	var res []runbook.Runbook
	for _, item := range m.items {
		if category == "" || item.Category == category {
			res = append(res, item)
		}
	}
	return res, len(res), nil
}

func (m *mockRunbookRepo) GetByID(ctx context.Context, id string) (*runbook.Runbook, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	item, ok := m.items[id]
	if !ok {
		return nil, pkgErrors.ErrNotFound
	}
	return &item, nil
}

func (m *mockRunbookRepo) Create(ctx context.Context, r *runbook.Runbook) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.items[r.ID] = *r
	return nil
}

func (m *mockRunbookRepo) Update(ctx context.Context, r *runbook.Runbook) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.items[r.ID] = *r
	return nil
}

func (m *mockRunbookRepo) Delete(ctx context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.items, id)
	return nil
}

type mockAuditRepo struct {
	recordedActions []map[string]interface{}
	recordActionErr error
}

func (m *mockAuditRepo) ListFindings(ctx context.Context, status string) ([]audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockAuditRepo) GetFinding(ctx context.Context, id string) (*audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockAuditRepo) ResolveFinding(ctx context.Context, id string) error {
	return nil
}
func (m *mockAuditRepo) RecordRun(ctx context.Context, run *audit.AuditRun) error {
	return nil
}
func (m *mockAuditRepo) GetLastRun(ctx context.Context) (*audit.AuditRun, error) {
	return nil, nil
}
func (m *mockAuditRepo) ListLogs(ctx context.Context, filter audit.AuditLogFilter) ([]audit.AuditLog, int, error) {
	return nil, 0, nil
}
func (m *mockAuditRepo) RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error {
	if m.recordActionErr != nil {
		return m.recordActionErr
	}
	m.recordedActions = append(m.recordedActions, map[string]interface{}{
		"actor":      actor,
		"action":     action,
		"targetType": targetType,
		"targetID":   targetID,
		"targetName": targetName,
		"result":     result,
		"details":    details,
	})
	return nil
}

func setupRunbookRouter(repo runbook.Repository, auditRepo audit.Repository) (*RunbookHandler, *chi.Mux) {
	handler := NewRunbookHandler(repo, auditRepo)
	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return handler, r
}

func TestRunbookHandler_List(t *testing.T) {
	repo := newMockRunbookRepo()
	auditRepo := &mockAuditRepo{}
	_, r := setupRunbookRouter(repo, auditRepo)

	repo.items["rb-1"] = runbook.Runbook{ID: "rb-1", Title: "Restart Pod", Category: "k8s"}
	repo.items["rb-2"] = runbook.Runbook{ID: "rb-2", Title: "Clean Logs", Category: "system"}

	req := httptest.NewRequest(http.MethodGet, "/?category=k8s", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), resp["total"])

	// Test repo failure
	repo.listErr = errors.New("db error")
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRunbookHandler_GetByID(t *testing.T) {
	repo := newMockRunbookRepo()
	auditRepo := &mockAuditRepo{}
	_, r := setupRunbookRouter(repo, auditRepo)

	validID := "11111111-1111-1111-1111-111111111111"
	repo.items[validID] = runbook.Runbook{ID: validID, Title: "Restart Pod", Category: "k8s"}

	t.Run("invalid UUID", func(t *testing.T) {
		tooLongID := bytes.Repeat([]byte("a"), 129)
		req := httptest.NewRequest(http.MethodGet, "/"+string(tooLongID), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/22222222-2222-2222-2222-222222222222", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/"+validID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestRunbookHandler_Create(t *testing.T) {
	repo := newMockRunbookRepo()
	auditRepo := &mockAuditRepo{}
	_, r := setupRunbookRouter(repo, auditRepo)

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("{invalid")))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("validation failed", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"title": ""})
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"title": "Test RB", "category": "infra"})
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
		assert.NotEmpty(t, auditRepo.recordedActions)
	})
}

func TestRunbookHandler_Update(t *testing.T) {
	repo := newMockRunbookRepo()
	auditRepo := &mockAuditRepo{}
	_, r := setupRunbookRouter(repo, auditRepo)

	validID := "11111111-1111-1111-1111-111111111111"

	body, _ := json.Marshal(map[string]string{"title": "Updated Title", "category": "infra"})
	req := httptest.NewRequest(http.MethodPut, "/"+validID, bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRunbookHandler_Delete(t *testing.T) {
	repo := newMockRunbookRepo()
	auditRepo := &mockAuditRepo{}
	_, r := setupRunbookRouter(repo, auditRepo)

	validID := "11111111-1111-1111-1111-111111111111"

	req := httptest.NewRequest(http.MethodDelete, "/"+validID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRunbookHandler_Execute(t *testing.T) {
	repo := newMockRunbookRepo()
	auditRepo := &mockAuditRepo{}
	_, r := setupRunbookRouter(repo, auditRepo)

	validID := "11111111-1111-1111-1111-111111111111"
	repo.items[validID] = runbook.Runbook{
		ID:         validID,
		Title:      "Restart Deployment",
		Category:   "k8s",
		StepsCount: 3,
	}

	t.Run("invalid uuid", func(t *testing.T) {
		tooLongID := bytes.Repeat([]byte("a"), 129)
		req := httptest.NewRequest(http.MethodPost, "/"+string(tooLongID)+"/execute", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/22222222-2222-2222-2222-222222222222/execute", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/"+validID+"/execute", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "executed", resp["status"])

		// Check repo was updated with lastUsedAt
		updated := repo.items[validID]
		assert.NotNil(t, updated.LastUsedAt)

		// Check audit action recorded
		require.NotEmpty(t, auditRepo.recordedActions)
		lastAction := auditRepo.recordedActions[len(auditRepo.recordedActions)-1]
		assert.Equal(t, "execute", lastAction["action"])
		assert.Equal(t, "runbook", lastAction["targetType"])
	})

	t.Run("repo update error logged gracefully", func(t *testing.T) {
		repo.updateErr = errors.New("update last_used_at failed")
		req := httptest.NewRequest(http.MethodPost, "/"+validID+"/execute", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		repo.updateErr = nil
	})

	t.Run("audit action error logged gracefully at error level", func(t *testing.T) {
		auditRepo.recordActionErr = errors.New("audit log write failed")
		req := httptest.NewRequest(http.MethodPost, "/"+validID+"/execute", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		auditRepo.recordActionErr = nil
	})
}
