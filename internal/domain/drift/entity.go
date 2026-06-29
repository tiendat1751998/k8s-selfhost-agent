// Package drift provides domain entities for GitOps drift detection.
package drift

import "time"

// DriftStatus represents whether a resource is in sync.
type DriftStatus string

const (
	InSync  DriftStatus = "in_sync"
	Drifted DriftStatus = "drifted"
	Unknown DriftStatus = "unknown"
)

// DriftRecord represents a drift detection result.
type DriftRecord struct {
	ID            string      `json:"id"`
	Cluster       string      `json:"cluster"`
	Namespace     string      `json:"namespace"`
	Resource      string      `json:"resource"`
	ResourceKind  string      `json:"resource_kind"`
	ExpectedState string      `json:"expected_state"` // YAML/JSON from Git
	ActualState   string      `json:"actual_state"`   // YAML/JSON from cluster
	Diff          string      `json:"diff"`
	Status        DriftStatus `json:"status"`
	DetectedAt    time.Time   `json:"detected_at"`
}
