package catalog

import "context"

type Repository interface {
	Create(ctx context.Context, entry *ServiceEntry) error
	GetByID(ctx context.Context, id string) (*ServiceEntry, error)
	List(ctx context.Context, tenantID string, filter ListFilter) ([]ServiceEntry, error)
	Update(ctx context.Context, entry *ServiceEntry) error
	Delete(ctx context.Context, id string) error
	Stats(ctx context.Context, tenantID string) (*CatalogStats, error)
}

// ListFilter for querying the catalog
type ListFilter struct {
	Type      string `json:"type,omitempty"`
	Lifecycle string `json:"lifecycle,omitempty"`
	OwnerTeam string `json:"owner_team,omitempty"`
	Search    string `json:"search,omitempty"` // name or description contains
}
