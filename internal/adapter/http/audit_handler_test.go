package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/audit"
)

type mockAuditHandlerRepo struct {
	findings      []audit.AuditFinding
	lastRun       *audit.AuditRun
	runs          []*audit.AuditRun
	actions       []map[string]interface{}
	logs          []audit.AuditLog
	totalLogs     int
	listLogsErr   error
	lastLogFilter audit.AuditLogFilter
}

func (m *mockAuditHandlerRepo) ListFindings(ctx context.Context, status string) ([]audit.AuditFinding, error) {
	return m.findings, nil
}

func (m *mockAuditHandlerRepo) GetFinding(ctx context.Context, id string) (*audit.AuditFinding, error) {
	for _, f := range m.findings {
		if f.ID == id {
			return &f, nil
		}
	}
	return nil, errors.New("finding not found")
}

func (m *mockAuditHandlerRepo) ResolveFinding(ctx context.Context, id string) error {
	for i, f := range m.findings {
		if f.ID == id {
			m.findings[i].Status = "resolved"
			now := time.Now()
			m.findings[i].ResolvedAt = &now
			return nil
		}
	}
	return errors.New("finding not found")
}

func (m *mockAuditHandlerRepo) RecordRun(ctx context.Context, run *audit.AuditRun) error {
	run.ID = "test-run-id"
	m.runs = append(m.runs, run)
	m.lastRun = run
	return nil
}

func (m *mockAuditHandlerRepo) GetLastRun(ctx context.Context) (*audit.AuditRun, error) {
	return m.lastRun, nil
}

func (m *mockAuditHandlerRepo) RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error {
	m.actions = append(m.actions, map[string]interface{}{
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

func (m *mockAuditHandlerRepo) ListLogs(ctx context.Context, filter audit.AuditLogFilter) ([]audit.AuditLog, int, error) {
	m.lastLogFilter = filter
	if m.listLogsErr != nil {
		return nil, 0, m.listLogsErr
	}
	return m.logs, m.totalLogs, nil
}

func TestAuditHandler_ListLogs_DefaultParams(t *testing.T) {
	repo := &mockAuditHandlerRepo{
		logs: []audit.AuditLog{
			{
				ID:             "audit-1",
				Actor:          "admin@enterprise.io",
				Action:         "apply",
				ActionType:     "mutation",
				TargetType:     "k8s_manifest",
				TargetResource: "istio-ingress-gateway.yaml",
				Status:         "success",
				Severity:       "medium",
				Details:        map[string]interface{}{"namespace": "istio-system"},
				Payload:        map[string]interface{}{"kind": "Gateway"},
				IPAddress:      "10.240.0.15",
				UserAgent:      "kubectl/v1.30.0",
				Timestamp:      time.Now(),
			},
		},
		totalLogs: 1,
	}

	h := NewAuditHandler(repo)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 50, repo.lastLogFilter.Limit)
	assert.Equal(t, 0, repo.lastLogFilter.Offset)

	var resp struct {
		Data  []audit.AuditLog `json:"data"`
		Total int              `json:"total"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "audit-1", resp.Data[0].ID)
	assert.Equal(t, "admin@enterprise.io", resp.Data[0].Actor)
	assert.Equal(t, "mutation", resp.Data[0].ActionType)
	assert.Equal(t, "istio-ingress-gateway.yaml", resp.Data[0].TargetResource)
	assert.Equal(t, "success", resp.Data[0].Status)
	assert.Equal(t, "medium", resp.Data[0].Severity)
}

func TestAuditHandler_ListLogs_WithFilters(t *testing.T) {
	repo := &mockAuditHandlerRepo{
		logs:      []audit.AuditLog{},
		totalLogs: 0,
	}

	h := NewAuditHandler(repo)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/logs?search=worker-02&action_type=mutation&severity=critical&actor=sre-team&status=error&limit=20&offset=40", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "worker-02", repo.lastLogFilter.Search)
	assert.Equal(t, "mutation", repo.lastLogFilter.ActionType)
	assert.Equal(t, "critical", repo.lastLogFilter.Severity)
	assert.Equal(t, "sre-team", repo.lastLogFilter.Actor)
	assert.Equal(t, "error", repo.lastLogFilter.Status)
	assert.Equal(t, 20, repo.lastLogFilter.Limit)
	assert.Equal(t, 40, repo.lastLogFilter.Offset)

	var resp struct {
		Data  []audit.AuditLog `json:"data"`
		Total int              `json:"total"`
	}
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Total)
	assert.Empty(t, resp.Data)
}

func TestAuditHandler_ListLogs_RepoError(t *testing.T) {
	repo := &mockAuditHandlerRepo{
		listLogsErr: errors.New("database timeout"),
	}

	h := NewAuditHandler(repo)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}