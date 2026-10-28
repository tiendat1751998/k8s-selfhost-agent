package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/alert"
	alertUsecase "github.com/datdt/k8sselfhost/internal/usecase/alert"
)

type mockAlertRepoForHandlerTest struct {
	channels map[string]*alert.NotificationChannel
}

func newMockAlertRepoForHandlerTest() *mockAlertRepoForHandlerTest {
	return &mockAlertRepoForHandlerTest{
		channels: map[string]*alert.NotificationChannel{
			"chan-1": {
				ID:       "chan-1",
				TenantID: "default",
				Name:     "Slack DevOps",
				Type:     "slack",
				Enabled:  true,
			},
			"chan-acme": {
				ID:       "chan-acme",
				TenantID: "acme",
				Name:     "Acme Pager",
				Type:     "pagerduty",
				Enabled:  true,
			},
		},
	}
}

func (m *mockAlertRepoForHandlerTest) CreateChannel(ctx context.Context, ch *alert.NotificationChannel) error {
	m.channels[ch.ID] = ch
	return nil
}

func (m *mockAlertRepoForHandlerTest) ListChannels(ctx context.Context, tenantID string) ([]*alert.NotificationChannel, error) {
	var list []*alert.NotificationChannel
	for _, ch := range m.channels {
		if ch.TenantID == tenantID {
			list = append(list, ch)
		}
	}
	return list, nil
}

func (m *mockAlertRepoForHandlerTest) DeleteChannel(ctx context.Context, id, tenantID string) error {
	ch, ok := m.channels[id]
	if !ok || ch.TenantID != tenantID {
		return alert.ErrNotFound
	}
	delete(m.channels, id)
	return nil
}

func (m *mockAlertRepoForHandlerTest) CreateRule(ctx context.Context, rule *alert.AlertRule) error {
	return nil
}
func (m *mockAlertRepoForHandlerTest) ListRules(ctx context.Context, tenantID string) ([]*alert.AlertRule, error) {
	return nil, nil
}
func (m *mockAlertRepoForHandlerTest) UpdateRule(ctx context.Context, rule *alert.AlertRule) error {
	return nil
}
func (m *mockAlertRepoForHandlerTest) DeleteRule(ctx context.Context, id, tenantID string) error {
	return nil
}
func (m *mockAlertRepoForHandlerTest) CreateHistory(ctx context.Context, history *alert.AlertHistory) error {
	return nil
}
func (m *mockAlertRepoForHandlerTest) ListHistory(ctx context.Context, tenantID string) ([]*alert.AlertHistory, error) {
	return nil, nil
}
func (m *mockAlertRepoForHandlerTest) AcknowledgeAlert(ctx context.Context, id, tenantID, userID string) error {
	return nil
}
func (m *mockAlertRepoForHandlerTest) ResolveAlert(ctx context.Context, id, tenantID, userID string) error {
	return nil
}
func (m *mockAlertRepoForHandlerTest) CreateSilence(ctx context.Context, silence *alert.Silence) error {
	return nil
}
func (m *mockAlertRepoForHandlerTest) ListSilences(ctx context.Context, tenantID string) ([]*alert.Silence, error) {
	return nil, nil
}
func (m *mockAlertRepoForHandlerTest) IsAlertSilenced(ctx context.Context, tenantID, alertID string) (bool, error) {
	return false, nil
}

func TestAlertHandler_DeleteChannel(t *testing.T) {
	repo := newMockAlertRepoForHandlerTest()
	uc := alertUsecase.NewUsecase(repo)
	handler := NewAlertHandler(uc)

	r := chi.NewRouter()
	r.Route("/api/v1/alerts", handler.RegisterRoutes)

	t.Run("successfully deletes existing channel", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/alerts/channels/chan-1", nil)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
			t.Fatalf("expected status 200 or 204, got %d: %s", rec.Code, rec.Body.String())
		}

		if _, exists := repo.channels["chan-1"]; exists {
			t.Fatalf("expected chan-1 to be deleted from repo")
		}
	})

	t.Run("returns 404 for unknown channel", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/alerts/channels/unknown-chan-999", nil)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("returns 404 for channel in another tenant (tenant isolation)", func(t *testing.T) {
		// Default tenant context cannot delete acme channel
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/alerts/channels/chan-acme", nil)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404 for cross-tenant delete, got %d: %s", rec.Code, rec.Body.String())
		}

		// Ensure it was NOT deleted
		if _, exists := repo.channels["chan-acme"]; !exists {
			t.Fatalf("chan-acme should still exist in repo")
		}
	})

	t.Run("allows deletion when matching tenant context provided", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/alerts/channels/chan-acme", nil)
		ctx := context.WithValue(req.Context(), middleware.TenantIDKey, "acme")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
			t.Fatalf("expected status 200 or 204 for acme tenant delete, got %d: %s", rec.Code, rec.Body.String())
		}

		if _, exists := repo.channels["chan-acme"]; exists {
			t.Fatalf("expected chan-acme to be deleted from repo")
		}
	})
}
