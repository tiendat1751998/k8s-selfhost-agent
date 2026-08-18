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

// SetCurrentTask sets the active task context on the project state.
func (s *ProjectState) SetCurrentTask(task *Task) {
	if task != nil {
		s.CurrentPhase = task.Phase
		s.CurrentModule = task.Module
		s.CurrentFeature = task.Feature
		s.CurrentTaskID = task.ID
	}
	s.UpdatedAt = time.Now().UTC()
}

// ClearCurrentTask clears the active task from the project state.
func (s *ProjectState) ClearCurrentTask() {
	s.CurrentTaskID = ""
	s.CurrentSubtaskID = ""
	s.UpdatedAt = time.Now().UTC()
}

// UpdateMetrics updates the project metrics.
func (s *ProjectState) UpdateMetrics(health, techDebt, archScore, quality float64) {
	s.RepositoryHealth = health
	s.TechnicalDebt = techDebt
	s.ArchitectureScore = archScore
	s.QualityScore = quality
	s.UpdatedAt = time.Now().UTC()
}
