package backup

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type WorkerPool struct {
	engine   *Engine
	logger   *zap.Logger
	jobQueue chan string
	resQueue chan string
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewWorkerPool(engine *Engine, logger *zap.Logger, queueSize int) *WorkerPool {
	if queueSize <= 0 {
		queueSize = 50
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		engine:   engine,
		logger:   logger,
		jobQueue: make(chan string, queueSize),
		resQueue: make(chan string, queueSize),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (w *WorkerPool) Start(workersCount int) {
	if workersCount <= 0 {
		workersCount = 3
	}

	if err := w.engine.ReconcileStaleJobs(w.ctx); err != nil {
		w.logger.Error("failed to reconcile stale backup jobs on worker pool start", zap.Error(err))
	}

	for i := 0; i < workersCount; i++ {
		w.wg.Add(1)
		go w.workerLoop(i)
	}
}

func (w *WorkerPool) workerLoop(id int) {
	defer w.wg.Done()
	w.logger.Info("starting backup worker", zap.Int("worker_id", id))

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("stopping backup worker", zap.Int("worker_id", id))
			return
		case jobID := <-w.jobQueue:
			ctx, cancel := context.WithTimeout(w.ctx, 2*time.Hour)
			w.logger.Info("executing backup job", zap.String("job_id", jobID), zap.Int("worker_id", id))
			if err := w.engine.ExecuteBackup(ctx, jobID); err != nil {
				w.logger.Error("backup job failed", zap.String("job_id", jobID), zap.Error(err))
			} else {
				w.logger.Info("backup job completed successfully", zap.String("job_id", jobID))
			}
			cancel()
		case restoreID := <-w.resQueue:
			ctx, cancel := context.WithTimeout(w.ctx, 2*time.Hour)
			w.logger.Info("executing restore job", zap.String("restore_id", restoreID), zap.Int("worker_id", id))
			if err := w.engine.ExecuteRestore(ctx, restoreID); err != nil {
				w.logger.Error("restore job failed", zap.String("restore_id", restoreID), zap.Error(err))
			} else {
				w.logger.Info("restore job completed successfully", zap.String("restore_id", restoreID))
			}
			cancel()
		}
	}
}

func (w *WorkerPool) EnqueueBackup(jobID string) bool {
	select {
	case <-w.ctx.Done():
		return false
	default:
	}
	select {
	case w.jobQueue <- jobID:
		return true
	default:
		return false
	}
}

func (w *WorkerPool) EnqueueRestore(restoreID string) bool {
	select {
	case <-w.ctx.Done():
		return false
	default:
	}
	select {
	case w.resQueue <- restoreID:
		return true
	default:
		return false
	}
}

func (w *WorkerPool) Stop() {
	w.cancel()
	w.wg.Wait()
}
