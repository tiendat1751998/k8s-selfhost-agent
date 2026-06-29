package agent

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/agent"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	"github.com/datdt/k8sselfhost/pkg/logger"
)

type WSBroadcaster interface {
	Broadcast(msgType string, data interface{})
}

type Orchestrator struct {
	repo      agent.Repository
	llmClient llm.Client
	wsHub     WSBroadcaster
}

func NewOrchestrator(repo agent.Repository, llmClient llm.Client, wsHub WSBroadcaster) *Orchestrator {
	return &Orchestrator{
		repo:      repo,
		llmClient: llmClient,
		wsHub:     wsHub,
	}
}

// ExecuteTask executes the full 10-step agent pipeline for a task.
func (o *Orchestrator) ExecuteTask(ctx context.Context, task *agent.Task) error {
	log := logger.WithContext(ctx).With(zap.String("task_id", task.ID))
	log.Info("starting multi-agent execution pipeline for task", zap.String("title", task.Title))

	// Update task status in DB
	task.Status = agent.TaskInProgress
	task.UpdatedAt = time.Now().UTC()
	if err := o.repo.UpdateTask(ctx, task); err != nil {
		return err
	}

	// Update global project state
	state, err := o.repo.GetProjectState(ctx)
	if err == nil {
		state.CurrentPhase = task.Phase
		state.CurrentModule = task.Module
		state.CurrentFeature = task.Feature
		state.CurrentTaskID = task.ID
		state.UpdatedAt = time.Now().UTC()
		_ = o.repo.UpdateProjectState(ctx, state)
	}

	// Step sequence
	steps := []struct {
		stepName string
		role     agent.AgentType
	}{
		{"TASK", agent.Planner}, // Planner acts first
		{"LLM", agent.Architect}, // Architect reviews architecture
		{"CODE", agent.RepositoryAnalyzer}, // Analyzer checks reusable modules
		{"TEST", agent.BackendEngineer}, // Backend implementations
		{"FIX", agent.QAEngineer}, // QA validates compilation and tests
		{"COMMIT", agent.CodeReviewer}, // Code Reviewer audits
		{"PR", agent.ReleaseManager}, // Release Manager deploys/finalizes
	}

	var cumulativeDuration int64 = 0
	var finalErr error

	// Run pipeline chronologically
	for _, step := range steps {
		start := time.Now()
		o.broadcastAgentStep(step.stepName, "running", cumulativeDuration)

		log.Info("running agent step", zap.String("step", step.stepName), zap.String("role", string(step.role)))

		// Create execution run record
		exec := NewExecution(task.ID, step.role, fmt.Sprintf("Running %s step context", step.stepName))
		if err := o.repo.RecordExecution(ctx, exec); err != nil {
			log.Error("failed to record execution start", zap.Error(err))
		}

		// Execute specialized agent logic
		activeAgent := NewAgent(step.role, o.llmClient)
		output, execErr := activeAgent.Execute(ctx, task, fmt.Sprintf("Previous step execution results (cumulative duration: %dms)", cumulativeDuration))

		duration := time.Since(start).Milliseconds()
		cumulativeDuration += duration

		CompleteExecution(exec, output, execErr)
		if err := o.repo.UpdateExecution(ctx, exec); err != nil {
			log.Error("failed to update execution result", zap.Error(err))
		}

		if execErr != nil {
			o.broadcastAgentStep(step.stepName, "failed", cumulativeDuration)
			finalErr = fmt.Errorf("step %s failed: %w", step.stepName, execErr)
			break
		}

		o.broadcastAgentStep(step.stepName, "success", cumulativeDuration)
		time.Sleep(1 * time.Second) // Small delay for UI smoothness
	}

	task.UpdatedAt = time.Now().UTC()
	if finalErr != nil {
		task.Status = agent.TaskFailed
		_ = o.repo.UpdateTask(ctx, task)
		log.Error("multi-agent execution pipeline failed", zap.Error(finalErr))
		return finalErr
	}

	now := time.Now().UTC()
	task.Status = agent.TaskSuccess
	task.CompletedAt = &now
	_ = o.repo.UpdateTask(ctx, task)

	// Adjust metrics upon task success
	if state, err := o.repo.GetProjectState(ctx); err == nil {
		state.CurrentTaskID = ""
		state.CurrentSubtaskID = ""
		state.RepositoryHealth = 100.0
		state.TechnicalDebt = 0.0
		state.ArchitectureScore = 100.0
		state.QualityScore = 100.0
		state.UpdatedAt = time.Now().UTC()
		_ = o.repo.UpdateProjectState(ctx, state)
	}

	log.Info("multi-agent execution pipeline completed successfully")
	return nil
}

// broadcastAgentStep broadcasts the step state to connected WebSocket clients.
func (o *Orchestrator) broadcastAgentStep(step string, status string, durationMs int64) {
	if o.wsHub != nil {
		o.wsHub.Broadcast("agent", map[string]interface{}{
			"step":     step,
			"status":   status,
			"duration": durationMs,
		})
		// Also send standard log messages
		o.wsHub.Broadcast("log", fmt.Sprintf("agent: step %s is %s (total run time: %dms)", step, status, durationMs))
	}
}
