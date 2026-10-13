package promotion

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Status constants for Canary release lifecycle.
const (
	CanaryStatusProgressing = "PROGRESSING"
	CanaryStatusPromoted    = "PROMOTED"
	CanaryStatusAborted     = "ABORTED"
)

// Verdict constants for canary metrics analysis.
const (
	VerdictProceed = "PROCEED"
	VerdictWarn    = "WARN"
	VerdictAbort   = "ABORT"
)

// RFC 7807 Error Code Constants.
const (
	ErrCodeCanaryAbortedHighErrors    = "ERR_CANARY_ABORTED_HIGH_ERRORS"
	ErrCodeSystemOverloadLoadShedding = "ERR_SYSTEM_OVERLOAD_LOAD_SHEDDING"
)

// Sentinel domain errors.
var (
	ErrCanaryNotFound               = errors.New("canary release not found")
	ErrCanaryAlreadyTerminal        = errors.New("canary release is already in terminal state")
	ErrCanaryAbortedHighErrors      = errors.New("canary release aborted: error rate threshold exceeded")
	ErrCanaryAbortedHighLatency     = errors.New("canary release aborted: latency threshold exceeded")
	ErrSystemOverloadedLoadShedding = errors.New("system overload detected: request shed")
)

// CanaryStep defines a progressive traffic step.
type CanaryStep struct {
	TrafficWeightPercentage int           `json:"traffic_weight_percentage"` // 10%, 25%, 50%, 100%
	EvaluationDuration      time.Duration `json:"evaluation_duration"`       // e.g. 5m
}

// UnmarshalJSON unmarshals CanaryStep supporting both duration string (e.g. "5m") and int64 nanoseconds.
func (s *CanaryStep) UnmarshalJSON(data []byte) error {
	type Alias CanaryStep
	aux := &struct {
		EvaluationDuration interface{} `json:"evaluation_duration"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch v := aux.EvaluationDuration.(type) {
	case string:
		d, err := time.ParseDuration(v)
		if err != nil {
			return err
		}
		s.EvaluationDuration = d
	case float64:
		s.EvaluationDuration = time.Duration(v)
	}
	return nil
}

// CanaryMetricsComparison compares baseline and canary metrics.
type CanaryMetricsComparison struct {
	BaselineErrorRate float64       `json:"baseline_error_rate"`
	CanaryErrorRate   float64       `json:"canary_error_rate"`
	ErrorRateDelta    float64       `json:"error_rate_delta"` // Canary - Baseline
	BaselineP99       time.Duration `json:"baseline_p99"`
	CanaryP99         time.Duration `json:"canary_p99"`
	P99LatencyRatio   float64       `json:"p99_latency_ratio"`
	ResultVerdict     string        `json:"result_verdict"` // PROCEED, WARN, ABORT
}

// CanaryReleaseExecution captures the live state of a canary rollout.
type CanaryReleaseExecution struct {
	ReleaseID        string                   `json:"release_id"`
	WorkloadName     string                   `json:"workload_name"`
	Namespace        string                   `json:"namespace"`
	NewImage         string                   `json:"new_image,omitempty"`
	Steps            []CanaryStep             `json:"steps,omitempty"`
	CurrentStepIndex int                      `json:"current_step_index"`
	CurrentWeight    int                      `json:"current_weight"`
	Status           string                   `json:"status"` // PROGRESSING, PROMOTED, ABORTED
	AbortReason      string                   `json:"abort_reason,omitempty"`
	LatestMetrics    *CanaryMetricsComparison `json:"latest_metrics,omitempty"`
	StartedAt        time.Time                `json:"started_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
}

// IsActive returns true if rollout is still progressing.
func (c *CanaryReleaseExecution) IsActive() bool {
	return c.Status == CanaryStatusProgressing
}

// IsTerminal returns true if rollout completed or was aborted.
func (c *CanaryReleaseExecution) IsTerminal() bool {
	return c.Status == CanaryStatusPromoted || c.Status == CanaryStatusAborted
}

// Abort transitions rollout to aborted state with 0% weight.
func (c *CanaryReleaseExecution) Abort(reason string) {
	c.Status = CanaryStatusAborted
	c.CurrentWeight = 0
	c.AbortReason = reason
	c.UpdatedAt = time.Now().UTC()
}

// CalculateMetricsComparison computes metrics delta and auto-verdict based on SLO thresholds.
func CalculateMetricsComparison(baselineErr, canaryErr float64, baselineP99, canaryP99 time.Duration) *CanaryMetricsComparison {
	delta := canaryErr - baselineErr
	var ratio float64
	if baselineP99 <= 0 {
		ratio = 1.0
	} else {
		ratio = float64(canaryP99) / float64(baselineP99)
	}

	verdict := VerdictProceed
	if delta > 0.005 || ratio > 1.8 {
		verdict = VerdictAbort
	} else if delta > 0.002 || ratio > 1.3 {
		verdict = VerdictWarn
	}

	return &CanaryMetricsComparison{
		BaselineErrorRate: baselineErr,
		CanaryErrorRate:   canaryErr,
		ErrorRateDelta:    delta,
		BaselineP99:       baselineP99,
		CanaryP99:         canaryP99,
		P99LatencyRatio:   ratio,
		ResultVerdict:     verdict,
	}
}

// ProgressiveDeliveryEngine defines the domain interface for progressive canary rollouts.
type ProgressiveDeliveryEngine interface {
	StartCanaryRelease(ctx context.Context, workload, namespace, newImage string, steps []CanaryStep) (*CanaryReleaseExecution, error)
	EvaluateCanaryHealth(ctx context.Context, releaseID string) (*CanaryMetricsComparison, error)
	PromoteNextStep(ctx context.Context, releaseID string) error
	AbortCanaryRollback(ctx context.Context, releaseID string, reason string) error
	GetCanaryRelease(ctx context.Context, releaseID string) (*CanaryReleaseExecution, error)
	ListCanaryReleases(ctx context.Context) ([]*CanaryReleaseExecution, error)
}
