package ecosystem

import (
	"errors"
	"strings"
	"time"
)

// Categories for detected tools
const (
	CategoryGitOps       = "gitops"
	CategorySecurity     = "security"
	CategoryPolicy       = "policy"
	CategoryMesh         = "mesh"
	CategoryMonitoring   = "monitoring"
	CategorySecrets      = "secrets"
	CategoryCertificates = "certificates"
	CategoryCompute      = "compute"
	CategoryDatabase     = "database"
	CategoryMessaging    = "messaging"
)

// Status values
const (
	StatusDetected      = "detected"
	StatusUnreachable   = "unreachable"
	StatusNotConfigured = "not_configured"
)

// Source values
const (
	SourceSettings     = "settings"
	SourceK8sDiscovery = "k8s_discovery"
	SourceManual       = "manual"
)

// Health values
const (
	HealthHealthy  = "healthy"
	HealthDegraded = "degraded"
	HealthUnknown  = "unknown"
)

// DetectedTool represents an installed or configured infrastructure/platform tool.
type DetectedTool struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`         // ArgoCD, Trivy, Kyverno, Istio, Prometheus, Grafana, Vault, cert-manager
	Category    string            `json:"category"`     // gitops, security, policy, mesh, monitoring, secrets, certificates
	Status      string            `json:"status"`       // detected, unreachable, not_configured
	Version     string            `json:"version"`      // detected version if available
	Endpoint    string            `json:"endpoint"`     // URL or K8s service reference
	Source      string            `json:"source"`       // settings, k8s_discovery, manual
	Health      string            `json:"health"`       // healthy, degraded, unknown
	LastChecked time.Time         `json:"last_checked"`
	Metadata    map[string]string `json:"metadata"` // extra info per tool
	TenantID    string            `json:"tenant_id"`
}

// Validate checks whether mandatory fields of DetectedTool are populated.
func (t *DetectedTool) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("tool name is required")
	}
	if strings.TrimSpace(t.Category) == "" {
		return errors.New("tool category is required")
	}
	return nil
}

// EcosystemSummary provides aggregate statistics on detected ecosystem tools.
type EcosystemSummary struct {
	Total      int            `json:"total"`
	Healthy    int            `json:"healthy"`
	Degraded   int            `json:"degraded"`
	ByCategory map[string]int `json:"by_category"`
}
