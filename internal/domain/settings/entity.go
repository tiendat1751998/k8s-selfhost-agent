package settings

import "time"

// Setting categories
const (
	CategoryPlatform      = "platform"      // name, logo, timezone, language
	CategorySecurity      = "security"      // session timeout, password policy
	CategoryNotifications = "notifications" // SMTP, webhooks
	CategoryIntegrations  = "integrations"  // ArgoCD URL, Trivy URL, Vault URL
)

// Setting represents a key-value platform configuration item scoped by category and tenant.
type Setting struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	TenantID  string    `json:"tenant_id"`
	UpdatedBy string    `json:"updated_by,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Defaults contains the default settings with their initial values organized by category.
var Defaults = map[string]map[string]string{
	CategoryPlatform: {
		"name":     "K8s Self-Host Platform",
		"timezone": "UTC",
		"language": "en",
	},
	CategorySecurity: {
		"session_timeout_minutes": "60",
		"password_min_length":     "8",
		"require_2fa":             "false",
	},
	CategoryNotifications: {
		"smtp_enabled": "false",
		"smtp_host":    "",
		"smtp_port":    "587",
		"webhook_url":  "",
	},
	CategoryIntegrations: {
		"argocd_url":  "",
		"trivy_url":   "",
		"vault_url":   "",
		"grafana_url": "",
	},
}
