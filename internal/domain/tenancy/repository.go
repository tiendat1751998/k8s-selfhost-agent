package tenancy

import "context"

type Repository interface {
	GetOrganizations(ctx context.Context) ([]Organization, error)
	GetProjects(ctx context.Context) ([]Project, error)
	GetMembers(ctx context.Context) ([]Member, error)
	GetRBAC(ctx context.Context) (map[string]map[string]bool, error)
	CreateOrganization(ctx context.Context, org Organization) error
	CreateProject(ctx context.Context, proj Project) error
}
