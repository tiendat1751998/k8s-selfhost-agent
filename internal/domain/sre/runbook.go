// Package sre defines domain entities and interfaces for Site Reliability Engineering operations.
package sre

import (
	"context"
	"time"
)

// RunbookID represents a unique identifier for an interactive operational runbook.
type RunbookID string

const (
	// RunbookRollbackDeployment triggers a deployment rollback to revision N-1.
	RunbookRollbackDeployment RunbookID = "RUNBOOK-01-ROLLBACK"
	// RunbookFencingNode cordons a node and isolates failing workloads.
	RunbookFencingNode RunbookID = "RUNBOOK-02-NODE-FENCING"
	// RunbookRotateSecret performs zero-downtime secret rotation and triggers consumers.
	RunbookRotateSecret RunbookID = "RUNBOOK-03-ROTATE-SECRET"
	// RunbookRestoreDrill verifies backup integrity and runs a non-destructive restore drill.
	RunbookRestoreDrill RunbookID = "RUNBOOK-04-RESTORE-DRILL"
	// RunbookRenewCertificates renews expiring TLS certificates via cert-manager.
	RunbookRenewCertificates RunbookID = "RUNBOOK-05-RENEW-CERTS"
)

// RunbookExecutionRequest encapsulates parameters to execute an interactive runbook.
type RunbookExecutionRequest struct {
	RunbookID   RunbookID         `json:"runbook_id"`
	TriggeredBy string            `json:"triggered_by"` // Telegram:@user, WebUI:admin
	Parameters  map[string]string `json:"parameters"`
	IncidentID  string            `json:"incident_id,omitempty"`
}

// RunbookExecutionResult contains execution status and audit telemetry.
type RunbookExecutionResult struct {
	ExecutionID string        `json:"execution_id"`
	Status      string        `json:"status"` // SUCCESS, FAILED, TIMEOUT
	Output      string        `json:"output"`
	Duration    time.Duration `json:"duration"`
	ExecutedAt  time.Time     `json:"executed_at"`
}

// RunbookExecutor defines the port for executing automated/interactive runbooks.
type RunbookExecutor interface {
	Execute(ctx context.Context, req RunbookExecutionRequest) (*RunbookExecutionResult, error)
}
