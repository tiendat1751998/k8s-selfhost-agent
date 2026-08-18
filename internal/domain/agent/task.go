package agent

import (
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "inprogress"
	TaskSuccess    TaskStatus = "success"
	TaskBlocked    TaskStatus = "blocked"
	TaskFailed     TaskStatus = "failed"
)

type SubtaskStatus string

const (
	SubtaskPending    SubtaskStatus = "pending"
	SubtaskInProgress SubtaskStatus = "inprogress"
	SubtaskSuccess    SubtaskStatus = "success"
	SubtaskFailed     SubtaskStatus = "failed"
)

// Task represents an engineering/development task.
type Task struct {
	ID           string       `json:"id"`
	Phase        string       `json:"phase"`
	Module       string       `json:"module"`
	Feature      string       `json:"feature"`
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	Status       TaskStatus   `json:"status"`
	Dependencies []string     `json:"dependencies"` // Parent task IDs
	Subtasks     []Subtask    `json:"subtasks,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
}

// NewTask creates a new Task entity with initial pending state and UTC timestamps.
func NewTask(id, phase, module, feature, title, description string, dependencies []string) *Task {
	if id == "" {
		id = uuid.New().String()
	}
	if dependencies == nil {
		dependencies = []string{}
	}
	now := time.Now().UTC()
	return &Task{
		ID:           id,
		Phase:        phase,
		Module:       module,
		Feature:      feature,
		Title:        title,
		Description:  description,
		Status:       TaskPending,
		Dependencies: dependencies,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// Start transitions the task status to inprogress.
func (t *Task) Start() {
	t.Status = TaskInProgress
	t.UpdatedAt = time.Now().UTC()
}

// Complete transitions the task status to success and records completion timestamp.
func (t *Task) Complete() {
	now := time.Now().UTC()
	t.Status = TaskSuccess
	t.CompletedAt = &now
	t.UpdatedAt = now
}

// Fail transitions the task status to failed.
func (t *Task) Fail() {
	t.Status = TaskFailed
	t.UpdatedAt = time.Now().UTC()
}

// Block transitions the task status to blocked by unresolved dependencies.
func (t *Task) Block() {
	t.Status = TaskBlocked
	t.UpdatedAt = time.Now().UTC()
}

// Subtask represents a small unit of a task.
type Subtask struct {
	ID          string        `json:"id"`
	TaskID      string        `json:"task_id"`
	Title       string        `json:"title"`
	Status      SubtaskStatus `json:"status"`
	Complexity  int           `json:"complexity"` // e.g. 1 to 5
	ExecOrder   int           `json:"exec_order"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
}

// NewSubtask creates a new Subtask entity with initial pending state.
func NewSubtask(id, taskID, title string, complexity, execOrder int) *Subtask {
	if id == "" {
		id = uuid.New().String()
	}
	return &Subtask{
		ID:         id,
		TaskID:     taskID,
		Title:      title,
		Status:     SubtaskPending,
		Complexity: complexity,
		ExecOrder:  execOrder,
	}
}

// Start transitions the subtask status to inprogress.
func (s *Subtask) Start() {
	s.Status = SubtaskInProgress
}

// Complete transitions the subtask status to success and records completion timestamp.
func (s *Subtask) Complete() {
	now := time.Now().UTC()
	s.Status = SubtaskSuccess
	s.CompletedAt = &now
}

// Fail transitions the subtask status to failed.
func (s *Subtask) Fail() {
	s.Status = SubtaskFailed
}
