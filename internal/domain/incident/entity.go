// Package incident defines the Incident aggregate root and its domain logic.
package incident

import (
	"fmt"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// Incident is the aggregate root for Kubernetes incidents.
type Incident struct {
	ID          string            `json:"id"`
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	ClusterID   string            `json:"cluster_id,omitempty"`
	ServiceName string            `json:"service_name,omitempty"`
	TenantID    string            `json:"tenant_id,omitempty"`
	ClusterName string            `json:"cluster_name"`
	Namespace   string            `json:"namespace"`
	PodName     string            `json:"pod_name"`
	Type        Type              `json:"type"`
	Status      Status            `json:"status"`
	Severity    Severity          `json:"severity"`
	Message     string            `json:"message"`
	RawData     map[string]string `json:"raw_data,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
}

// New creates a new Incident with validated fields.
func New(clusterName, namespace, podName string, incidentType Type, severity Severity, message string) (*Incident, error) {
	if clusterName == "" {
		return nil, errors.NewValidation("cluster_name", "must not be empty")
	}
	if namespace == "" {
		return nil, errors.NewValidation("namespace", "must not be empty")
	}
	if podName == "" {
		return nil, errors.NewValidation("pod_name", "must not be empty")
	}
	if !incidentType.IsValid() {
		return nil, errors.NewValidation("type", fmt.Sprintf("invalid incident type: %s", incidentType))
	}
	if !severity.IsValid() {
		return nil, errors.NewValidation("severity", fmt.Sprintf("invalid severity: %s", severity))
	}

	now := time.Now().UTC()
	return &Incident{
		ClusterName: clusterName,
		Namespace:   namespace,
		PodName:     podName,
		Type:        incidentType,
		Status:      StatusDetected,
		Severity:    severity,
		Message:     message,
		RawData:     make(map[string]string),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// MarkAnalyzing transitions the incident status to Analyzing.
func (i *Incident) MarkAnalyzing() error {
	if i.Status != StatusDetected {
		return errors.NewConflict("incident", fmt.Sprintf("cannot analyze incident in status %s", i.Status))
	}
	i.Status = StatusAnalyzing
	i.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkRemediating transitions the incident status to Remediating.
func (i *Incident) MarkRemediating() error {
	if i.Status != StatusAnalyzing {
		return errors.NewConflict("incident", fmt.Sprintf("cannot remediate incident in status %s", i.Status))
	}
	i.Status = StatusRemediating
	i.UpdatedAt = time.Now().UTC()
	return nil
}

// Acknowledge transitions the incident status to investigating and records operator details.
func (i *Incident) Acknowledge(operator, notes string) error {
	if i.Status == StatusResolved {
		return errors.NewConflict("incident", "cannot acknowledge already resolved incident")
	}
	i.Status = StatusInvestigating
	now := time.Now().UTC()
	i.UpdatedAt = now
	if i.RawData == nil {
		i.RawData = make(map[string]string)
	}
	if notes != "" {
		i.RawData["acknowledgement_notes"] = notes
	}
	if operator != "" {
		i.RawData["acknowledged_by"] = operator
	}
	i.RawData["acknowledged_at"] = now.Format(time.RFC3339)
	return nil
}

// Mitigate transitions the incident status to mitigating or mitigated and records operator notes.
func (i *Incident) Mitigate(operator, notes string, targetStatus Status) error {
	if i.Status == StatusResolved {
		return errors.NewConflict("incident", "cannot mitigate already resolved incident")
	}
	if targetStatus != StatusMitigated && targetStatus != StatusMitigating {
		targetStatus = StatusMitigating
	}
	i.Status = targetStatus
	now := time.Now().UTC()
	i.UpdatedAt = now
	if i.RawData == nil {
		i.RawData = make(map[string]string)
	}
	if notes != "" {
		i.RawData["mitigation_notes"] = notes
	}
	if operator != "" {
		i.RawData["mitigated_by"] = operator
	}
	i.RawData["mitigated_at"] = now.Format(time.RFC3339)
	return nil
}

// Resolve transitions the incident status to Resolved and records resolution details.
func (i *Incident) Resolve(operator, notes string) error {
	if i.Status == StatusResolved {
		return errors.NewConflict("incident", "incident is already resolved")
	}
	now := time.Now().UTC()
	i.Status = StatusResolved
	i.UpdatedAt = now
	i.ResolvedAt = &now
	if i.RawData == nil {
		i.RawData = make(map[string]string)
	}
	if notes != "" {
		i.RawData["resolution_notes"] = notes
	}
	if operator != "" {
		i.RawData["resolved_by"] = operator
	}
	i.RawData["resolved_at"] = now.Format(time.RFC3339)
	return nil
}

// MarkResolved transitions the incident status to Resolved.
func (i *Incident) MarkResolved() error {
	return i.Resolve("", "")
}

// MarkFailed transitions the incident status to Failed.
func (i *Incident) MarkFailed() error {
	i.Status = StatusFailed
	i.UpdatedAt = time.Now().UTC()
	return nil
}

// AddRawData attaches collected diagnostic data to the incident.
func (i *Incident) AddRawData(key, value string) {
	if i.RawData == nil {
		i.RawData = make(map[string]string)
	}
	i.RawData[key] = value
	i.UpdatedAt = time.Now().UTC()
}
