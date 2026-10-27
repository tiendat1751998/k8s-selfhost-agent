package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
	alertUsecase "github.com/datdt/k8sselfhost/internal/usecase/alert"
)

type mockAlertRepoForPartialUpdate struct {
	rules []*alert.AlertRule
}

func (m *mockAlertRepoForPartialUpdate) CreateChannel(ctx context.Context, channel *alert.NotificationChannel) error {
	return nil
}
func (m *mockAlertRepoForPartialUpdate) ListChannels(ctx context.Context, tenantID string) ([]*alert.NotificationChannel, error) {
	return nil, nil
}
func (m *mockAlertRepoForPartialUpdate) CreateRule(ctx context.Context, rule *alert.AlertRule) error {
	m.rules = append(m.rules, rule)
	return nil
}
func (m *mockAlertRepoForPartialUpdate) ListRules(ctx context.Context, tenantID string) ([]*alert.AlertRule, error) {
	return m.rules, nil
}
func (m *mockAlertRepoForPartialUpdate) UpdateRule(ctx context.Context, rule *alert.AlertRule) error {
	for i, r := range m.rules {
		if r.ID == rule.ID {
			m.rules[i] = rule
			return nil
		}
	}
	return nil
}
func (m *mockAlertRepoForPartialUpdate) DeleteRule(ctx context.Context, id, tenantID string) error {
	return nil
}
func (m *mockAlertRepoForPartialUpdate) CreateHistory(ctx context.Context, history *alert.AlertHistory) error {
	return nil
}
func (m *mockAlertRepoForPartialUpdate) ListHistory(ctx context.Context, tenantID string) ([]*alert.AlertHistory, error) {
	return nil, nil
}
func (m *mockAlertRepoForPartialUpdate) AcknowledgeAlert(ctx context.Context, id, tenantID, userID string) error {
	return nil
}
func (m *mockAlertRepoForPartialUpdate) ResolveAlert(ctx context.Context, id, tenantID, userID string) error {
	return nil
}
func (m *mockAlertRepoForPartialUpdate) CreateSilence(ctx context.Context, s *alert.Silence) error {
	return nil
}
func (m *mockAlertRepoForPartialUpdate) ListSilences(ctx context.Context, tenantID string) ([]*alert.Silence, error) {
	return nil, nil
}
func (m *mockAlertRepoForPartialUpdate) IsAlertSilenced(ctx context.Context, tenantID, alertID string) (bool, error) {
	return false, nil
}


func TestAlertHandler_UpdateRule_PartialUpdate(t *testing.T) {
	ruleID := uuid.NewString()
	initialRule := &alert.AlertRule{
		ID:              ruleID,
		TenantID:        "default",
		Name:            "High CPU Alert",
		Description:     "Triggers when node CPU > 80%",
		MetricName:      "node_cpu_utilization",
		Condition:       ">",
		Threshold:       80.0,
		DurationSeconds: 300,
		Severity:        "warning",
		Enabled:         true,
	}

	repo := &mockAlertRepoForPartialUpdate{
		rules: []*alert.AlertRule{initialRule},
	}
	uc := alertUsecase.NewUsecase(repo)
	handler := NewAlertHandler(uc)

	r := chi.NewRouter()
	r.Route("/alerts", handler.RegisterRoutes)

	// 1. Partial update: disable rule with {"enabled": false}
	bodyToggle := `{"enabled": false}`
	req := httptest.NewRequest(http.MethodPut, "/alerts/rules/"+ruleID, bytes.NewBufferString(bodyToggle))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for partial update enabled, got %d: %s", rec.Code, rec.Body.String())
	}
	var updated1 alert.AlertRule
	if err := json.Unmarshal(rec.Body.Bytes(), &updated1); err != nil {
		t.Fatalf("failed to parse updated rule: %v", err)
	}
	if updated1.Enabled != false {
		t.Fatal("expected enabled to be false")
	}
	if updated1.Name != "High CPU Alert" || updated1.Threshold != 80.0 || updated1.MetricName != "node_cpu_utilization" {
		t.Fatalf("existing fields were wiped: %+v", updated1)
	}

	// 2. Partial update: change threshold to 92.5
	bodyThreshold := `{"threshold": 92.5}`
	req = httptest.NewRequest(http.MethodPut, "/alerts/rules/"+ruleID, bytes.NewBufferString(bodyThreshold))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for partial update threshold, got %d: %s", rec.Code, rec.Body.String())
	}
	var updated2 alert.AlertRule
	if err := json.Unmarshal(rec.Body.Bytes(), &updated2); err != nil {
		t.Fatalf("failed to parse updated rule: %v", err)
	}
	if updated2.Threshold != 92.5 {
		t.Fatalf("expected threshold 92.5, got %f", updated2.Threshold)
	}
	if updated2.Enabled != false {
		t.Fatal("expected enabled to remain false")
	}

	// 3. Update non-existent rule -> 404
	randomID := uuid.NewString()
	req = httptest.NewRequest(http.MethodPut, "/alerts/rules/"+randomID, bytes.NewBufferString(bodyToggle))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent rule, got %d", rec.Code)
	}

	// 4. Update with empty name -> 400 validation error
	bodyBad := `{"name": "   "}`
	req = httptest.NewRequest(http.MethodPut, "/alerts/rules/"+ruleID, bytes.NewBufferString(bodyBad))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name, got %d", rec.Code)
	}
}
