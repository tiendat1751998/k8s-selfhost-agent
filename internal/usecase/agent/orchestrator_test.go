package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/agent"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
)

type mockAgentRepository struct {
	tasks      map[string]*agent.Task
	subtasks   map[string]*agent.Subtask
	executions map[string]*agent.Execution
	state      *agent.ProjectState
}

func newMockAgentRepository() *mockAgentRepository {
	return &mockAgentRepository{
		tasks:      make(map[string]*agent.Task),
		subtasks:   make(map[string]*agent.Subtask),
		executions: make(map[string]*agent.Execution),
		state: &agent.ProjectState{
			ID:                "latest",
			CurrentPhase:      "Phase 11",
			CurrentModule:     "Multi-Agent",
			CurrentFeature:    "Test",
			RepositoryHealth:  100.0,
			TechnicalDebt:     0.0,
			ArchitectureScore: 100.0,
			QualityScore:      100.0,
			UpdatedAt:         time.Now().UTC(),
		},
	}
}

func (m *mockAgentRepository) CreateTask(ctx context.Context, task *agent.Task) error {
	m.tasks[task.ID] = task
	return nil
}

func (m *mockAgentRepository) GetTask(ctx context.Context, id string) (*agent.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}

func (m *mockAgentRepository) ListTasks(ctx context.Context) ([]agent.Task, error) {
	var list []agent.Task
	for _, t := range m.tasks {
		list = append(list, *t)
	}
	return list, nil
}

func (m *mockAgentRepository) UpdateTask(ctx context.Context, task *agent.Task) error {
	m.tasks[task.ID] = task
	return nil
}

func (m *mockAgentRepository) CreateSubtask(ctx context.Context, sub *agent.Subtask) error {
	m.subtasks[sub.ID] = sub
	return nil
}

func (m *mockAgentRepository) UpdateSubtask(ctx context.Context, sub *agent.Subtask) error {
	m.subtasks[sub.ID] = sub
	return nil
}

func (m *mockAgentRepository) RecordExecution(ctx context.Context, exec *agent.Execution) error {
	m.executions[exec.ID] = exec
	return nil
}

func (m *mockAgentRepository) GetExecution(ctx context.Context, id string) (*agent.Execution, error) {
	e, ok := m.executions[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return e, nil
}

func (m *mockAgentRepository) ListExecutions(ctx context.Context, taskID string) ([]agent.Execution, error) {
	var list []agent.Execution
	for _, e := range m.executions {
		if taskID == "" || e.TaskID == taskID {
			list = append(list, *e)
		}
	}
	return list, nil
}

func (m *mockAgentRepository) UpdateExecution(ctx context.Context, exec *agent.Execution) error {
	m.executions[exec.ID] = exec
	return nil
}

func (m *mockAgentRepository) GetProjectState(ctx context.Context) (*agent.ProjectState, error) {
	return m.state, nil
}

func (m *mockAgentRepository) UpdateProjectState(ctx context.Context, state *agent.ProjectState) error {
	m.state = state
	return nil
}

type mockWSBroadcaster struct {
	events []string
}

func (m *mockWSBroadcaster) Broadcast(msgType string, data interface{}) {
	m.events = append(m.events, msgType)
}

type mockLLM struct{}

func (m *mockLLM) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return &llm.CompletionResponse{
		Content: "Mock Completed output",
	}, nil
}

func (m *mockLLM) HealthCheck(ctx context.Context) error {
	return nil
}

type mockLLMWithError struct {
	err error
}

func (m *mockLLMWithError) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return nil, m.err
}

func (m *mockLLMWithError) HealthCheck(ctx context.Context) error {
	return nil
}

type mockAgentRepositoryWithErrors struct {
	*mockAgentRepository
	failUpdateTask bool
}

func (m *mockAgentRepositoryWithErrors) UpdateTask(ctx context.Context, task *agent.Task) error {
	if m.failUpdateTask {
		return errors.New("mock repository update error")
	}
	return m.mockAgentRepository.UpdateTask(ctx, task)
}

func TestOrchestratorPipeline(t *testing.T) {
	repo := newMockAgentRepository()
	ws := &mockWSBroadcaster{}
	mockL := &mockLLM{}

	orch := NewOrchestrator(repo, mockL, ws)

	task := &agent.Task{
		ID:          "task-1",
		Phase:       "Phase 11",
		Module:      "agents",
		Feature:     "Integration",
		Title:       "Integration Test Task",
		Description: "Test agent orchestrator pipeline end-to-end",
		Status:      agent.TaskPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	_ = repo.CreateTask(context.Background(), task)

	// Execute task pipeline (mocking LLM calls)
	err := orch.ExecuteTask(context.Background(), task)
	if err != nil {
		t.Errorf("Orchestration pipeline execution failed: %v", err)
	}

	updatedTask, _ := repo.GetTask(context.Background(), task.ID)
	if updatedTask.Status != agent.TaskSuccess {
		t.Errorf("Expected task status 'success', got %s", updatedTask.Status)
	}

	if len(ws.events) == 0 {
		t.Errorf("Expected websocket broadcast events to be emitted")
	}
}

func TestOrchestratorPipeline_StepFailure(t *testing.T) {
	repo := newMockAgentRepository()
	ws := &mockWSBroadcaster{}
	mockL := &mockLLMWithError{err: errors.New("LLM completed with error")}

	orch := NewOrchestrator(repo, mockL, ws)

	task := &agent.Task{
		ID:          "task-fail",
		Phase:       "Phase 11",
		Module:      "agents",
		Feature:     "Integration",
		Title:       "Failure Test Task",
		Description: "Test agent orchestrator pipeline step failure handling",
		Status:      agent.TaskPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	_ = repo.CreateTask(context.Background(), task)

	err := orch.ExecuteTask(context.Background(), task)
	if err == nil {
		t.Errorf("Expected pipeline execution to fail due to step failure, but got nil")
	}

	updatedTask, _ := repo.GetTask(context.Background(), task.ID)
	if updatedTask.Status != agent.TaskFailed {
		t.Errorf("Expected task status 'failed', got %s", updatedTask.Status)
	}

	// Verify that at least one "agent" or "log" event of failure was broadcast
	hasFailedEvent := false
	for _, e := range ws.events {
		if e == "agent" || e == "log" {
			hasFailedEvent = true
		}
	}
	if !hasFailedEvent {
		t.Errorf("Expected websocket failure events to be broadcast")
	}
}

func TestOrchestratorPipeline_DbUpdateFailure(t *testing.T) {
	innerRepo := newMockAgentRepository()
	repo := &mockAgentRepositoryWithErrors{
		mockAgentRepository: innerRepo,
		failUpdateTask:      true,
	}
	ws := &mockWSBroadcaster{}
	mockL := &mockLLM{}

	orch := NewOrchestrator(repo, mockL, ws)

	task := &agent.Task{
		ID:          "task-db-fail",
		Phase:       "Phase 11",
		Module:      "agents",
		Feature:     "Integration",
		Title:       "DB Fail Test Task",
		Description: "Test agent orchestrator database update failure",
		Status:      agent.TaskPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	_ = innerRepo.CreateTask(context.Background(), task)

	err := orch.ExecuteTask(context.Background(), task)
	if err == nil {
		t.Errorf("Expected pipeline execution to fail due to DB update failure, but got nil")
	}
}
