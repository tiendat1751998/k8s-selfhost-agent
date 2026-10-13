package promotion_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/promotion"
	usecasePromotion "github.com/datdt/k8sselfhost/internal/usecase/promotion"
)

type mockTrafficRouter struct {
	mu      sync.Mutex
	weights map[string]int
}

func newMockTrafficRouter() *mockTrafficRouter {
	return &mockTrafficRouter{weights: make(map[string]int)}
}

func (r *mockTrafficRouter) SetWeight(ctx context.Context, namespace, workload string, weight int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s/%s", namespace, workload)
	r.weights[key] = weight
	return nil
}

func (r *mockTrafficRouter) GetWeight(namespace, workload string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.weights[fmt.Sprintf("%s/%s", namespace, workload)]
}

func TestCanary_AbortOnHighErrorRate(t *testing.T) {
	ctx := context.Background()
	router := newMockTrafficRouter()
	engine := usecasePromotion.NewCanaryEngine(usecasePromotion.WithTrafficRouter(router))

	steps := []promotion.CanaryStep{
		{TrafficWeightPercentage: 10, EvaluationDuration: 5 * time.Minute},
		{TrafficWeightPercentage: 25, EvaluationDuration: 5 * time.Minute},
		{TrafficWeightPercentage: 50, EvaluationDuration: 5 * time.Minute},
		{TrafficWeightPercentage: 100, EvaluationDuration: 5 * time.Minute},
	}

	exec, err := engine.StartCanaryRelease(ctx, "payment-api", "production", "payment-api:v2.0.0", steps)
	require.NoError(t, err)
	require.NotNil(t, exec)
	assert.Equal(t, 10, exec.CurrentWeight)
	assert.Equal(t, 10, router.GetWeight("production", "payment-api"))
	assert.Equal(t, promotion.CanaryStatusProgressing, exec.Status)

	// Inject error rate: Primary = 0.1% (0.001), Canary = 1.2% (0.012). Delta = +1.1% (+0.011) > 0.5% (0.005)
	engine.SetMetricsForWorkload("payment-api", "production", 0.001, 0.012, 50*time.Millisecond, 55*time.Millisecond)

	comparison, err := engine.EvaluateCanaryHealth(ctx, exec.ReleaseID)
	require.NoError(t, err)
	require.NotNil(t, comparison)
	assert.Equal(t, promotion.VerdictAbort, comparison.ResultVerdict)
	assert.InDelta(t, 0.011, comparison.ErrorRateDelta, 0.0001)

	// Verify release execution is immediately aborted and weight is set to 0%
	updatedExec, err := engine.GetCanaryRelease(ctx, exec.ReleaseID)
	require.NoError(t, err)
	assert.Equal(t, promotion.CanaryStatusAborted, updatedExec.Status)
	assert.Equal(t, 0, updatedExec.CurrentWeight)
	assert.Equal(t, 0, router.GetWeight("production", "payment-api"))
	assert.NotEmpty(t, updatedExec.AbortReason)

	// Promote on aborted release must fail
	err = engine.PromoteNextStep(ctx, exec.ReleaseID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, promotion.ErrCanaryAbortedHighErrors)
}

func TestCanary_AbortOnHighLatency(t *testing.T) {
	ctx := context.Background()
	router := newMockTrafficRouter()
	engine := usecasePromotion.NewCanaryEngine(usecasePromotion.WithTrafficRouter(router))

	exec, err := engine.StartCanaryRelease(ctx, "order-service", "production", "order-service:v3.1.0", nil)
	require.NoError(t, err)
	assert.Equal(t, 10, exec.CurrentWeight)

	// Inject P99 latency: Primary = 100ms, Canary = 220ms. Ratio = 2.2 > 1.8
	engine.SetMetricsForWorkload("order-service", "production", 0.001, 0.001, 100*time.Millisecond, 220*time.Millisecond)

	comparison, err := engine.EvaluateCanaryHealth(ctx, exec.ReleaseID)
	require.NoError(t, err)
	assert.Equal(t, promotion.VerdictAbort, comparison.ResultVerdict)
	assert.InDelta(t, 2.2, comparison.P99LatencyRatio, 0.01)

	updatedExec, err := engine.GetCanaryRelease(ctx, exec.ReleaseID)
	require.NoError(t, err)
	assert.Equal(t, promotion.CanaryStatusAborted, updatedExec.Status)
	assert.Equal(t, 0, updatedExec.CurrentWeight)
	assert.Equal(t, 0, router.GetWeight("production", "order-service"))
}

func TestCanary_PromoteSuccessfulSteps(t *testing.T) {
	ctx := context.Background()
	router := newMockTrafficRouter()
	engine := usecasePromotion.NewCanaryEngine(usecasePromotion.WithTrafficRouter(router))

	steps := []promotion.CanaryStep{
		{TrafficWeightPercentage: 10, EvaluationDuration: 5 * time.Minute},
		{TrafficWeightPercentage: 25, EvaluationDuration: 5 * time.Minute},
		{TrafficWeightPercentage: 50, EvaluationDuration: 5 * time.Minute},
		{TrafficWeightPercentage: 100, EvaluationDuration: 5 * time.Minute},
	}

	exec, err := engine.StartCanaryRelease(ctx, "catalog-service", "production", "catalog:v1.5.0", steps)
	require.NoError(t, err)
	assert.Equal(t, 0, exec.CurrentStepIndex)
	assert.Equal(t, 10, exec.CurrentWeight)
	assert.Equal(t, 10, router.GetWeight("production", "catalog-service"))

	// Step 1: Healthy metrics
	engine.SetMetricsForWorkload("catalog-service", "production", 0.001, 0.0015, 80*time.Millisecond, 85*time.Millisecond)
	comp, err := engine.EvaluateCanaryHealth(ctx, exec.ReleaseID)
	require.NoError(t, err)
	assert.Equal(t, promotion.VerdictProceed, comp.ResultVerdict)

	// Promote to Step 2 (25%)
	err = engine.PromoteNextStep(ctx, exec.ReleaseID)
	require.NoError(t, err)
	exec, err = engine.GetCanaryRelease(ctx, exec.ReleaseID)
	require.NoError(t, err)
	assert.Equal(t, 1, exec.CurrentStepIndex)
	assert.Equal(t, 25, exec.CurrentWeight)
	assert.Equal(t, 25, router.GetWeight("production", "catalog-service"))
	assert.Equal(t, promotion.CanaryStatusProgressing, exec.Status)

	// Promote to Step 3 (50%)
	err = engine.PromoteNextStep(ctx, exec.ReleaseID)
	require.NoError(t, err)
	exec, err = engine.GetCanaryRelease(ctx, exec.ReleaseID)
	require.NoError(t, err)
	assert.Equal(t, 2, exec.CurrentStepIndex)
	assert.Equal(t, 50, exec.CurrentWeight)
	assert.Equal(t, 50, router.GetWeight("production", "catalog-service"))

	// Promote to Step 4 (100% -> PROMOTED)
	err = engine.PromoteNextStep(ctx, exec.ReleaseID)
	require.NoError(t, err)
	exec, err = engine.GetCanaryRelease(ctx, exec.ReleaseID)
	require.NoError(t, err)
	assert.Equal(t, 3, exec.CurrentStepIndex)
	assert.Equal(t, 100, exec.CurrentWeight)
	assert.Equal(t, 100, router.GetWeight("production", "catalog-service"))
	assert.Equal(t, promotion.CanaryStatusPromoted, exec.Status)
}

func TestCanary_ManualAbortRollback(t *testing.T) {
	ctx := context.Background()
	router := newMockTrafficRouter()
	engine := usecasePromotion.NewCanaryEngine(usecasePromotion.WithTrafficRouter(router))

	exec, err := engine.StartCanaryRelease(ctx, "auth-service", "production", "auth:v2.0", nil)
	require.NoError(t, err)

	err = engine.AbortCanaryRollback(ctx, exec.ReleaseID, "manual emergency abort by sre")
	require.NoError(t, err)

	exec, err = engine.GetCanaryRelease(ctx, exec.ReleaseID)
	require.NoError(t, err)
	assert.Equal(t, promotion.CanaryStatusAborted, exec.Status)
	assert.Equal(t, 0, exec.CurrentWeight)
	assert.Equal(t, "manual emergency abort by sre", exec.AbortReason)
}

func TestCanary_ConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	engine := usecasePromotion.NewCanaryEngine()

	exec, err := engine.StartCanaryRelease(ctx, "concurrent-test", "prod", "v1", nil)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, _ = engine.EvaluateCanaryHealth(ctx, exec.ReleaseID)
			_, _ = engine.GetCanaryRelease(ctx, exec.ReleaseID)
			_ = engine.PromoteNextStep(ctx, exec.ReleaseID)
		}(i)
	}
	wg.Wait()
}

func BenchmarkCanaryEvaluation(b *testing.B) {
	ctx := context.Background()
	engine := usecasePromotion.NewCanaryEngine()
	engine.SetMetricsForWorkload("bench-service", "default", 0.001, 0.002, 50*time.Millisecond, 60*time.Millisecond)

	exec, err := engine.StartCanaryRelease(ctx, "bench-service", "default", "v1", nil)
	if err != nil {
		b.Fatalf("failed to start canary: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.EvaluateCanaryHealth(ctx, exec.ReleaseID)
	}
}
