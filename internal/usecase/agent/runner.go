package agent

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/agent"
	"github.com/datdt/k8sselfhost/internal/pkg/concurrency"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

type Runner struct {
	repo         agent.Repository
	orchestrator *Orchestrator
	interval     time.Duration
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	running      bool
	mu           sync.Mutex
}

func NewRunner(repo agent.Repository, orchestrator *Orchestrator, interval time.Duration) *Runner {
	return &Runner{
		repo:         repo,
		orchestrator: orchestrator,
		interval:     interval,
	}
}

// Start begins polling the DB for pending tasks.
func (r *Runner) Start(ctx context.Context) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	ctx, r.cancel = context.WithCancel(ctx)
	r.wg.Add(1)
	r.mu.Unlock()

	concurrency.Go(logger.Get(), func() {
		defer r.wg.Done()
		log := logger.WithContext(ctx)
		log.Info("agent runner polling loop started", zap.Duration("interval", r.interval))

		timer := time.NewTimer(r.interval)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info("agent runner polling loop stopped")
				return
			case <-timer.C:
				r.processNextTask(ctx)
				timer.Reset(r.interval)
			}
		}
	})
}

// Stop stops the runner.
func (r *Runner) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	if r.cancel != nil {
		r.cancel()
	}
	r.running = false
	r.mu.Unlock() // Unlock BEFORE waiting to avoid deadlock
	r.wg.Wait()
}

func (r *Runner) processNextTask(ctx context.Context) {
	log := logger.WithContext(ctx)

	// Fetch all tasks
	tasks, err := r.repo.ListTasks(ctx)
	if err != nil {
		log.Error("failed to retrieve task list", zap.Error(err))
		return
	}

	// Find the oldest pending task
	var nextTask *agent.Task
	for i := range tasks {
		if tasks[i].Status == agent.TaskPending {
			nextTask = &tasks[i]
			break
		}
	}

	if nextTask == nil {
		return
	}

	// Verify dependencies are met
	for _, depID := range nextTask.Dependencies {
		dep, err := r.repo.GetTask(ctx, depID)
		if err != nil {
			log.Warn("skipping task, dependency metadata missing", zap.String("task_id", nextTask.ID), zap.String("dep_id", depID))
			return
		}
		if dep.Status != agent.TaskSuccess {
			// Dependency not met, block task
			if nextTask.Status != agent.TaskBlocked {
				nextTask.Block()
				if err := r.repo.UpdateTask(ctx, nextTask); err != nil {
					log.Error("failed to update task status to blocked", zap.Error(err))
				}
				log.Info("task blocked by dependency", zap.String("task_id", nextTask.ID), zap.String("dep_id", depID))
			}
			return
		}
	}

	// Execute task using Orchestrator
	err = r.orchestrator.ExecuteTask(ctx, nextTask)
	if err != nil {
		log.Error("error during task pipeline execution", zap.String("task_id", nextTask.ID), zap.Error(err))
	}
}
