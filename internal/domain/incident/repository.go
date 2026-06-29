package incident

import "context"

// Repository defines the port interface for incident persistence.
// Implementations live in the infrastructure layer.
type Repository interface {
	// Create persists a new incident and assigns an ID.
	Create(ctx context.Context, incident *Incident) error

	// GetByID retrieves an incident by its unique identifier.
	GetByID(ctx context.Context, id string) (*Incident, error)

	// Update saves changes to an existing incident.
	Update(ctx context.Context, incident *Incident) error

	// List retrieves incidents matching the given filter with pagination.
	List(ctx context.Context, filter Filter) ([]*Incident, int64, error)

	// GetByPodAndType finds an active incident for a specific pod and type.
	GetByPodAndType(ctx context.Context, namespace, podName string, incidentType Type) (*Incident, error)
}

// Filter contains parameters for querying incidents.
type Filter struct {
	ClusterName string
	Namespace   string
	Status      *Status
	Type        *Type
	Severity    *Severity
	Limit       int
	Offset      int
}
