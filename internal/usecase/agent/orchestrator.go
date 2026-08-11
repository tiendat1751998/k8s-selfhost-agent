package agent

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/agent"
	"github.com/datdt/k8sselfhost/internal/domain/ports"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

type WSBroadcaster interface {
	Broadcast(msgType string, data interface{})
}

type Orchestrator struct {
	repo      agent.Repository
	llmClient ports.LLMClient
	wsHub     WSBroadcaster
}

func NewOrchestrator(repo agent.Repository, llmClient ports.LLMClient, wsHub WSBroadcaster) *Orchestrator {
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
		if err := o.repo.UpdateProjectState(ctx, state); err != nil {
			log.Error("failed to update project state", zap.Error(err))
		}
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
		if err := o.repo.UpdateTask(ctx, task); err != nil {
			log.Error("failed to update task status to failed", zap.String("task_id", task.ID), zap.Error(err))
		}
		log.Error("multi-agent execution pipeline failed", zap.Error(finalErr))
		return finalErr
	}

	now := time.Now().UTC()
	task.Status = agent.TaskSuccess
	task.CompletedAt = &now
	if err := o.repo.UpdateTask(ctx, task); err != nil {
		log.Error("failed to update task status to success", zap.String("task_id", task.ID), zap.Error(err))
	}

		// Adjust metrics upon task success
	if state, err := o.repo.GetProjectState(ctx); err == nil {
		state.CurrentTaskID = ""
		state.CurrentSubtaskID = ""

		// Calculate dynamic quality metrics
		var totalTasks, successTasks int
		if tasks, err := o.repo.ListTasks(ctx); err == nil {
			totalTasks = len(tasks)
			for _, t := range tasks {
				if t.Status == agent.TaskSuccess {
					successTasks++
				}
			}
		}

		// Calculate execution success rate
		var totalExecs, successExecs int
		if tasks, err := o.repo.ListTasks(ctx); err == nil {
			for _, t := range tasks {
				if execs, err := o.repo.ListExecutions(ctx, t.ID); err == nil {
					totalExecs += len(execs)
					for _, e := range execs {
						if e.Status == "success" {
							successExecs++
						}
					}
				}
			}
		}

		health := 100.0
		if totalTasks > 0 {
			health = (float64(successTasks) / float64(totalTasks)) * 100.0
		}

		quality := 100.0
		if totalExecs > 0 {
			quality = (float64(successExecs) / float64(totalExecs)) * 100.0
		}

		// Calculate architecture score based on Architect agent execution success
		var archTotal, archSuccess int
		if tasks, err := o.repo.ListTasks(ctx); err == nil {
			for _, t := range tasks {
				if execs, err := o.repo.ListExecutions(ctx, t.ID); err == nil {
					for _, e := range execs {
						if e.AgentType == agent.Architect {
							archTotal++
							if e.Status == "success" {
								archSuccess++
							}
						}
					}
				}
			}
		}
		archScore := 100.0
		if archTotal > 0 {
			archScore = (float64(archSuccess) / float64(archTotal)) * 100.0
		}

		// Technical debt can be calculated as the inverse of health/quality
		techDebt := 100.0 - health
		if techDebt < 0 {
			techDebt = 0.0
		}

		state.RepositoryHealth = health
		state.TechnicalDebt = techDebt
		state.ArchitectureScore = archScore
		state.QualityScore = quality
		state.UpdatedAt = time.Now().UTC()
		if err := o.repo.UpdateProjectState(ctx, state); err != nil {
			log.Error("failed to update project state metrics", zap.Error(err))
		}
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
