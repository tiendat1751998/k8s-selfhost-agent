// Package incident defines the Incident aggregate root and its domain logic.
package incident

import (
	"fmt"
	"time"

	"github.com/datdt/k8sselfhost/pkg/errors"
)

// Incident is the aggregate root for Kubernetes incidents.
type Incident struct {
	ID          string
	ClusterName string
	Namespace   string
	PodName     string
	Type        Type
	Status      Status
	Severity    Severity
	Message     string
	RawData     map[string]string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ResolvedAt  *time.Time
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

// MarkResolved transitions the incident status to Resolved.
func (i *Incident) MarkResolved() error {
	if i.Status == StatusResolved {
		return errors.NewConflict("incident", "incident is already resolved")
	}
	now := time.Now().UTC()
	i.Status = StatusResolved
	i.UpdatedAt = now
	i.ResolvedAt = &now
	return nil
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
