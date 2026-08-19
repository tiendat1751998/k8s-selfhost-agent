package scaffold

import (
	"context"
	"time"
)

// Standard template categories
const (
	CategoryWeb       = "web"
	CategoryAPI       = "api"
	CategoryWorker    = "worker"
	CategoryDatabase  = "database"
	CategoryFullstack = "fullstack"
)

// Standard framework types
const (
	FrameworkGoChi        = "go-chi"
	FrameworkNodeExpress  = "node-express"
	FrameworkPythonFastAPI = "python-fastapi"
	FrameworkReact        = "react"
	FrameworkVue          = "vue"
	FrameworkPostgres     = "postgres"
)

// Template represents a reusable application scaffold template.
type Template struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	Category      string             `json:"category"`       // web, api, worker, database, fullstack
	Framework     string             `json:"framework"`      // go-chi, node-express, python-fastapi, react, vue, postgres
	ManifestYAML  string             `json:"manifest_yaml"`  // K8s YAML template
	HelmValues    string             `json:"helm_values"`    // Helm values.yaml template
	DockerCompose string             `json:"docker_compose"` // docker-compose template
	Variables     []TemplateVariable `json:"variables"`      // user-fillable variables
	Tags          []string           `json:"tags"`
	BuiltIn       bool               `json:"built_in"`       // system template vs user-created
	TenantID      string             `json:"tenant_id"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

// TemplateVariable defines a customizable parameter for the template.
type TemplateVariable struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"` // string, number, boolean, select
	Default  string   `json:"default"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"` // for select type
}

// ScaffoldResult represents the rendered output across all targets.
type ScaffoldResult struct {
	RenderedYAML    string `json:"rendered_yaml"`
	RenderedCompose string `json:"rendered_compose"`
	RenderedHelm    string `json:"rendered_helm"`
}

// Repository defines data access operations for templates.
type Repository interface {
	Create(ctx context.Context, t *Template) error
	GetByID(ctx context.Context, id string) (*Template, error)
	List(ctx context.Context, tenantID string, filter ListFilter) ([]Template, error)
	Update(ctx context.Context, t *Template) error
	Delete(ctx context.Context, id string) error
}

// ListFilter provides query criteria when listing templates.
type ListFilter struct {
	Category  string `json:"category,omitempty"`
	Framework string `json:"framework,omitempty"`
	Search    string `json:"search,omitempty"`
}
