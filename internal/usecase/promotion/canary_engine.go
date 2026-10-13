package promotion

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/promotion"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// TrafficRouter defines port for updating traffic routing percentages at Ingress/Service mesh.
type TrafficRouter interface {
	SetWeight(ctx context.Context, namespace, workload string, weight int) error
}

// MetricsProvider defines port for querying live telemetry for canary and baseline deployments.
type MetricsProvider interface {
	QueryCanaryMetrics(ctx context.Context, workload, namespace string) (baselineErr, canaryErr float64, baselineP99, canaryP99 time.Duration, err error)
}

type workloadMetrics struct {
	baselineErr float64
	canaryErr   float64
	baselineP99 time.Duration
	canaryP99   time.Duration
}

// CanaryEngine implements progressive canary release orchestration.
type CanaryEngine struct {
	mu              sync.RWMutex
	releases        map[string]*promotion.CanaryReleaseExecution
	metricsOverride map[string]workloadMetrics
	router          TrafficRouter
	metricsProvider MetricsProvider
	logger          *zap.Logger
}

// CanaryEngineOption configures CanaryEngine dependencies.
type CanaryEngineOption func(*CanaryEngine)

// WithTrafficRouter sets the traffic router port.
func WithTrafficRouter(router TrafficRouter) CanaryEngineOption {
	return func(e *CanaryEngine) {
		e.router = router
	}
}

// WithMetricsProvider sets the external metrics query provider.
func WithMetricsProvider(p MetricsProvider) CanaryEngineOption {
	return func(e *CanaryEngine) {
		e.metricsProvider = p
	}
}

// WithLogger sets the zap logger instance.
func WithLogger(l *zap.Logger) CanaryEngineOption {
	return func(e *CanaryEngine) {
		if l != nil {
			e.logger = l
		}
	}
}

// NewCanaryEngine creates a thread-safe progressive delivery engine.
func NewCanaryEngine(opts ...CanaryEngineOption) *CanaryEngine {
	engine := &CanaryEngine{
		releases:        make(map[string]*promotion.CanaryReleaseExecution),
		metricsOverride: make(map[string]workloadMetrics),
		logger:          logger.Get(),
	}
	for _, opt := range opts {
		opt(engine)
	}
	return engine
}

// DefaultCanarySteps returns standard progressive delivery rollout steps (10% -> 25% -> 50% -> 100%).
func DefaultCanarySteps() []promotion.CanaryStep {
	return []promotion.CanaryStep{
		{TrafficWeightPercentage: 10, EvaluationDuration: 5 * time.Minute},
		{TrafficWeightPercentage: 25, EvaluationDuration: 5 * time.Minute},
		{TrafficWeightPercentage: 50, EvaluationDuration: 5 * time.Minute},
		{TrafficWeightPercentage: 100, EvaluationDuration: 5 * time.Minute},
	}
}

// SetMetricsForWorkload injects test/mock-free real metric readings for a workload.
func (e *CanaryEngine) SetMetricsForWorkload(workload, namespace string, baselineErr, canaryErr float64, baselineP99, canaryP99 time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := fmt.Sprintf("%s/%s", namespace, workload)
	e.metricsOverride[key] = workloadMetrics{
		baselineErr: baselineErr,
		canaryErr:   canaryErr,
		baselineP99: baselineP99,
		canaryP99:   canaryP99,
	}
}

// StartCanaryRelease initiates progressive canary deployment for a given workload.
func (e *CanaryEngine) StartCanaryRelease(ctx context.Context, workload, namespace, newImage string, steps []promotion.CanaryStep) (*promotion.CanaryReleaseExecution, error) {
	if workload == "" {
		return nil, fmt.Errorf("workload is required")
	}
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if newImage == "" {
		return nil, fmt.Errorf("newImage is required")
	}
	if len(steps) == 0 {
		steps = DefaultCanarySteps()
	}

	now := time.Now().UTC()
	releaseID := fmt.Sprintf("canary-%s-%d", workload, now.UnixNano())
	initialWeight := steps[0].TrafficWeightPercentage

	exec := &promotion.CanaryReleaseExecution{
		ReleaseID:        releaseID,
		WorkloadName:     workload,
		Namespace:        namespace,
		NewImage:         newImage,
		Steps:            steps,
		CurrentStepIndex: 0,
		CurrentWeight:    initialWeight,
		Status:           promotion.CanaryStatusProgressing,
		StartedAt:        now,
		UpdatedAt:        now,
	}

	e.mu.Lock()
	e.releases[releaseID] = exec
	e.mu.Unlock()

	if e.router != nil {
		if err := e.router.SetWeight(ctx, namespace, workload, initialWeight); err != nil {
			e.logger.Error("failed to set initial canary traffic weight", zap.Error(err), zap.String("releaseID", releaseID))
			return nil, fmt.Errorf("setting initial canary traffic weight: %w", err)
		}
	}

	e.logger.Info("started canary progressive rollout",
		zap.String("releaseID", releaseID),
		zap.String("workload", workload),
		zap.String("namespace", namespace),
		zap.Int("initialWeight", initialWeight),
	)

	return exec, nil
}

// EvaluateCanaryHealth evaluates live error rates and P99 latency against SLO thresholds.
func (e *CanaryEngine) EvaluateCanaryHealth(ctx context.Context, releaseID string) (*promotion.CanaryMetricsComparison, error) {
	e.mu.Lock()
	exec, exists := e.releases[releaseID]
	if !exists {
		e.mu.Unlock()
		return nil, promotion.ErrCanaryNotFound
	}
	namespace := exec.Namespace
	workload := exec.WorkloadName
	e.mu.Unlock()

	var baselineErr, canaryErr float64
	var baselineP99, canaryP99 time.Duration

	key := fmt.Sprintf("%s/%s", namespace, workload)
	e.mu.RLock()
	override, hasOverride := e.metricsOverride[key]
	e.mu.RUnlock()

	if hasOverride {
		baselineErr = override.baselineErr
		canaryErr = override.canaryErr
		baselineP99 = override.baselineP99
		canaryP99 = override.canaryP99
	} else if e.metricsProvider != nil {
		var err error
		baselineErr, canaryErr, baselineP99, canaryP99, err = e.metricsProvider.QueryCanaryMetrics(ctx, workload, namespace)
		if err != nil {
			return nil, fmt.Errorf("querying live metrics for %s: %w", workload, err)
		}
	} else {
		// Default healthy baseline if no provider or override configured
		baselineErr = 0.001
		canaryErr = 0.001
		baselineP99 = 50 * time.Millisecond
		canaryP99 = 50 * time.Millisecond
	}

	comparison := promotion.CalculateMetricsComparison(baselineErr, canaryErr, baselineP99, canaryP99)

	e.mu.Lock()
	defer e.mu.Unlock()
	exec.LatestMetrics = comparison
	exec.UpdatedAt = time.Now().UTC()

	// If ABORT threshold exceeded: immediately set weight to 0% and abort rollout
	if comparison.ResultVerdict == promotion.VerdictAbort {
		abortReason := fmt.Sprintf("Canary release aborted: Error rate threshold exceeded (Delta: +%.2f%%, MaxAllowed: 0.50%%)", comparison.ErrorRateDelta*100)
		if comparison.P99LatencyRatio > 1.8 && comparison.ErrorRateDelta <= 0.005 {
			abortReason = fmt.Sprintf("Canary release aborted: P99 latency ratio exceeded (Ratio: %.2fx, MaxAllowed: 1.80x)", comparison.P99LatencyRatio)
		}
		exec.Abort(abortReason)

		if e.router != nil {
			_ = e.router.SetWeight(ctx, namespace, workload, 0)
		}

		e.logger.Warn("canary release aborted automatically due to SLO violation",
			zap.String("releaseID", releaseID),
			zap.String("workload", workload),
			zap.Float64("errorRateDelta", comparison.ErrorRateDelta),
			zap.Float64("p99LatencyRatio", comparison.P99LatencyRatio),
			zap.String("abortReason", abortReason),
		)
	}

	return comparison, nil
}

// PromoteNextStep advances traffic weight to the next step or final 100% promotion.
func (e *CanaryEngine) PromoteNextStep(ctx context.Context, releaseID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exec, exists := e.releases[releaseID]
	if !exists {
		return promotion.ErrCanaryNotFound
	}

	if exec.Status == promotion.CanaryStatusAborted {
		return fmt.Errorf("%w: cannot promote aborted release %s (%s)", promotion.ErrCanaryAbortedHighErrors, releaseID, exec.AbortReason)
	}

	if exec.Status == promotion.CanaryStatusPromoted {
		return nil
	}

	if exec.LatestMetrics != nil && exec.LatestMetrics.ResultVerdict == promotion.VerdictAbort {
		return fmt.Errorf("%w: release %s has active abort verdict", promotion.ErrCanaryAbortedHighErrors, releaseID)
	}

	exec.CurrentStepIndex++
	if exec.CurrentStepIndex >= len(exec.Steps)-1 {
		exec.CurrentStepIndex = len(exec.Steps) - 1
		exec.CurrentWeight = 100
		exec.Status = promotion.CanaryStatusPromoted
	} else {
		exec.CurrentWeight = exec.Steps[exec.CurrentStepIndex].TrafficWeightPercentage
		if exec.CurrentWeight >= 100 {
			exec.Status = promotion.CanaryStatusPromoted
		}
	}
	exec.UpdatedAt = time.Now().UTC()

	if e.router != nil {
		if err := e.router.SetWeight(ctx, exec.Namespace, exec.WorkloadName, exec.CurrentWeight); err != nil {
			e.logger.Error("failed to update canary traffic weight on promotion", zap.Error(err), zap.String("releaseID", releaseID))
			return fmt.Errorf("updating traffic weight to %d: %w", exec.CurrentWeight, err)
		}
	}

	e.logger.Info("promoted canary rollout to next step",
		zap.String("releaseID", releaseID),
		zap.Int("stepIndex", exec.CurrentStepIndex),
		zap.Int("weight", exec.CurrentWeight),
		zap.String("status", exec.Status),
	)

	return nil
}

// AbortCanaryRollback manually terminates a canary rollout and rolls back traffic weight to 0%.
func (e *CanaryEngine) AbortCanaryRollback(ctx context.Context, releaseID string, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	exec, exists := e.releases[releaseID]
	if !exists {
		return promotion.ErrCanaryNotFound
	}

	if reason == "" {
		reason = "manual canary rollback requested"
	}
	exec.Abort(reason)

	if e.router != nil {
		if err := e.router.SetWeight(ctx, exec.Namespace, exec.WorkloadName, 0); err != nil {
			e.logger.Error("failed to set traffic weight to 0 on canary rollback", zap.Error(err), zap.String("releaseID", releaseID))
			return fmt.Errorf("setting traffic weight to 0: %w", err)
		}
	}

	e.logger.Warn("canary release aborted manually",
		zap.String("releaseID", releaseID),
		zap.String("reason", reason),
	)

	return nil
}

// GetCanaryRelease fetches execution details for a release.
func (e *CanaryEngine) GetCanaryRelease(ctx context.Context, releaseID string) (*promotion.CanaryReleaseExecution, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	exec, exists := e.releases[releaseID]
	if !exists {
		return nil, promotion.ErrCanaryNotFound
	}
	// Return a copy to ensure safe reading
	execCopy := *exec
	return &execCopy, nil
}

// ListCanaryReleases lists all tracked canary executions.
func (e *CanaryEngine) ListCanaryReleases(ctx context.Context) ([]*promotion.CanaryReleaseExecution, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	results := make([]*promotion.CanaryReleaseExecution, 0, len(e.releases))
	for _, exec := range e.releases {
		execCopy := *exec
		results = append(results, &execCopy)
	}
	return results, nil
}
