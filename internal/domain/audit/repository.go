package audit

import "context"

// Repository defines data access for the platform audit system.
type Repository interface {
	ListFindings(ctx context.Context, status string) ([]AuditFinding, error)
	GetFinding(ctx context.Context, id string) (*AuditFinding, error)
	ResolveFinding(ctx context.Context, id string) error
	RecordRun(ctx context.Context, run *AuditRun) error
	GetLastRun(ctx context.Context) (*AuditRun, error)
	RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error
}
