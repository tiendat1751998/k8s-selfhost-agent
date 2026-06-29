package audit

import "time"

// AuditFinding represents an issue or gap found by the continuous self-assessment.
type AuditFinding struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"`    // missing_integration | missing_dashboard | broken_route | stale_provider | disconnected_cluster
	Severity    string    `json:"severity"`    // critical | high | medium | low
	Description string    `json:"description"`
	Remediation string    `json:"remediation"`
	Status      string    `json:"status"`      // open | resolved | ignored
	DetectedAt  time.Time `json:"detected_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// AuditRun represents a single execution of the platform audit.
type AuditRun struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"` // running | completed | failed
	StartTime   time.Time `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	FindingsCount int       `json:"findings_count"`
}
