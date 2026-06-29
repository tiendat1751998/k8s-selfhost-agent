package report

import "context"

// Repository defines the port interface for RCA report persistence.
type Repository interface {
	// Create persists a new RCA report and assigns an ID.
	Create(ctx context.Context, report *Report) error

	// GetByID retrieves a report by its unique identifier.
	GetByID(ctx context.Context, id string) (*Report, error)

	// GetByIncidentID retrieves the report for a specific incident.
	GetByIncidentID(ctx context.Context, incidentID string) (*Report, error)

	// List retrieves reports with pagination.
	List(ctx context.Context, limit, offset int) ([]*Report, int64, error)
}
