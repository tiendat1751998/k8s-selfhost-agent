package compliance

import "context"

// Repository defines the data access interface for compliance.
type Repository interface {
	ListFrameworks(ctx context.Context) ([]Framework, error)
	GetFramework(ctx context.Context, id string) (*Framework, error)
	UpsertFramework(ctx context.Context, f *Framework) error

	ListViolations(ctx context.Context, severity *ViolationSeverity, limit, offset int) ([]Violation, int, error)
	CreateViolation(ctx context.Context, v *Violation) error
	ResolveViolation(ctx context.Context, id string) error
}
