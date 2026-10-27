package compliance

import (
	"context"
	"time"
)

// ComputeHostRow represents a host queried from PostgreSQL compute_hosts table.
type ComputeHostRow struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Address  string            `json:"address"`
	Status   string            `json:"status"`
	Labels   map[string]string `json:"labels"`
}

// FrameworkRecord represents compliance posture scorecard for a framework.
type FrameworkRecord struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Icon         string    `json:"icon"`
	TotalChecks  int       `json:"total_checks"`
	PassedChecks int       `json:"passed_checks"`
	FailedChecks int       `json:"failed_checks"`
	Score        float64   `json:"score"`
	LastScanAt   time.Time `json:"last_scan_at"`
}

// ViolationRecord represents an active policy violation finding.
type ViolationRecord struct {
	ID          string    `json:"id"`
	FrameworkID string    `json:"framework_id"`
	Severity    string    `json:"severity"` // "critical" | "high" | "medium" | "low"
	Policy      string    `json:"policy"`
	Resource    string    `json:"resource"`
	Namespace   string    `json:"namespace"`
	Cluster     string    `json:"cluster"`
	Message     string    `json:"message"`
	Resolved    bool      `json:"resolved"`
	DetectedAt  time.Time `json:"detected_at"`
}

// AuditLogRecord represents an entry in the enterprise audit trail.
type AuditLogRecord struct {
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetName string         `json:"target_name"`
	Result     string         `json:"result"`
	Details    map[string]any `json:"details"`
	IPAddress  string         `json:"ip_address"`
	UserAgent  string         `json:"user_agent"`
	TenantID   string         `json:"tenant_id"`
}

// Store defines repository access for the compliance scanner.
type Store interface {
	ListHosts(ctx context.Context) ([]ComputeHostRow, error)
	CountActiveAutomationRules(ctx context.Context) (int, error)
	CountAuditLogs(ctx context.Context) (int, error)
	UpsertFramework(ctx context.Context, fw *FrameworkRecord) error
	ReplaceViolations(ctx context.Context, violations []ViolationRecord) error
	InsertAuditLog(ctx context.Context, log *AuditLogRecord) error
}

// ScanResult holds the aggregate outcome of a compliance run.
type ScanResult struct {
	Frameworks []FrameworkRecord `json:"frameworks"`
	Violations []ViolationRecord `json:"violations"`
	ScannedAt  time.Time         `json:"scanned_at"`
}
