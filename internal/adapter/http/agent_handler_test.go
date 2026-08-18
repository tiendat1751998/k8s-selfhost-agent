package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/agent"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/agent"
)

type mockAgentRepoForHTTP struct {
	tasks      map[string]*agent.Task
	subtasks   map[string]*agent.Subtask
	executions map[string]*agent.Execution
	state      *agent.ProjectState
	failState  bool
	failTasks  bool
	failExecs  bool
}

func newMockAgentRepoForHTTP() *mockAgentRepoForHTTP {
	return &mockAgentRepoForHTTP{
		tasks:      make(map[string]*agent.Task),
		subtasks:   make(map[string]*agent.Subtask),
		executions: make(map[string]*agent.Execution),
		state: &agent.ProjectState{
			ID:                "latest",
			CurrentPhase:      "Phase 11",
			CurrentModule:     "Multi-Agent",
			CurrentFeature:    "HTTP Test",
			RepositoryHealth:  98.5,
			TechnicalDebt:     1.5,
			ArchitectureScore: 95.0,
			QualityScore:      99.0,
			UpdatedAt:         time.Now().UTC(),
		},
	}
}

func (m *mockAgentRepoForHTTP) CreateTask(ctx context.Context, task *agent.Task) error {
	if m.failTasks {
		return errors.New("db error")
	}
	m.tasks[task.ID] = task
	return nil
}

func (m *mockAgentRepoForHTTP) GetTask(ctx context.Context, id string) (*agent.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return t, nil
}

func (m *mockAgentRepoForHTTP) ListTasks(ctx context.Context) ([]agent.Task, error) {
	if m.failTasks {
		return nil, errors.New("db error")
	}
	var list []agent.Task
	for _, t := range m.tasks {
		list = append(list, *t)
	}
	return list, nil
}

func (m *mockAgentRepoForHTTP) UpdateTask(ctx context.Context, task *agent.Task) error {
	m.tasks[task.ID] = task
	return nil
}

func (m *mockAgentRepoForHTTP) CreateSubtask(ctx context.Context, sub *agent.Subtask) error {
	m.subtasks[sub.ID] = sub
	return nil
}

func (m *mockAgentRepoForHTTP) UpdateSubtask(ctx context.Context, sub *agent.Subtask) error {
	m.subtasks[sub.ID] = sub
	return nil
}

func (m *mockAgentRepoForHTTP) RecordExecution(ctx context.Context, exec *agent.Execution) error {
	m.executions[exec.ID] = exec
	return nil
}

func (m *mockAgentRepoForHTTP) GetExecution(ctx context.Context, id string) (*agent.Execution, error) {
	e, ok := m.executions[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return e, nil
}

func (m *mockAgentRepoForHTTP) ListExecutions(ctx context.Context, taskID string) ([]agent.Execution, error) {
	if m.failExecs {
		return nil, errors.New("db error")
	}
	var list []agent.Execution
	for _, e := range m.executions {
		if taskID == "" || e.TaskID == taskID {
			list = append(list, *e)
		}
	}
	return list, nil
}

func (m *mockAgentRepoForHTTP) UpdateExecution(ctx context.Context, exec *agent.Execution) error {
	m.executions[exec.ID] = exec
	return nil
}

func (m *mockAgentRepoForHTTP) GetProjectState(ctx context.Context) (*agent.ProjectState, error) {
	if m.failState {
		return nil, errors.New("db error")
	}
	return m.state, nil
}

func (m *mockAgentRepoForHTTP) UpdateProjectState(ctx context.Context, state *agent.ProjectState) error {
	m.state = state
	return nil
}

type mockLLMClientForHTTP struct{}

func (m *mockLLMClientForHTTP) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return &llm.CompletionResponse{Content: "mock output"}, nil
}

func (m *mockLLMClientForHTTP) HealthCheck(ctx context.Context) error {
	return nil
}

func setupAgentHandler() (*AgentHandler, *mockAgentRepoForHTTP, *usecase.Orchestrator) {
	repo := newMockAgentRepoForHTTP()
	orch := usecase.NewOrchestrator(repo, &mockLLMClientForHTTP{}, nil, nil)
	handler := NewAgentHandler(repo, orch)
	return handler, repo, orch
}

func TestAgentHandler_GetState(t *testing.T) {
	handler, _, _ := setupAgentHandler()
	r := chi.NewRouter()
	r.Route("/agents", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/agents/state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var state agent.ProjectState
	if err := json.NewDecoder(w.Body).Decode(&state); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if state.CurrentModule != "Multi-Agent" {
		t.Errorf("expected module 'Multi-Agent', got %s", state.CurrentModule)
	}
}

func TestAgentHandler_GetState_Error(t *testing.T) {
	handler, repo, _ := setupAgentHandler()
	repo.failState = true
	r := chi.NewRouter()
	r.Route("/agents", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/agents/state", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestAgentHandler_ListTasks(t *testing.T) {
	handler, repo, _ := setupAgentHandler()
	_ = repo.CreateTask(context.Background(), &agent.Task{
		ID:        "t-1",
		Title:     "Task 1",
		Phase:     "Phase 1",
		Module:    "M1",
		Status:    agent.TaskPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	r := chi.NewRouter()
	r.Route("/agents", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/agents/tasks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Data []agent.Task `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 || resp.Data[0].Title != "Task 1" {
		t.Errorf("unexpected tasks response: %+v", resp.Data)
	}
}

func TestAgentHandler_CreateTask_Success(t *testing.T) {
	handler, repo, _ := setupAgentHandler()
	r := chi.NewRouter()
	r.Route("/agents", handler.RegisterRoutes)

	body := map[string]interface{}{
		"phase":       "Phase 11",
		"module":      "agent-system",
		"feature":     "orchestrator",
		"title":       "Implement Orchestrator HTTP wiring",
		"description": "Ensure handler uses usecase layer",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/agents/tasks", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
	}

	var created agent.Task
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode created task: %v", err)
	}

	if created.ID == "" {
		t.Errorf("expected generated task ID")
	}
	if created.Title != "Implement Orchestrator HTTP wiring" {
		t.Errorf("expected title 'Implement Orchestrator HTTP wiring', got %s", created.Title)
	}

	// Verify task in repo
	saved, err := repo.GetTask(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("task was not persisted in repo: %v", err)
	}
	if saved.Module != "agent-system" {
		t.Errorf("expected module 'agent-system', got %s", saved.Module)
	}
}

func TestAgentHandler_CreateTask_ValidationError(t *testing.T) {
	handler, _, _ := setupAgentHandler()
	r := chi.NewRouter()
	r.Route("/agents", handler.RegisterRoutes)

	// Missing title, phase, module
	body := map[string]interface{}{
		"description": "Empty fields",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/agents/tasks", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for validation failure, got %d", w.Code)
	}
}

func TestAgentHandler_ListRuns(t *testing.T) {
	handler, repo, _ := setupAgentHandler()
	_ = repo.RecordExecution(context.Background(), &agent.Execution{
		ID:        "exec-1",
		TaskID:    "t-1",
		AgentType: agent.Architect,
		Status:    "success",
		CreatedAt: time.Now().UTC(),
	})

	r := chi.NewRouter()
	r.Route("/agents", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/agents/runs?task_id=t-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Data []agent.Execution `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 1 || resp.Data[0].AgentType != agent.Architect {
		t.Errorf("unexpected executions response: %+v", resp.Data)
	}
}
