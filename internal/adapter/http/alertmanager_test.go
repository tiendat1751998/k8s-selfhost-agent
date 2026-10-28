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
	silences       []*alert.Silence
	resolvedAlerts map[string]bool
	ackedAlerts    map[string]bool
}

func newMockAlertRepo() *mockAlertRepo {
	return &mockAlertRepo{
		channels: map[string]*alert.NotificationChannel{
			"ch-1": {ID: "ch-1", TenantID: "default", Name: "slack", Type: "slack", Enabled: true},
			"ch-ssrf": {
				ID: "ch-ssrf", TenantID: "default", Name: "ssrf-test", Type: "slack", Enabled: true,
				Config: map[string]interface{}{"webhook_url": "http://169.254.169.254/secret"},
			},
		},
		rules:          make(map[string]*alert.AlertRule),
		history:        make([]*alert.AlertHistory, 0),
		silences:       make([]*alert.Silence, 0),
		resolvedAlerts: make(map[string]bool),
		ackedAlerts:    make(map[string]bool),
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

func (m *mockAlertRepo) DeleteChannel(ctx context.Context, id, tenantID string) error {
	ch, ok := m.channels[id]
	if !ok || (tenantID != "" && ch.TenantID != tenantID) {
		return alert.ErrNotFound
	}
	delete(m.channels, id)
	return nil
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
	m.ackedAlerts[id] = true
	return nil
}

func (m *mockAlertRepo) ResolveAlert(ctx context.Context, id, tenantID, userID string) error {
	m.resolvedAlerts[id] = true
	return nil
}

func (m *mockAlertRepo) CreateSilence(ctx context.Context, s *alert.Silence) error {
	m.silences = append(m.silences, s)
	return nil
}

func (m *mockAlertRepo) ListSilences(ctx context.Context, tenantID string) ([]*alert.Silence, error) {
	return m.silences, nil
}

func (m *mockAlertRepo) IsAlertSilenced(ctx context.Context, tenantID, alertID string) (bool, error) {
	for _, s := range m.silences {
		if s.AlertID == alertID && s.IsActive() {
			return true, nil
		}
	}
	return false, nil
}

func TestAlertHandler_AlertmanagerRoutes(t *testing.T) {
	repo := newMockAlertRepo()
	uc := alertUsecase.NewUsecase(repo)
	handler := NewAlertHandler(uc)

	r := chi.NewRouter()
	r.Route("/alertmanager", handler.RegisterAlertmanagerRoutes)

	// 1. POST /alertmanager/channels/{id}/test without webhook_url -> 400 Bad Request
	req := httptest.NewRequest(http.MethodPost, "/alertmanager/channels/ch-1/test", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unconfigured channel, got %d: %s", rr.Code, rr.Body.String())
	}

	// 2. POST /alertmanager/channels/{id}/test with SSRF private IP -> 400 Bad Request
	req = httptest.NewRequest(http.MethodPost, "/alertmanager/channels/ch-ssrf/test", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for SSRF channel, got %d: %s", rr.Code, rr.Body.String())
	}

	// 3. POST /alertmanager/silences -> creates and persists silence
	silenceBody := `{"alert_id":"alert-101","duration_minutes":120,"comment":"upgrading node"}`
	req = httptest.NewRequest(http.MethodPost, "/alertmanager/silences", bytes.NewBufferString(silenceBody))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(repo.silences) != 1 || repo.silences[0].AlertID != "alert-101" {
		t.Fatalf("expected silence persisted in repository, got %+v", repo.silences)
	}

	// 4. GET /alertmanager/silences -> returns active silences
	req = httptest.NewRequest(http.MethodGet, "/alertmanager/silences", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// 5. POST /alertmanager/alerts/{id}/resolve -> calls ResolveAlert on repo, NOT AcknowledgeAlert
	req = httptest.NewRequest(http.MethodPost, "/alertmanager/alerts/alert-101/resolve", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !repo.resolvedAlerts["alert-101"] {
		t.Fatalf("expected alert-101 to be in resolvedAlerts map")
	}
	if repo.ackedAlerts["alert-101"] {
		t.Fatalf("ResolveAlert must NOT call AcknowledgeAlert")
	}
}

