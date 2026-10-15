package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
	alertUsecase "github.com/datdt/k8sselfhost/internal/usecase/alert"
)

type mockAlertRepo struct {
	channels map[string]*alert.NotificationChannel
	rules    map[string]*alert.AlertRule
	history  []*alert.AlertHistory
	silences map[string]bool
}

func newMockAlertRepo() *mockAlertRepo {
	return &mockAlertRepo{
		channels: map[string]*alert.NotificationChannel{
			"ch-1": {ID: "ch-1", TenantID: "default", Name: "slack", Type: "slack", Enabled: true},
		},
		rules:    make(map[string]*alert.AlertRule),
		history:  make([]*alert.AlertHistory, 0),
		silences: make(map[string]bool),
	}
}

func (m *mockAlertRepo) CreateChannel(ctx context.Context, ch *alert.NotificationChannel) error {
	m.channels[ch.ID] = ch
	return nil
}

func (m *mockAlertRepo) ListChannels(ctx context.Context, tenantID string) ([]*alert.NotificationChannel, error) {
	var res []*alert.NotificationChannel
	for _, c := range m.channels {
		res = append(res, c)
	}
	return res, nil
}

func (m *mockAlertRepo) CreateRule(ctx context.Context, r *alert.AlertRule) error {
	m.rules[r.ID] = r
	return nil
}

func (m *mockAlertRepo) ListRules(ctx context.Context, tenantID string) ([]*alert.AlertRule, error) {
	var res []*alert.AlertRule
	for _, r := range m.rules {
		res = append(res, r)
	}
	return res, nil
}

func (m *mockAlertRepo) UpdateRule(ctx context.Context, r *alert.AlertRule) error {
	m.rules[r.ID] = r
	return nil
}

func (m *mockAlertRepo) DeleteRule(ctx context.Context, id, tenantID string) error {
	delete(m.rules, id)
	return nil
}

func (m *mockAlertRepo) ListHistory(ctx context.Context, tenantID string) ([]*alert.AlertHistory, error) {
	return m.history, nil
}

func (m *mockAlertRepo) CreateHistory(ctx context.Context, h *alert.AlertHistory) error {
	m.history = append(m.history, h)
	return nil
}

func (m *mockAlertRepo) AcknowledgeAlert(ctx context.Context, id, tenantID, userID string) error {
	return nil
}

func TestAlertHandler_AlertmanagerRoutes(t *testing.T) {
	repo := newMockAlertRepo()
	uc := alertUsecase.NewUsecase(repo)
	handler := NewAlertHandler(uc)

	r := chi.NewRouter()
	r.Route("/alertmanager", handler.RegisterAlertmanagerRoutes)

	// 1. POST /alertmanager/channels/{id}/test
	req := httptest.NewRequest(http.MethodPost, "/alertmanager/channels/ch-1/test", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// 2. POST /alertmanager/silences
	silenceBody := `{"alert_id":"alert-101","duration_minutes":120,"comment":"upgrading node"}`
	req = httptest.NewRequest(http.MethodPost, "/alertmanager/silences", bytes.NewBufferString(silenceBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. POST /alertmanager/alerts/{id}/resolve
	req = httptest.NewRequest(http.MethodPost, "/alertmanager/alerts/alert-101/resolve", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
