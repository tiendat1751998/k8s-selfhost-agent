// Package report defines the RCA Report aggregate root.
package report

import (
	"time"

	"github.com/datdt/k8sselfhost/pkg/errors"
)

// Report represents the result of a Root Cause Analysis for an incident.
type Report struct {
	ID             string
	IncidentID     string
	RootCause      string
	Evidence       []string
	Confidence     float64
	RiskLevel      RiskLevel
	Remediation    string
	RollbackPlan   string
	LLMModel       string
	PromptTokens   int
	ResponseTokens int
	CreatedAt      time.Time
}

// RiskLevel represents the assessed risk of the incident.
type RiskLevel string

const (
	RiskCritical RiskLevel = "critical"
	RiskHigh     RiskLevel = "high"
	RiskMedium   RiskLevel = "medium"
	RiskLow      RiskLevel = "low"
)

// New creates a new RCA Report with validated fields.
func New(incidentID, rootCause string, evidence []string, confidence float64, risk RiskLevel, remediation, rollbackPlan string) (*Report, error) {
	if incidentID == "" {
		return nil, errors.NewValidation("incident_id", "must not be empty")
	}
	if rootCause == "" {
		return nil, errors.NewValidation("root_cause", "must not be empty")
	}
	if confidence < 0 || confidence > 1 {
		return nil, errors.NewValidation("confidence", "must be between 0 and 1")
	}
	if remediation == "" {
		return nil, errors.NewValidation("remediation", "must not be empty")
	}

	return &Report{
		IncidentID:   incidentID,
		RootCause:    rootCause,
		Evidence:     evidence,
		Confidence:   confidence,
		RiskLevel:    risk,
		Remediation:  remediation,
		RollbackPlan: rollbackPlan,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

// IsHighConfidence returns true if the report confidence meets the threshold for auto-remediation.
func (r *Report) IsHighConfidence(threshold float64) bool {
	return r.Confidence >= threshold
}
