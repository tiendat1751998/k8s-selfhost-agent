package promotion_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/datdt/k8sselfhost/internal/domain/promotion"
)

func TestCanaryStep_Validation(t *testing.T) {
	step := promotion.CanaryStep{
		TrafficWeightPercentage: 10,
		EvaluationDuration:      5 * time.Minute,
	}
	assert.Equal(t, 10, step.TrafficWeightPercentage)
	assert.Equal(t, 5*time.Minute, step.EvaluationDuration)
}

func TestCanaryMetricsComparison_VerdictEvaluation(t *testing.T) {
	tests := []struct {
		name            string
		baselineErr     float64
		canaryErr       float64
		baselineP99     time.Duration
		canaryP99       time.Duration
		expectedVerdict string
	}{
		{
			name:            "Healthy metrics",
			baselineErr:     0.001,
			canaryErr:       0.002,
			baselineP99:     100 * time.Millisecond,
			canaryP99:       110 * time.Millisecond,
			expectedVerdict: promotion.VerdictProceed,
		},
		{
			name:            "High error rate triggers abort (delta > 0.005)",
			baselineErr:     0.001,
			canaryErr:       0.013, // delta 0.012 > 0.005
			baselineP99:     100 * time.Millisecond,
			canaryP99:       120 * time.Millisecond,
			expectedVerdict: promotion.VerdictAbort,
		},
		{
			name:            "High latency triggers abort (ratio > 1.8)",
			baselineErr:     0.001,
			canaryErr:       0.002,
			baselineP99:     100 * time.Millisecond,
			canaryP99:       200 * time.Millisecond, // ratio 2.0 > 1.8
			expectedVerdict: promotion.VerdictAbort,
		},
		{
			name:            "Baseline P99 is zero defaults ratio to 1.0",
			baselineErr:     0.001,
			canaryErr:       0.002,
			baselineP99:     0,
			canaryP99:       100 * time.Millisecond,
			expectedVerdict: promotion.VerdictProceed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comparison := promotion.CalculateMetricsComparison(tt.baselineErr, tt.canaryErr, tt.baselineP99, tt.canaryP99)
			assert.Equal(t, tt.expectedVerdict, comparison.ResultVerdict)
		})
	}
}

func TestCanaryReleaseExecution_StateTransitions(t *testing.T) {
	exec := &promotion.CanaryReleaseExecution{
		ReleaseID:        "canary-123",
		WorkloadName:     "payment-service",
		Namespace:        "production",
		CurrentStepIndex: 0,
		CurrentWeight:    10,
		Status:           promotion.CanaryStatusProgressing,
		StartedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	assert.True(t, exec.IsActive())
	assert.False(t, exec.IsTerminal())

	exec.Abort("high error rate")
	assert.Equal(t, promotion.CanaryStatusAborted, exec.Status)
	assert.Equal(t, 0, exec.CurrentWeight)
	assert.Equal(t, "high error rate", exec.AbortReason)
	assert.True(t, exec.IsTerminal())
	assert.False(t, exec.IsActive())
}

func TestProgressiveDeliveryEngine_InterfaceContract(t *testing.T) {
	// Assert interface compilation
	var _ promotion.ProgressiveDeliveryEngine = (*dummyEngine)(nil)
}

type dummyEngine struct{}

func (d *dummyEngine) StartCanaryRelease(ctx context.Context, workload, namespace, newImage string, steps []promotion.CanaryStep) (*promotion.CanaryReleaseExecution, error) {
	return nil, nil
}
func (d *dummyEngine) EvaluateCanaryHealth(ctx context.Context, releaseID string) (*promotion.CanaryMetricsComparison, error) {
	return nil, nil
}
func (d *dummyEngine) PromoteNextStep(ctx context.Context, releaseID string) error {
	return nil
}
func (d *dummyEngine) AbortCanaryRollback(ctx context.Context, releaseID string, reason string) error {
	return nil
}
func (d *dummyEngine) GetCanaryRelease(ctx context.Context, releaseID string) (*promotion.CanaryReleaseExecution, error) {
	return nil, nil
}
func (d *dummyEngine) ListCanaryReleases(ctx context.Context) ([]*promotion.CanaryReleaseExecution, error) {
	return nil, nil
}
