package catalog

import "time"

// Service types
const (
	TypeService  = "service"
	TypeAPI      = "api"
	TypeLibrary  = "library"
	TypeDatabase = "database"
	TypeFrontend = "frontend"
	TypeWorker   = "worker"
)

// Lifecycle stages
const (
	LifecycleDevelopment = "development"
	LifecycleStaging     = "staging"
	LifecycleProduction  = "production"
	LifecycleDeprecated  = "deprecated"
)

// ServiceEntry represents a registered service in the catalog
type ServiceEntry struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"`        // service, api, library, database, frontend, worker
	Lifecycle   string            `json:"lifecycle"`   // development, staging, production, deprecated
	OwnerTeam   string            `json:"owner_team"`
	OwnerEmail  string            `json:"owner_email"`
	RepoURL     string            `json:"repo_url"`
	DocsURL     string            `json:"docs_url"`
	Tags        []string          `json:"tags"`
	Annotations map[string]string `json:"annotations"` // k8s refs: namespace, cluster, deployment
	TenantID    string            `json:"tenant_id"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CatalogStats aggregates catalog metrics
type CatalogStats struct {
	Total       int            `json:"total"`
	ByType      map[string]int `json:"by_type"`
	ByLifecycle map[string]int `json:"by_lifecycle"`
}
