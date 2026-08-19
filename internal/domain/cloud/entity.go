package cloud

import (
	"time"
)

// ProviderType represents supported cloud provider platforms.
type ProviderType string

const (
	ProviderAWS   ProviderType = "aws"
	ProviderGCP   ProviderType = "gcp"
	ProviderAzure ProviderType = "azure"
)

// AccountStatus represents the operational status of a cloud account.
type AccountStatus string

const (
	AccountStatusActive  AccountStatus = "active"
	AccountStatusInvalid AccountStatus = "invalid"
	AccountStatusExpired AccountStatus = "expired"
)

// CloudAccount represents a registered cloud provider credential and configuration.
type CloudAccount struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Provider       ProviderType `json:"provider"`
	EncryptedCreds string       `json:"-"` // NEVER in JSON
	Region         string       `json:"region"`
	Status         string       `json:"status"` // active, invalid, expired
	TenantID       string       `json:"tenant_id"`
	LastSyncAt     *time.Time   `json:"last_sync_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// CloudCluster represents a discovered Kubernetes cluster hosted on a cloud provider.
type CloudCluster struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Provider   ProviderType      `json:"provider"`
	Region     string            `json:"region"`
	Status     string            `json:"status"` // e.g. ACTIVE, CREATING, UPDATING, DELETING, FAILED
	Version    string            `json:"version"`
	Endpoint   string            `json:"endpoint,omitempty"`
	NodeGroups []CloudNodeGroup  `json:"node_groups,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	CreatedAt  *time.Time        `json:"created_at,omitempty"`
}

// CloudNodeGroup represents a managed worker node group / pool in a cloud Kubernetes cluster.
type CloudNodeGroup struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	ClusterName  string            `json:"cluster_name,omitempty"`
	Status       string            `json:"status"`
	InstanceType string            `json:"instance_type,omitempty"`
	MinSize      int               `json:"min_size"`
	MaxSize      int               `json:"max_size"`
	DesiredSize  int               `json:"desired_size"`
	CurrentSize  int               `json:"current_size,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// NewCloudAccount creates a new CloudAccount initialized with active status and UTC timestamps.
func NewCloudAccount(name string, provider ProviderType, encryptedCreds, region, tenantID string) *CloudAccount {
	now := time.Now().UTC()
	return &CloudAccount{
		Name:           name,
		Provider:       provider,
		EncryptedCreds: encryptedCreds,
		Region:         region,
		Status:         string(AccountStatusActive),
		TenantID:       tenantID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
