package agent

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AgentType represents a specialized agent role.
type AgentType string

const (
	Orchestrator          AgentType = "Orchestrator"
	Planner               AgentType = "Planner"
	Architect             AgentType = "Architect"
	RepositoryAnalyzer    AgentType = "Repository Analyzer"
	BackendEngineer       AgentType = "Backend Engineer"
	FrontendEngineer      AgentType = "Frontend Engineer"
	KubernetesEngineer    AgentType = "Kubernetes Engineer"
	GitOpsEngineer        AgentType = "GitOps Engineer"
	SecurityEngineer      AgentType = "Security Engineer"
	PerformanceEngineer   AgentType = "Performance Engineer"
	QAEngineer            AgentType = "QA Engineer"
	CodeReviewer          AgentType = "Code Reviewer"
	RefactoringEngineer   AgentType = "Refactoring Engineer"
	DocumentationEngineer AgentType = "Documentation Engineer"
	ReleaseManager        AgentType = "Release Manager"
)

const (
	ExecutionRunning = "running"
	ExecutionSuccess = "success"
	ExecutionFailed  = "failed"
)

// Execution represents a specific execution run of an agent.
type Execution struct {
	ID          string     `json:"id"`
	TaskID      string     `json:"task_id"`
	AgentType   AgentType  `json:"agent_type"`
	Status      string     `json:"status"` // running | success | failed
	Input       string     `json:"input,omitempty"`
	Output      string     `json:"output,omitempty"`
	ErrorDetail string     `json:"error_detail,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// NewExecution creates a new Execution entity with initial running state.
func NewExecution(taskID string, agentType AgentType, input string) *Execution {
	return &Execution{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		AgentType: agentType,
		Status:    ExecutionRunning,
		Input:     input,
		CreatedAt: time.Now().UTC(),
	}
}

// Complete marks the execution as successfully completed with output.
func (e *Execution) Complete(output string) {
	now := time.Now().UTC()
	e.Status = ExecutionSuccess
	e.Output = output
	e.CompletedAt = &now
}

// Fail marks the execution as failed with the given error detail.
func (e *Execution) Fail(errMsg string) {
	now := time.Now().UTC()
	e.Status = ExecutionFailed
	e.ErrorDetail = errMsg
	e.CompletedAt = &now
}

// Agent defines the generic execution contract for a specialized agent.
type Agent interface {
	Type() AgentType
	Execute(ctx context.Context, task *Task, input string) (string, error)
}

// Repository defines the persistence interface for agent tasks, executions, and project states.
type Repository interface {
	// Tasks
	CreateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, id string) (*Task, error)
	ListTasks(ctx context.Context) ([]Task, error)
	UpdateTask(ctx context.Context, task *Task) error

	// Subtasks
	CreateSubtask(ctx context.Context, sub *Subtask) error
	UpdateSubtask(ctx context.Context, sub *Subtask) error

	// Executions
	RecordExecution(ctx context.Context, exec *Execution) error
	GetExecution(ctx context.Context, id string) (*Execution, error)
	ListExecutions(ctx context.Context, taskID string) ([]Execution, error)
	UpdateExecution(ctx context.Context, exec *Execution) error

	// Project State
	GetProjectState(ctx context.Context) (*ProjectState, error)
	UpdateProjectState(ctx context.Context, state *ProjectState) error
}
