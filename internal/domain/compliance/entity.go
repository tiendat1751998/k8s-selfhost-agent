// Package compliance provides domain entities for compliance and security posture.
package compliance

import "time"

// Framework represents a compliance framework and its check results.
type Framework struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Icon         string    `json:"icon"`
	TotalChecks  int       `json:"total_checks"`
	PassedChecks int       `json:"passed_checks"`
	FailedChecks int       `json:"failed_checks"`
	Score        float64   `json:"score"`
	LastScanAt   time.Time `json:"last_scan_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ViolationSeverity represents the severity level of a violation.
type ViolationSeverity string

const (
	SeverityCritical ViolationSeverity = "critical"
	SeverityHigh     ViolationSeverity = "high"
	SeverityMedium   ViolationSeverity = "medium"
	SeverityLow      ViolationSeverity = "low"
)

// Violation represents a policy violation.
type Violation struct {
	ID          string            `json:"id"`
	FrameworkID string            `json:"framework_id"`
	Severity    ViolationSeverity `json:"severity"`
	Policy      string            `json:"policy"`
	Resource    string            `json:"resource"`
	Namespace   string            `json:"namespace"`
	Cluster     string            `json:"cluster"`
	Message     string            `json:"message"`
	Resolved    bool              `json:"resolved"`
	DetectedAt  time.Time         `json:"detected_at"`
}
