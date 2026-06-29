package gitops

import "context"

// Repository defines the port interface for GitOps PR persistence.
type Repository interface {
	// Create persists a new pull request and assigns an ID.
	Create(ctx context.Context, pr *PullRequest) error

	// GetByID retrieves a pull request by its unique identifier.
	GetByID(ctx context.Context, id string) (*PullRequest, error)

	// GetByIncidentID retrieves the pull request for a specific incident.
	GetByIncidentID(ctx context.Context, incidentID string) (*PullRequest, error)

	// Update saves changes to an existing pull request.
	Update(ctx context.Context, pr *PullRequest) error

	// List retrieves pull requests with optional status filtering and pagination.
	List(ctx context.Context, status *PRStatus, limit, offset int) ([]*PullRequest, int64, error)
}
