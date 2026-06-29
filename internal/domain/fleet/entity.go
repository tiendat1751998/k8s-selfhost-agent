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
	Nodes          int       `json:"nodes"`
	EncryptedToken string    `json:"encrypted_token,omitempty"`
	TenantID       string    `json:"tenant_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
