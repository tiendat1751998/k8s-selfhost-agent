package agent

import (
	"time"
)

// ProjectState holds overall metrics and current execution phase/progress.
type ProjectState struct {
	ID                string    `json:"id"` // Defaults to "latest"
	CurrentPhase      string    `json:"current_phase"`
	CurrentModule     string    `json:"current_module"`
	CurrentFeature    string    `json:"current_feature"`
	CurrentTaskID     string    `json:"current_task_id,omitempty"`
	CurrentSubtaskID  string    `json:"current_subtask_id,omitempty"`
	RepositoryHealth  float64   `json:"repository_health"`
	TechnicalDebt     float64   `json:"technical_debt"`
	ArchitectureScore float64   `json:"architecture_score"`
	QualityScore      float64   `json:"quality_score"`
	UpdatedAt         time.Time `json:"updated_at"`
}
