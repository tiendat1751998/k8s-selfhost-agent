package slo

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
)

// SLOSnapshotUpdater periodically computes rolling SLI compliance and updates SLO snapshots.
type SLOSnapshotUpdater struct {
	repo     observability.Repository
	logger   *zap.Logger
	interval time.Duration
	stopCh   chan struct{}
	mu       sync.Mutex
}

// UpdaterOption configures SLOSnapshotUpdater.
type UpdaterOption func(*SLOSnapshotUpdater)

// WithUpdaterInterval sets the update ticker interval (default: 5m).
func WithUpdaterInterval(d time.Duration) UpdaterOption {
	return func(u *SLOSnapshotUpdater) {
		if d > 0 {
			u.interval = d
		}
	}
}

// NewSLOSnapshotUpdater creates a new SLO compliance snapshot updater.
func NewSLOSnapshotUpdater(repo observability.Repository, logger *zap.Logger, opts ...UpdaterOption) *SLOSnapshotUpdater {
	if logger == nil {
		logger = zap.NewNop()
	}
	u := &SLOSnapshotUpdater{
		repo:     repo,
		logger:   logger,
		interval: 5 * time.Minute,
		stopCh:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// Start begins periodic calculation of SLO compliance snapshots.
func (u *SLOSnapshotUpdater) Start(ctx context.Context) {
	u.logger.Info("Starting SLO compliance snapshot updater", zap.Duration("interval", u.interval))

	// Initial calculation immediately
	u.UpdateOnce(ctx)

	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			u.logger.Info("Stopping SLO compliance snapshot updater (context cancelled)")
			return
		case <-u.stopCh:
			u.logger.Info("Stopping SLO compliance snapshot updater (stop requested)")
			return
		case <-ticker.C:
			u.UpdateOnce(ctx)
		}
	}
}

// Stop terminates the snapshot updater background loop.
func (u *SLOSnapshotUpdater) Stop() {
	u.mu.Lock()
	defer u.mu.Unlock()
	select {
	case <-u.stopCh:
	default:
		close(u.stopCh)
	}
}

// UpdateOnce evaluates all SLO definitions against recorded health samples and upserts current snapshots.
func (u *SLOSnapshotUpdater) UpdateOnce(ctx context.Context) {
	if u.repo == nil {
		return
	}

	evalCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	defs, err := u.repo.ListSLODefinitions(evalCtx)
	if err != nil {
		u.logger.Warn("Failed to list SLO definitions for snapshot update", zap.Error(err))
		return
	}

	for _, d := range defs {
		window := ParseWindow(d.Window)
		actualSLI, computeErr := u.repo.ComputeSLI(evalCtx, d.Service, window)
		if computeErr != nil {
			u.logger.Debug("Failed to compute SLI for service", zap.String("service", d.Service), zap.Error(computeErr))
			actualSLI = 100.0
		}

		target := d.Target
		if target <= 0 {
			target = 99.9
		}
		allowedUnhealthy := 100.0 - target
		if allowedUnhealthy <= 0 {
			allowedUnhealthy = 0.01
		}
		unhealthyActual := 100.0 - actualSLI
		if unhealthyActual < 0 {
			unhealthyActual = 0
		}
		burnRate := unhealthyActual / allowedUnhealthy
		errorBudget := (1.0 - (unhealthyActual / allowedUnhealthy)) * 100.0
		if errorBudget < 0 {
			errorBudget = 0
		}
		if errorBudget > 100 {
			errorBudget = 100
		}

		budgetStatus := "healthy"
		if burnRate > 2.0 {
			budgetStatus = "critical"
		} else if burnRate > 1.0 {
			budgetStatus = "warning"
		}

		existing, getErr := u.repo.GetSLOSnapshotBySLOID(evalCtx, d.ID)
		if getErr != nil {
			u.logger.Debug("Failed to get existing snapshot by SLO ID", zap.String("slo_id", d.ID), zap.Error(getErr))
		}

		now := time.Now().UTC()
		if existing == nil {
			snap := &observability.SLOSnapshot{
				ID:           uuid.NewString(),
				SLOID:        d.ID,
				Service:      d.Service,
				Target:       d.Target,
				Actual:       math.Round(actualSLI*1000) / 1000,
				BurnRate:     math.Round(burnRate*100) / 100,
				ErrorBudget:  math.Round(errorBudget*10) / 10,
				BudgetStatus: budgetStatus,
				RecordedAt:   now,
			}
			if createErr := u.repo.CreateSLOSnapshot(evalCtx, snap); createErr != nil {
				u.logger.Warn("Failed to create SLO snapshot", zap.String("service", d.Service), zap.Error(createErr))
			}
		} else {
			existing.Service = d.Service
			existing.Target = d.Target
			existing.Actual = math.Round(actualSLI*1000) / 1000
			existing.BurnRate = math.Round(burnRate*100) / 100
			existing.ErrorBudget = math.Round(errorBudget*10) / 10
			existing.BudgetStatus = budgetStatus
			existing.RecordedAt = now
			if updateErr := u.repo.UpdateSLOSnapshot(evalCtx, existing); updateErr != nil {
				u.logger.Warn("Failed to update SLO snapshot", zap.String("service", d.Service), zap.Error(updateErr))
			}
		}
	}
}

// ParseWindow converts window string (e.g. "7d", "14d", "30d", "90d", "1h", "24h") to time.Duration.
func ParseWindow(windowStr string) time.Duration {
	windowStr = strings.TrimSpace(strings.ToLower(windowStr))
	if windowStr == "" {
		return 30 * 24 * time.Hour
	}
	if strings.HasSuffix(windowStr, "d") {
		daysStr := strings.TrimSuffix(windowStr, "d")
		var days int
		if _, err := fmt.Sscanf(daysStr, "%d", &days); err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour
		}
	}
	d, err := time.ParseDuration(windowStr)
	if err == nil && d > 0 {
		return d
	}
	return 30 * 24 * time.Hour
}
