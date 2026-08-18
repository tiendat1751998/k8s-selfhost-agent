package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/agent"
	"github.com/datdt/k8sselfhost/internal/domain/ports"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/pkg/concurrency"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

type WSBroadcaster interface {
	Broadcast(msgType string, data interface{})
}

type Orchestrator struct {
	repo      agent.Repository
	llmClient ports.LLMClient
	wsHub     WSBroadcaster
	txManager postgres.TransactionManager
}

func NewOrchestrator(repo agent.Repository, llmClient ports.LLMClient, wsHub WSBroadcaster, txManager postgres.TransactionManager) *Orchestrator {
	return &Orchestrator{
		repo:      repo,
		llmClient: llmClient,
		wsHub:     wsHub,
		txManager: txManager,
	}
}

// CreateAndScheduleTask validates the task, persists it via repository, and triggers execution.
func (o *Orchestrator) CreateAndScheduleTask(ctx context.Context, task *agent.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	if task.Title == "" {
		return fmt.Errorf("task title is required")
	}
	if task.Phase == "" {
		return fmt.Errorf("task phase is required")
	}
	if task.Module == "" {
		return fmt.Errorf("task module is required")
	}

	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	if task.Status == "" {
		task.Status = agent.TaskPending
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}

	if err := o.repo.CreateTask(ctx, task); err != nil {
		return fmt.Errorf("persisting task: %w", err)
	}

	if o.wsHub != nil {
		o.wsHub.Broadcast("log", fmt.Sprintf("agent: task '%s' created (id: %s)", task.Title, task.ID))
	}

	// Verify dependencies before immediate execution
	hasUnmetDeps := false
	for _, depID := range task.Dependencies {
		dep, err := o.repo.GetTask(ctx, depID)
		if err != nil || dep.Status != agent.TaskSuccess {
			hasUnmetDeps = true
			break
		}
	}

	if hasUnmetDeps {
		task.Block()
		if err := o.repo.UpdateTask(ctx, task); err != nil {
			logger.Get().Warn("failed to update task status to blocked", zap.String("task_id", task.ID), zap.Error(err))
		}
		return nil
	}

	// Trigger execution asynchronously
	concurrency.Go(logger.Get(), func() {
		execCtx := context.Background()
		if err := o.ExecuteTask(execCtx, task); err != nil {
			logger.Get().Error("failed to execute scheduled agent task", zap.String("task_id", task.ID), zap.Error(err))
		}
	})

	return nil
}

// ExecuteTask executes the full 10-step agent pipeline for a task.
func (o *Orchestrator) ExecuteTask(ctx context.Context, task *agent.Task) error {
	log := logger.WithContext(ctx).With(zap.String("task_id", task.ID))
	log.Info("starting multi-agent execution pipeline for task", zap.String("title", task.Title))

	// Update task status in DB & global project state atomically
	task.Start()

	startTx := func(txCtx context.Context) error {
		if err := o.repo.UpdateTask(txCtx, task); err != nil {
			return err
		}

		state, err := o.repo.GetProjectState(txCtx)
		if err == nil && state != nil {
			state.SetCurrentTask(task)
			if err := o.repo.UpdateProjectState(txCtx, state); err != nil {
				log.Error("failed to update project state", zap.Error(err))
				return err
			}
		}
		return nil
	}

	if o.txManager != nil {
		if err := o.txManager.RunInTx(ctx, startTx); err != nil {
			return err
		}
	} else {
		if err := startTx(ctx); err != nil {
			return err
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
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("task cancelled: %w", err)
		}

		start := time.Now()
		o.broadcastAgentStep(step.stepName, "running", cumulativeDuration)

		log.Info("running agent step", zap.String("step", step.stepName), zap.String("role", string(step.role)))

		// Create execution run record
		exec := agent.NewExecution(task.ID, step.role, fmt.Sprintf("Running %s step context", step.stepName))
		if err := o.repo.RecordExecution(ctx, exec); err != nil {
			log.Error("failed to record execution start", zap.Error(err))
		}

		// Execute specialized agent logic
		activeAgent := NewAgent(step.role, o.llmClient)
		output, execErr := activeAgent.Execute(ctx, task, fmt.Sprintf("Previous step execution results (cumulative duration: %dms)", cumulativeDuration))

		duration := time.Since(start).Milliseconds()
		cumulativeDuration += duration

		if execErr != nil {
			exec.Fail(execErr.Error())
		} else {
			exec.Complete(output)
		}
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

	if finalErr != nil {
		task.Fail()
		if err := o.repo.UpdateTask(ctx, task); err != nil {
			log.Error("failed to update task status to failed", zap.String("task_id", task.ID), zap.Error(err))
		}
		log.Error("multi-agent execution pipeline failed", zap.Error(finalErr))
		return finalErr
	}

	task.Complete()

	completionTx := func(txCtx context.Context) error {
		if err := o.repo.UpdateTask(txCtx, task); err != nil {
			log.Error("failed to update task status to success", zap.String("task_id", task.ID), zap.Error(err))
			return err
		}

		// Adjust metrics upon task success
		if state, err := o.repo.GetProjectState(txCtx); err == nil && state != nil {
			state.ClearCurrentTask()

			// Calculate dynamic quality metrics
			var totalTasks, successTasks int
			if tasks, err := o.repo.ListTasks(txCtx); err == nil {
				totalTasks = len(tasks)
				for _, t := range tasks {
					if t.Status == agent.TaskSuccess {
						successTasks++
					}
				}
			}

			// Calculate execution success rate
			var totalExecs, successExecs int
			if tasks, err := o.repo.ListTasks(txCtx); err == nil {
				for _, t := range tasks {
					if execs, err := o.repo.ListExecutions(txCtx, t.ID); err == nil {
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
			if tasks, err := o.repo.ListTasks(txCtx); err == nil {
				for _, t := range tasks {
					if execs, err := o.repo.ListExecutions(txCtx, t.ID); err == nil {
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

			state.UpdateMetrics(health, techDebt, archScore, quality)
			if err := o.repo.UpdateProjectState(txCtx, state); err != nil {
				log.Error("failed to update project state metrics", zap.Error(err))
				return err
			}
		}
		return nil
	}

	if o.txManager != nil {
		if err := o.txManager.RunInTx(ctx, completionTx); err != nil {
			return err
		}
	} else {
		if err := completionTx(ctx); err != nil {
			return err
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
