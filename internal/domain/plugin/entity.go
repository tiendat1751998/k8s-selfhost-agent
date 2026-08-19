package plugin

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Plugin categories
const (
	CategoryMonitoring  = "monitoring"
	CategorySecurity    = "security"
	CategoryDevtools    = "devtools"
	CategoryIntegration = "integration"
)

// Common domain errors
var (
	ErrNotFound      = errors.New("plugin not found")
	ErrDuplicateName = errors.New("plugin with this name already exists")
	ErrInvalidInput  = errors.New("invalid plugin input")
)

// ValidCategories contains all supported plugin categories.
var ValidCategories = map[string]bool{
	CategoryMonitoring:  true,
	CategorySecurity:    true,
	CategoryDevtools:    true,
	CategoryIntegration: true,
}

// Plugin represents an extensible runtime frontend JS plugin bundle.
type Plugin struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	Enabled     bool              `json:"enabled"`
	EntryPoint  string            `json:"entry_point"` // JS bundle URL
	Icon        string            `json:"icon"`
	Category    string            `json:"category"`    // monitoring, security, devtools, integration
	Permissions []string          `json:"permissions"` // required permissions
	Config      map[string]string `json:"config"`      // plugin-specific key-value configuration
	TenantID    string            `json:"tenant_id"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PluginStats aggregates plugin statistics.
type PluginStats struct {
	Total      int            `json:"total"`
	Enabled    int            `json:"enabled"`
	Disabled   int            `json:"disabled"`
	ByCategory map[string]int `json:"by_category"`
}

// Repository defines the persistence interface for plugins.
type Repository interface {
	Create(ctx context.Context, p *Plugin) error
	GetByID(ctx context.Context, id string) (*Plugin, error)
	List(ctx context.Context, tenantID string, enabledOnly bool) ([]Plugin, error)
	Update(ctx context.Context, p *Plugin) error
	Delete(ctx context.Context, id string) error
	Toggle(ctx context.Context, id string, enabled bool) error
	GetStats(ctx context.Context, tenantID string) (*PluginStats, error)
}

// Validate checks the domain invariants of a Plugin.
func (p *Plugin) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("plugin name is required")
	}
	if p.Category != "" && !ValidCategories[p.Category] {
		return errors.New("invalid plugin category: " + p.Category)
	}
	return nil
}
