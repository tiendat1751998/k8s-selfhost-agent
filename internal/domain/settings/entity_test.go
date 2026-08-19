package settings_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/settings"
)

func TestDefaults_AllCategoriesPresent(t *testing.T) {
	requiredCategories := []string{
		settings.CategoryPlatform,
		settings.CategorySecurity,
		settings.CategoryNotifications,
		settings.CategoryIntegrations,
	}

	for _, cat := range requiredCategories {
		categoryMap, exists := settings.Defaults[cat]
		if !exists {
			t.Errorf("expected Defaults map to contain category %q", cat)
			continue
		}
		if len(categoryMap) == 0 {
			t.Errorf("expected Defaults category %q to contain entries, got empty map", cat)
		}
	}

	// Verify specific default keys
	if val := settings.Defaults[settings.CategoryPlatform]["name"]; val != "K8s Self-Host Platform" {
		t.Errorf("expected platform name 'K8s Self-Host Platform', got %q", val)
	}
	if val := settings.Defaults[settings.CategorySecurity]["session_timeout_minutes"]; val != "60" {
		t.Errorf("expected session_timeout_minutes '60', got %q", val)
	}
	if val := settings.Defaults[settings.CategoryNotifications]["smtp_port"]; val != "587" {
		t.Errorf("expected smtp_port '587', got %q", val)
	}
	if _, ok := settings.Defaults[settings.CategoryIntegrations]["argocd_url"]; !ok {
		t.Errorf("expected argocd_url key in integrations category")
	}
}

func TestSetting_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	setting := settings.Setting{
		ID:        "set-123",
		Category:  settings.CategoryPlatform,
		Key:       "timezone",
		Value:     "UTC",
		TenantID:  "tenant-default",
		UpdatedBy: "admin@example.com",
		UpdatedAt: now,
	}

	data, err := json.Marshal(setting)
	if err != nil {
		t.Fatalf("failed to marshal Setting: %v", err)
	}

	var parsed settings.Setting
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal Setting: %v", err)
	}

	if parsed.ID != setting.ID {
		t.Errorf("ID mismatch: got %q, want %q", parsed.ID, setting.ID)
	}
	if parsed.Category != setting.Category {
		t.Errorf("Category mismatch: got %q, want %q", parsed.Category, setting.Category)
	}
	if parsed.Key != setting.Key {
		t.Errorf("Key mismatch: got %q, want %q", parsed.Key, setting.Key)
	}
	if parsed.Value != setting.Value {
		t.Errorf("Value mismatch: got %q, want %q", parsed.Value, setting.Value)
	}
	if parsed.TenantID != setting.TenantID {
		t.Errorf("TenantID mismatch: got %q, want %q", parsed.TenantID, setting.TenantID)
	}
	if parsed.UpdatedBy != setting.UpdatedBy {
		t.Errorf("UpdatedBy mismatch: got %q, want %q", parsed.UpdatedBy, setting.UpdatedBy)
	}
	if !parsed.UpdatedAt.Equal(setting.UpdatedAt) {
		t.Errorf("UpdatedAt mismatch: got %v, want %v", parsed.UpdatedAt, setting.UpdatedAt)
	}
}
