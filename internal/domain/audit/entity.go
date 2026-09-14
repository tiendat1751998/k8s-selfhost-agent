package audit

import "time"

// AuditFinding represents an issue or gap found by the continuous self-assessment.
type AuditFinding struct {
	ID          string     `json:"id"`
	Category    string     `json:"category"` // missing_integration | missing_dashboard | broken_route | stale_provider | disconnected_cluster
	Severity    string     `json:"severity"` // critical | high | medium | low
	Description string     `json:"description"`
	Remediation string     `json:"remediation"`
	Status      string     `json:"status"` // open | resolved | ignored
	DetectedAt  time.Time  `json:"detected_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// AuditRun represents a single execution of the platform audit.
type AuditRun struct {
	ID            string     `json:"id"`
	Status        string     `json:"status"` // running | completed | failed
	StartTime     time.Time  `json:"start_time"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	FindingsCount int        `json:"findings_count"`
}

// AuditLog represents an enterprise audit event for security and compliance tracking.
type AuditLog struct {
	ID             string                 `json:"id"`
	Actor          string                 `json:"actor"`
	Action         string                 `json:"action"`
	ActionType     string                 `json:"action_type"` // 'mutation' | 'access' | 'rbac_grant' | 'deletion'
	TargetType     string                 `json:"target_type"`
	TargetID       *string                `json:"target_id,omitempty"`
	TargetResource string                 `json:"target_resource"`
	Status         string                 `json:"status"`   // 'success' | 'denied' | 'error' | 'pending'
	Severity       string                 `json:"severity"` // 'critical' | 'high' | 'medium' | 'low' | 'info'
	Details        map[string]interface{} `json:"details"`
	Payload        map[string]interface{} `json:"payload"`
	IPAddress      string                 `json:"ip_address"`
	UserAgent      string                 `json:"user_agent"`
	Timestamp      time.Time              `json:"timestamp"`
	TenantID       *string                `json:"tenant_id,omitempty"`
}

// AuditLogFilter defines search and pagination criteria for querying audit logs.
type AuditLogFilter struct {
	Search     string
	ActionType string
	Severity   string
	Actor      string
	Status     string
	Limit      int
	Offset     int
}
