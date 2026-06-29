package agent

import (
	"time"
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
