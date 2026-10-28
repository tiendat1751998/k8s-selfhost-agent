package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
	"github.com/datdt/k8sselfhost/internal/domain/settings"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	alertUsecase "github.com/datdt/k8sselfhost/internal/usecase/alert"
)

func TestSettingsHandler_Masking(t *testing.T) {
	repo := newMockSettingsRepo()
	handler := NewSettingsHandler(repo, nil)

	ctx := tenancy.WithTenantID(context.Background(), "test-tenant")

	_ = repo.Upsert(ctx, &settings.Setting{
		Category: "email",
		Key:      "smtp_password",
		Value:    "supersecretpass",
		TenantID: "test-tenant",
	})
	_ = repo.Upsert(ctx, &settings.Setting{
		Category: "auth",
		Key:      "api_token",
		Value:    "tok_abcdef123456",
		TenantID: "test-tenant",
	})
	_ = repo.Upsert(ctx, &settings.Setting{
		Category: "general",
		Key:      "app_name",
		Value:    "My Kubernetes Platform",
		TenantID: "test-tenant",
	})

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var items []settings.Setting
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	for _, item := range items {
		if item.Key == "smtp_password" && item.Value != "***" {
			t.Fatalf("expected smtp_password to be masked with '***', got %q", item.Value)
		}
		if item.Key == "api_token" && item.Value != "***" {
			t.Fatalf("expected api_token to be masked with '***', got %q", item.Value)
		}
		if item.Key == "app_name" && item.Value != "My Kubernetes Platform" {
			t.Fatalf("expected app_name to be unchanged, got %q", item.Value)
		}
	}
}

func TestSettingsHandler_UpdateSkipsMasked(t *testing.T) {
	repo := newMockSettingsRepo()
	handler := NewSettingsHandler(repo, nil)

	ctx := tenancy.WithTenantID(context.Background(), "test-tenant")

	_ = repo.Upsert(ctx, &settings.Setting{
		Category: "email",
		Key:      "smtp_password",
		Value:    "original_secret_pass",
		TenantID: "test-tenant",
	})
	_ = repo.Upsert(ctx, &settings.Setting{
		Category: "general",
		Key:      "app_name",
		Value:    "OldAppName",
		TenantID: "test-tenant",
	})

	payload := updateSettingsRequest{
		Settings: []updateSettingItem{
			{Category: "email", Key: "smtp_password", Value: "***"},
			{Category: "general", Key: "app_name", Value: "NewAppName"},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewReader(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.UpdateSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	pwSetting, err := repo.GetByKey(ctx, "test-tenant", "email", "smtp_password")
	if err != nil || pwSetting == nil {
		t.Fatalf("failed to get password setting: %v", err)
	}
	if pwSetting.Value != "original_secret_pass" {
		t.Fatalf("password setting was overwritten with masked value! got %q", pwSetting.Value)
	}

	nameSetting, err := repo.GetByKey(ctx, "test-tenant", "general", "app_name")
	if err != nil || nameSetting == nil {
		t.Fatalf("failed to get app_name setting: %v", err)
	}
	if nameSetting.Value != "NewAppName" {
		t.Fatalf("expected app_name to be 'NewAppName', got %q", nameSetting.Value)
	}
}

type mockAlertRepoForMasking struct {
	channels []*alert.NotificationChannel
}

func (m *mockAlertRepoForMasking) CreateChannel(ctx context.Context, channel *alert.NotificationChannel) error {
	return nil
}
func (m *mockAlertRepoForMasking) ListChannels(ctx context.Context, tenantID string) ([]*alert.NotificationChannel, error) {
	return m.channels, nil
}
func (m *mockAlertRepoForMasking) DeleteChannel(ctx context.Context, id, tenantID string) error {
	return nil
}
func (m *mockAlertRepoForMasking) CreateRule(ctx context.Context, rule *alert.AlertRule) error {
	return nil
}
func (m *mockAlertRepoForMasking) ListRules(ctx context.Context, tenantID string) ([]*alert.AlertRule, error) {
	return nil, nil
}
func (m *mockAlertRepoForMasking) UpdateRule(ctx context.Context, rule *alert.AlertRule) error {
	return nil
}
func (m *mockAlertRepoForMasking) DeleteRule(ctx context.Context, id, tenantID string) error {
	return nil
}
func (m *mockAlertRepoForMasking) CreateHistory(ctx context.Context, history *alert.AlertHistory) error {
	return nil
}
func (m *mockAlertRepoForMasking) ListHistory(ctx context.Context, tenantID string) ([]*alert.AlertHistory, error) {
	return nil, nil
}
func (m *mockAlertRepoForMasking) AcknowledgeAlert(ctx context.Context, id, tenantID, userID string) error {
	return nil
}
func (m *mockAlertRepoForMasking) ResolveAlert(ctx context.Context, id, tenantID, userID string) error {
	return nil
}
func (m *mockAlertRepoForMasking) CreateSilence(ctx context.Context, s *alert.Silence) error {
	return nil
}
func (m *mockAlertRepoForMasking) ListSilences(ctx context.Context, tenantID string) ([]*alert.Silence, error) {
	return nil, nil
}
func (m *mockAlertRepoForMasking) IsAlertSilenced(ctx context.Context, tenantID, alertID string) (bool, error) {
	return false, nil
}


func TestAlertHandler_ListChannels_Masking(t *testing.T) {
	testChannels := []*alert.NotificationChannel{
		{
			ID:       "chan-1",
			TenantID: "default",
			Name:     "Slack Alerts",
			Type:     "slack",
			Config: map[string]interface{}{
				"webhook_url": "https://hooks.slack.com/services/T00/B00/X00",
				"channel":     "#devops",
				"bot_token":   "xoxb-12345678",
			},
		},
	}

	repo := &mockAlertRepoForMasking{channels: testChannels}
	uc := alertUsecase.NewUsecase(repo)
	handler := NewAlertHandler(uc)

	req := httptest.NewRequest(http.MethodGet, "/channels", nil)
	rec := httptest.NewRecorder()

	handler.ListChannels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Data []*alert.NotificationChannel `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(resp.Data))
	}

	cfg := resp.Data[0].Config
	if cfg["webhook_url"] != "***" {
		t.Fatalf("expected webhook_url to be '***', got %v", cfg["webhook_url"])
	}
	if cfg["bot_token"] != "***" {
		t.Fatalf("expected bot_token to be '***', got %v", cfg["bot_token"])
	}
	if cfg["channel"] != "#devops" {
		t.Fatalf("expected non-sensitive channel '#devops' to remain unchanged, got %v", cfg["channel"])
	}
}
