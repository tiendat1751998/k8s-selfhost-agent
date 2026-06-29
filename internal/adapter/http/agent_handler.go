package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/datdt/k8sselfhost/internal/domain/agent"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/agent"
)

// AgentHandler provides HTTP handlers for the Multi-Agent Framework API.
type AgentHandler struct {
	repo         agent.Repository
	orchestrator *usecase.Orchestrator
}

// NewAgentHandler creates a new agent HTTP handler.
func NewAgentHandler(repo agent.Repository, orchestrator *usecase.Orchestrator) *AgentHandler {
	return &AgentHandler{
		repo:         repo,
		orchestrator: orchestrator,
	}
}

// RegisterRoutes registers the agent framework endpoints.
func (h *AgentHandler) RegisterRoutes(r chi.Router) {
	r.Get("/state", h.GetState)
	r.Get("/tasks", h.ListTasks)
	r.Post("/tasks", h.CreateTask)
	r.Get("/runs", h.ListRuns)
}

// GetState handles GET /api/v1/agents/state
func (h *AgentHandler) GetState(w http.ResponseWriter, r *http.Request) {
	state, err := h.repo.GetProjectState(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get project state", err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// ListTasks handles GET /api/v1/agents/tasks
func (h *AgentHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.repo.ListTasks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agent tasks", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": tasks})
}

type createTaskRequest struct {
	Phase        string   `json:"phase"`
	Module       string   `json:"module"`
	Feature      string   `json:"feature"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Dependencies []string `json:"dependencies"`
}

// CreateTask handles POST /api/v1/agents/tasks
func (h *AgentHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	task := &agent.Task{
		ID:           uuid.New().String(),
		Phase:        req.Phase,
		Module:       req.Module,
		Feature:      req.Feature,
		Title:        req.Title,
		Description:  req.Description,
		Status:       agent.TaskPending,
		Dependencies: req.Dependencies,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := h.repo.CreateTask(r.Context(), task); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create task", err)
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

// ListRuns handles GET /api/v1/agents/runs
func (h *AgentHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task_id")
	runs, err := h.repo.ListExecutions(r.Context(), taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agent runs", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": runs})
}
