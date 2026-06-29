package automation

import "context"

// Repository defines the data access interface for automation rules and executions.
type Repository interface {
	// Rules
	ListRules(ctx context.Context) ([]Rule, error)
	GetRule(ctx context.Context, id string) (*Rule, error)
	CreateRule(ctx context.Context, r *Rule) error
	UpdateRule(ctx context.Context, r *Rule) error
	DeleteRule(ctx context.Context, id string) error
	ToggleRule(ctx context.Context, id string, enabled bool) error

	// Executions
	ListExecutions(ctx context.Context, limit, offset int) ([]Execution, int, error)
	CreateExecution(ctx context.Context, e *Execution) error
}
