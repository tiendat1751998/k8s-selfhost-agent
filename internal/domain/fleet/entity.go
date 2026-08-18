package fleet

import "time"

// Cluster represents a Kubernetes cluster in the fleet.
type Cluster struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Group     string    `json:"group"`      // e.g. production, staging, edge
	Region    string    `json:"region"`
	Provider  string    `json:"provider"`   // e.g. aws, gcp, azure, onprem
	Status    string    `json:"status"`     // active, offline, upgrading, maintenance
	Version   string    `json:"version"`
	Nodes               int                    `json:"nodes"`
	EncryptedToken      string                 `json:"encrypted_token,omitempty"`
	ImportMethod        string                 `json:"import_method"`
	KubeconfigHash      string                 `json:"kubeconfig_hash,omitempty"`
	LastHealthCheck     *time.Time             `json:"last_health_check,omitempty"`
	HealthStatus        string                 `json:"health_status"`
	DiscoveredResources map[string]interface{} `json:"discovered_resources,omitempty"`
	TenantID            string                 `json:"tenant_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
