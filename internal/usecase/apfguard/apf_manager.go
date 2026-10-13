// Package apfguard implements application usecase orchestration for Kubernetes API Priority
// and Fairness (APF) queue optimization, watch cache 410 Gone thundering herd mitigation,
// and HPA metrics-server fallback scaling.
package apfguard

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/apfguard"
)

// PriorityLevelInfo represents an APF priority level configuration.
type PriorityLevelInfo struct {
	Name                     string `json:"name"`
	AssuredConcurrencyShares int32  `json:"assured_concurrency_shares"`
	QueueLengthLimit         int32  `json:"queue_length_limit"`
	HandSize                 int32  `json:"hand_size"`
	Queues                   int32  `json:"queues"`
	IsPrivileged             bool   `json:"is_privileged"`
}

// StarvationAlert records a priority level starvation event.
type StarvationAlert struct {
	Timestamp     time.Time `json:"timestamp"`
	PriorityLevel string    `json:"priority_level"`
	Code          string    `json:"code"`
	Severity      string    `json:"severity"`
	Message       string    `json:"message"`
}

// K8sAPFClient abstracts interaction with Kubernetes APF API and priority level statuses.
type K8sAPFClient interface {
	ListPriorityLevels(ctx context.Context) ([]PriorityLevelInfo, error)
	GetPriorityLevelStatuses(ctx context.Context) ([]domain.APFQueueStatus, error)
	PatchPriorityLevelShares(ctx context.Context, name string, shares int32) error
}

// WatchCacheClient abstracts chunked paginated listing for cluster workloads.
type WatchCacheClient interface {
	ListChunked(ctx context.Context, namespace string, limit int, continueToken string) (itemsCount int, nextContinueToken string, err error)
	GetActiveNamespaces(ctx context.Context) ([]string, error)
}

// HPAClient abstracts metrics server probing and deployment replica management.
type HPAClient interface {
	ProbeMetricsAPI(ctx context.Context) (available bool, statusCode int, err error)
	GetDeploymentScale(ctx context.Context, namespace, deployment string) (currentReplicas int32, minReplicas int32, maxReplicas int32, err error)
	UpdateDeploymentScale(ctx context.Context, namespace, deployment string, targetReplicas int32) error
}

// ClusterAPFClient composes all client dependencies for APFManager.
type ClusterAPFClient interface {
	K8sAPFClient
	WatchCacheClient
	HPAClient
}

// APFManager orchestrates control plane APF optimization and resilience mitigations.
type APFManager struct {
	client ClusterAPFClient
	logger *zap.Logger

	// Cache-line padded high-frequency telemetry counters.
	apfMetrics        domain.PaddedAPFMetrics
	compactionMetrics domain.PaddedCompactionMetrics

	// Concurrency-safe state guards.
	mu                             sync.RWMutex
	starvationAlerts               []StarvationAlert
	metricsUnavailableSince        time.Time
	totalServerCapacityShares      int32
	unprivilegedShareLimitPercent  int32
	jitterGenerator                func(min, max time.Duration) time.Duration
	sleepFn                        func(ctx context.Context, d time.Duration) error
}

// NewAPFManager constructs a new APFManager with production defaults.
func NewAPFManager(client ClusterAPFClient, logger *zap.Logger) *APFManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	mgr := &APFManager{
		client:                        client,
		logger:                        logger,
		starvationAlerts:              make([]StarvationAlert, 0),
		totalServerCapacityShares:     100,
		unprivilegedShareLimitPercent: 80,
	}

	// Default production jitter generator using crypto/rand
	mgr.jitterGenerator = func(min, max time.Duration) time.Duration {
		if max <= min {
			return min
		}
		span := max - min
		n, err := rand.Int(rand.Reader, big.NewInt(int64(span)))
		if err != nil {
			return min
		}
		return min + time.Duration(n.Int64())
	}

	// Default production sleep function
	mgr.sleepFn = func(ctx context.Context, d time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
			return nil
		}
	}

	return mgr
}

// SetJitterGenerator overrides the jitter generator (used for deterministic testing).
func (m *APFManager) SetJitterGenerator(fn func(min, max time.Duration) time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jitterGenerator = fn
}

// SetSleepFn overrides the sleep function (used for deterministic testing).
func (m *APFManager) SetSleepFn(fn func(ctx context.Context, d time.Duration) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sleepFn = fn
}

// SetMetricsUnavailableSince overrides the timestamp when metrics API became unavailable (for testing).
func (m *APFManager) SetMetricsUnavailableSince(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metricsUnavailableSince = t
}

// GetStarvationAlerts retrieves all recorded starvation alerts.
func (m *APFManager) GetStarvationAlerts() []StarvationAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]StarvationAlert, len(m.starvationAlerts))
	copy(res, m.starvationAlerts)
	return res
}

// GetMetrics returns snapshots of cache-line padded metrics.
func (m *APFManager) GetMetrics() (domain.PaddedAPFMetrics, domain.PaddedCompactionMetrics) {
	return domain.PaddedAPFMetrics{
		CurrentInQueue:   atomic.LoadUint64(&m.apfMetrics.CurrentInQueue),
		CurrentExecuting: atomic.LoadUint64(&m.apfMetrics.CurrentExecuting),
		ConcurrencyLimit: atomic.LoadUint64(&m.apfMetrics.ConcurrencyLimit),
		DropCount:        atomic.LoadUint64(&m.apfMetrics.DropCount),
		StarvationAlerts: atomic.LoadUint64(&m.apfMetrics.StarvationAlerts),
	}, domain.PaddedCompactionMetrics{
		Compacted410Errors: atomic.LoadUint64(&m.compactionMetrics.Compacted410Errors),
		PaginatedChunks:    atomic.LoadUint64(&m.compactionMetrics.PaginatedChunks),
		JitterWaitsTotal:   atomic.LoadUint64(&m.compactionMetrics.JitterWaitsTotal),
		LastRelistMicros:   atomic.LoadUint64(&m.compactionMetrics.LastRelistMicros),
	}
}

// AuditAPFQueueStatus inspects APF queue depths, drops, and starvation states.
// When system-leader-election is starved, alerts P0 and patches assuredConcurrencyShares.
func (m *APFManager) AuditAPFQueueStatus(ctx context.Context) ([]domain.APFQueueStatus, error) {
	statuses, err := m.client.GetPriorityLevelStatuses(ctx)
	if err != nil {
		m.logger.Error("Failed to fetch APF priority level statuses", zap.Error(err))
		return nil, fmt.Errorf("fetching APF queue statuses: %w", err)
	}

	var totalQueue, totalExecuting, totalLimit, totalDrops uint64

	for _, st := range statuses {
		totalQueue += uint64(st.CurrentInQueue)
		totalExecuting += uint64(st.CurrentExecuting)
		totalLimit += uint64(st.ConcurrencyLimit)
		totalDrops += st.DropCount

		// Starvation check for system-leader-election
		if strings.EqualFold(st.PriorityLevel, "system-leader-election") && st.IsStarved {
			atomic.AddUint64(&m.apfMetrics.StarvationAlerts, 1)

			m.logger.Warn("CRITICAL: system-leader-election queue starved",
				zap.Int("in_queue", st.CurrentInQueue),
				zap.Uint64("drop_count", st.DropCount),
			)

			m.recordAlert(StarvationAlert{
				Timestamp:     time.Now().UTC(),
				PriorityLevel: st.PriorityLevel,
				Code:          "LEADER_ELECTION_QUEUE_STARVED",
				Severity:      "P0",
				Message:       fmt.Sprintf("system-leader-election starved: in_queue=%d drops=%d", st.CurrentInQueue, st.DropCount),
			})

			// Auto-scale assuredConcurrencyShares for system-leader-election
			if scaleErr := m.autoScaleLeaderElectionShares(ctx); scaleErr != nil {
				m.logger.Error("Failed to auto-scale leader election shares", zap.Error(scaleErr))
			}
		}
	}

	atomic.StoreUint64(&m.apfMetrics.CurrentInQueue, totalQueue)
	atomic.StoreUint64(&m.apfMetrics.CurrentExecuting, totalExecuting)
	atomic.StoreUint64(&m.apfMetrics.ConcurrencyLimit, totalLimit)
	atomic.StoreUint64(&m.apfMetrics.DropCount, totalDrops)

	return statuses, nil
}

// autoScaleLeaderElectionShares increases concurrency shares for system-leader-election.
func (m *APFManager) autoScaleLeaderElectionShares(ctx context.Context) error {
	levels, err := m.client.ListPriorityLevels(ctx)
	if err != nil {
		return fmt.Errorf("listing priority levels for auto-scale: %w", err)
	}

	var currentShares int32 = 10
	for _, lvl := range levels {
		if strings.EqualFold(lvl.Name, "system-leader-election") {
			currentShares = lvl.AssuredConcurrencyShares
			break
		}
	}

	// Bump shares by +5 or to minimum 20 to absorb burst
	targetShares := currentShares + 10
	if targetShares < 20 {
		targetShares = 20
	}

	if err := m.client.PatchPriorityLevelShares(ctx, "system-leader-election", targetShares); err != nil {
		return fmt.Errorf("patching leader election shares to %d: %w", targetShares, err)
	}

	m.logger.Info("Successfully scaled system-leader-election concurrency shares",
		zap.Int32("previous_shares", currentShares),
		zap.Int32("target_shares", targetShares),
	)
	return nil
}

// AdjustPriorityLevelShares safely updates shares while enforcing the 80% rollback guardrail.
func (m *APFManager) AdjustPriorityLevelShares(ctx context.Context, name string, requestedShares int32) error {
	if requestedShares <= 0 {
		return errors.New("requested shares must be greater than zero")
	}

	levels, err := m.client.ListPriorityLevels(ctx)
	if err != nil {
		return fmt.Errorf("listing priority levels: %w", err)
	}

	var targetLevel *PriorityLevelInfo
	for i := range levels {
		if strings.EqualFold(levels[i].Name, name) {
			targetLevel = &levels[i]
			break
		}
	}

	if targetLevel == nil {
		return fmt.Errorf("priority level '%s' not found", name)
	}

	// Rollback Guardrail: Unprivileged flow concurrency shares must never exceed 80% of total API server capacity.
	if !targetLevel.IsPrivileged {
		var unprivilegedSum int32
		for _, lvl := range levels {
			if strings.EqualFold(lvl.Name, name) {
				unprivilegedSum += requestedShares
			} else if !lvl.IsPrivileged {
				unprivilegedSum += lvl.AssuredConcurrencyShares
			}
		}

		maxUnprivilegedShares := int32(float64(m.totalServerCapacityShares) * (float64(m.unprivilegedShareLimitPercent) / 100.0))
		if unprivilegedSum > maxUnprivilegedShares {
			m.logger.Warn("Guardrail violation: unprivileged shares exceed threshold",
				zap.Int32("calculated_unprivileged_sum", unprivilegedSum),
				zap.Int32("max_allowed", maxUnprivilegedShares),
			)
			return domain.ErrUnprivilegedCapExceeded
		}
	}

	return m.client.PatchPriorityLevelShares(ctx, name, requestedShares)
}

// MitigateWatchCacheCompactionThunderingHerd applies exponential jitter and paginated relisting to mitigate 410 Gone.
func (m *APFManager) MitigateWatchCacheCompactionThunderingHerd(ctx context.Context, policy domain.WatchCacheCompactionPolicy) error {
	if policy.BatchSize <= 0 || policy.MaxJitterWindow <= 0 {
		policy = domain.DefaultWatchCacheCompactionPolicy()
	}
	if err := policy.Validate(); err != nil {
		return err
	}

	atomic.AddUint64(&m.compactionMetrics.Compacted410Errors, 1)

	// Randomized exponential jitter window [5s, policy.MaxJitterWindow]
	minJitter := 5 * time.Second
	maxJitter := policy.MaxJitterWindow
	if maxJitter > 60*time.Second {
		maxJitter = 60 * time.Second
	}
	if maxJitter < minJitter {
		maxJitter = minJitter
	}

	m.mu.RLock()
	jGen := m.jitterGenerator
	sFn := m.sleepFn
	m.mu.RUnlock()

	jitterWait := jGen(minJitter, maxJitter)
	atomic.AddUint64(&m.compactionMetrics.JitterWaitsTotal, 1)

	m.logger.Info("Applying randomized jitter delay before relist to prevent thundering herd",
		zap.Duration("jitter_wait", jitterWait),
		zap.Int("batch_size", policy.BatchSize),
	)

	if err := sFn(ctx, jitterWait); err != nil {
		return fmt.Errorf("jitter delay cancelled: %w", err)
	}

	startRelist := time.Now()

	// Retrieve active namespaces for chunked pagination
	namespaces, err := m.client.GetActiveNamespaces(ctx)
	if err != nil || len(namespaces) == 0 {
		namespaces = []string{"default"}
	}

	// Paginate relist chunk by chunk to prevent 32GB RAM spikes
	for _, ns := range namespaces {
		continueToken := ""
		for {
			_, nextToken, listErr := m.client.ListChunked(ctx, ns, policy.BatchSize, continueToken)
			if listErr != nil {
				return fmt.Errorf("chunked relist failed for namespace %s: %w", ns, listErr)
			}
			atomic.AddUint64(&m.compactionMetrics.PaginatedChunks, 1)

			if nextToken == "" {
				break
			}
			continueToken = nextToken
		}
	}

	relistDuration := time.Since(startRelist)
	atomic.StoreUint64(&m.compactionMetrics.LastRelistMicros, uint64(relistDuration.Microseconds()))

	m.logger.Info("Successfully mitigated watch cache compaction thundering herd with paginated relists",
		zap.Duration("relist_duration", relistDuration),
		zap.Int("namespaces_count", len(namespaces)),
	)
	return nil
}

// CheckHPAHealthAndApplyFallback probes metrics-server and triggers fallback replica scaling if down >2m.
func (m *APFManager) CheckHPAHealthAndApplyFallback(ctx context.Context, namespace, deployment string) (*domain.HPAAutoscaleGuardReport, error) {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(deployment) == "" {
		return nil, errors.New("namespace and deployment must not be empty")
	}

	available, statusCode, probeErr := m.client.ProbeMetricsAPI(ctx)
	isUnavailable := probeErr != nil || !available || statusCode == 503

	m.mu.Lock()
	now := time.Now().UTC()

	if isUnavailable {
		if m.metricsUnavailableSince.IsZero() {
			m.metricsUnavailableSince = now
		}
		downDuration := now.Sub(m.metricsUnavailableSince)
		m.mu.Unlock()

		current, minR, maxR, scaleErr := m.client.GetDeploymentScale(ctx, namespace, deployment)
		if scaleErr != nil {
			return nil, fmt.Errorf("fetching deployment scale: %w", scaleErr)
		}

		// Trigger fallback scaling only if down for > 2m
		if downDuration >= 2*time.Minute {
			// Calculate target: Max(current, ceil(minReplicas * 1.5))
			scaledMin := int32(math.Ceil(float64(minR) * 1.5))
			target := current
			if scaledMin > target {
				target = scaledMin
			}
			if maxR > 0 && target > maxR {
				target = maxR
			}
			if target <= 0 {
				target = 1
			}

			if target > current {
				if updateErr := m.client.UpdateDeploymentScale(ctx, namespace, deployment, target); updateErr != nil {
					return nil, fmt.Errorf("applying fallback scale to %d: %w", target, updateErr)
				}
			}

			return &domain.HPAAutoscaleGuardReport{
				MetricsAPIAvailable: false,
				LastSampleTime:      now,
				FallbackTriggered:   true,
				TargetReplicas:      target,
				CurrentReplicas:     current,
				Message: fmt.Sprintf("Metrics API degraded for %v (>2m); triggered fallback scaling to %d replicas",
					downDuration.Round(time.Second), target),
			}, nil
		}

		// Down < 2m: Do not trigger fallback yet
		return &domain.HPAAutoscaleGuardReport{
			MetricsAPIAvailable: false,
			LastSampleTime:      now,
			FallbackTriggered:   false,
			TargetReplicas:      current,
			CurrentReplicas:     current,
			Message: fmt.Sprintf("Metrics API degraded for %v (<2m); within tolerance window",
				downDuration.Round(time.Second)),
		}, nil
	}

	// Metrics API is healthy
	m.metricsUnavailableSince = time.Time{}
	m.mu.Unlock()

	current, _, _, scaleErr := m.client.GetDeploymentScale(ctx, namespace, deployment)
	if scaleErr != nil {
		return nil, fmt.Errorf("fetching deployment scale: %w", scaleErr)
	}

	return &domain.HPAAutoscaleGuardReport{
		MetricsAPIAvailable: true,
		LastSampleTime:      now,
		FallbackTriggered:   false,
		TargetReplicas:      current,
		CurrentReplicas:     current,
		Message:             "Metrics API is healthy and reporting",
	}, nil
}

func (m *APFManager) recordAlert(alert StarvationAlert) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.starvationAlerts = append(m.starvationAlerts, alert)
	if len(m.starvationAlerts) > 100 {
		m.starvationAlerts = m.starvationAlerts[len(m.starvationAlerts)-100:]
	}
}
