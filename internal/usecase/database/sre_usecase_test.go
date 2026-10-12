package database_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/database"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/database"
)

type fakeDatabaseSREManager struct {
	mu sync.RWMutex

	idleTransactions []domain.IdleTransactionRecord
	tableReports     []domain.TableBloatReport
	redisStatus      *domain.RedisFragmentationStatus

	terminateFn func(pid int) error
	toggleFn    func(enable bool) error

	terminatedPIDs []int
	defragToggled  []bool
}

func (m *fakeDatabaseSREManager) InspectIdleTransactions(ctx context.Context, clusterID, dbName string) ([]domain.IdleTransactionRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.idleTransactions, nil
}

func (m *fakeDatabaseSREManager) TerminateIdleTransaction(ctx context.Context, clusterID, dbName string, pid int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.terminateFn != nil {
		if err := m.terminateFn(pid); err != nil {
			return err
		}
	}
	m.terminatedPIDs = append(m.terminatedPIDs, pid)
	return nil
}

func (m *fakeDatabaseSREManager) AnalyzeTableBloat(ctx context.Context, clusterID, dbName string) ([]domain.TableBloatReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tableReports, nil
}

func (m *fakeDatabaseSREManager) GetRedisFragmentation(ctx context.Context, clusterID string) (*domain.RedisFragmentationStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.redisStatus == nil {
		return nil, errors.New("no redis status configured")
	}
	copyStatus := *m.redisStatus
	return &copyStatus, nil
}

func (m *fakeDatabaseSREManager) ToggleRedisActiveDefrag(ctx context.Context, clusterID string, enable bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.toggleFn != nil {
		if err := m.toggleFn(enable); err != nil {
			return err
		}
	}
	m.defragToggled = append(m.defragToggled, enable)
	if m.redisStatus != nil {
		m.redisStatus.ActiveDefragEnabled = enable
	}
	return nil
}

type fakeAuditRecorder struct {
	mu      sync.Mutex
	actions []map[string]interface{}
}

func (r *fakeAuditRecorder) RecordAction(
	ctx context.Context,
	actor string,
	action string,
	targetType string,
	targetID string,
	targetName string,
	result string,
	details map[string]interface{},
	ipAddress string,
	userAgent string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actions = append(r.actions, map[string]interface{}{
		"actor":      actor,
		"action":     action,
		"targetType": targetType,
		"targetID":   targetID,
		"targetName": targetName,
		"result":     result,
		"details":    details,
	})
	return nil
}

func TestDatabaseSREUsecase_New_Validation(t *testing.T) {
	_, err := usecase.NewDatabaseSREUsecase(nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestDatabaseSREUsecase_TerminateIdleTransaction(t *testing.T) {
	ctx := context.Background()
	mgr := &fakeDatabaseSREManager{}
	audit := &fakeAuditRecorder{}
	uc, err := usecase.NewDatabaseSREUsecase(mgr, audit, zap.NewNop())
	require.NoError(t, err)

	t.Run("invalid pid", func(t *testing.T) {
		err := uc.TerminateIdleTransaction(ctx, "cluster-1", "app_db", -1, "test")
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrDatabaseSREInvalidInput))
	})

	t.Run("active transaction protected", func(t *testing.T) {
		mgr.terminateFn = func(pid int) error {
			if pid == 999 {
				return domain.ErrActiveTransactionProtected
			}
			return nil
		}

		err := uc.TerminateIdleTransaction(ctx, "cluster-1", "app_db", 999, "test active")
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrActiveTransactionProtected))

		audit.mu.Lock()
		defer audit.mu.Unlock()
		require.NotEmpty(t, audit.actions)
		last := audit.actions[len(audit.actions)-1]
		assert.Equal(t, "failed", last["result"])
	})

	t.Run("success terminates and records audit", func(t *testing.T) {
		mgr.terminateFn = nil
		err := uc.TerminateIdleTransaction(ctx, "cluster-1", "app_db", 1234, "idle > 10s")
		require.NoError(t, err)

		mgr.mu.RLock()
		assert.Contains(t, mgr.terminatedPIDs, 1234)
		mgr.mu.RUnlock()

		audit.mu.Lock()
		defer audit.mu.Unlock()
		last := audit.actions[len(audit.actions)-1]
		assert.Equal(t, "success", last["result"])
		assert.Equal(t, "1234", last["targetID"])
	})
}

func TestDatabaseSREUsecase_EvaluateAndTerminateIdleTransactions(t *testing.T) {
	ctx := context.Background()
	tx1, err := domain.NewIdleTransactionRecord(101, "db1", "u1", "127.0.0.1", "web", 15*time.Second, 15*time.Second, "SELECT 1")
	require.NoError(t, err)
	tx2, err := domain.NewIdleTransactionRecord(102, "db1", "u1", "127.0.0.1", "web", 5*time.Second, 5*time.Second, "SELECT 2")
	require.NoError(t, err)

	mgr := &fakeDatabaseSREManager{
		idleTransactions: []domain.IdleTransactionRecord{*tx1, *tx2},
	}
	audit := &fakeAuditRecorder{}
	uc, err := usecase.NewDatabaseSREUsecase(mgr, audit, zap.NewNop())
	require.NoError(t, err)

	terminated, err := uc.EvaluateAndTerminateIdleTransactions(ctx, "cluster-1", "db1", 10*time.Second)
	require.NoError(t, err)
	require.Len(t, terminated, 1)
	assert.Equal(t, 101, terminated[0].PID)

	mgr.mu.RLock()
	assert.Equal(t, []int{101}, mgr.terminatedPIDs)
	mgr.mu.RUnlock()
}

func TestDatabaseSREUsecase_AnalyzeTableBloat(t *testing.T) {
	ctx := context.Background()
	// Table 1: critical bloat (>30% and >1GB)
	rep1, err := domain.NewTableBloatReport("orders", 2*1024*1024*1024, 400000, 600000) // 40% bloat
	require.NoError(t, err)
	// Table 2: minor bloat (<20%)
	rep2, err := domain.NewTableBloatReport("users", 500*1024*1024, 10000, 90000) // 10% bloat
	require.NoError(t, err)

	mgr := &fakeDatabaseSREManager{
		tableReports: []domain.TableBloatReport{*rep1, *rep2},
	}
	audit := &fakeAuditRecorder{}
	uc, err := usecase.NewDatabaseSREUsecase(mgr, audit, zap.NewNop())
	require.NoError(t, err)

	all, critical, err := uc.AnalyzeTableBloat(ctx, "cluster-1", "db1", 20.0)
	require.NoError(t, err)
	assert.Len(t, all, 1) // only rep1 exceeds 20%
	assert.Len(t, critical, 1)
	assert.Equal(t, "orders", critical[0].TableName)
	assert.Contains(t, critical[0].RecommendedFix, "pg_repack")

	audit.mu.Lock()
	defer audit.mu.Unlock()
	require.NotEmpty(t, audit.actions)
	last := audit.actions[len(audit.actions)-1]
	assert.Equal(t, "table_bloat_critical", last["action"])
	assert.Equal(t, "warning", last["result"])
}

func TestDatabaseSREUsecase_EvaluateRedisDefrag(t *testing.T) {
	ctx := context.Background()

	t.Run("auto-enables defrag when ratio > 1.4", func(t *testing.T) {
		status, err := domain.NewRedisFragmentationStatus(1000, 1600, false, false) // 1.6 ratio
		require.NoError(t, err)

		mgr := &fakeDatabaseSREManager{redisStatus: status}
		audit := &fakeAuditRecorder{}
		uc, err := usecase.NewDatabaseSREUsecase(mgr, audit, zap.NewNop())
		require.NoError(t, err)

		res, toggled, err := uc.EvaluateRedisDefrag(ctx, "redis-prod")
		require.NoError(t, err)
		assert.True(t, toggled)
		assert.True(t, res.ActiveDefragEnabled)

		mgr.mu.RLock()
		assert.Equal(t, []bool{true}, mgr.defragToggled)
		mgr.mu.RUnlock()
	})

	t.Run("auto-disables defrag when ratio < 1.15", func(t *testing.T) {
		status, err := domain.NewRedisFragmentationStatus(1000, 1100, true, false) // 1.1 ratio
		require.NoError(t, err)

		mgr := &fakeDatabaseSREManager{redisStatus: status}
		audit := &fakeAuditRecorder{}
		uc, err := usecase.NewDatabaseSREUsecase(mgr, audit, zap.NewNop())
		require.NoError(t, err)

		res, toggled, err := uc.EvaluateRedisDefrag(ctx, "redis-prod")
		require.NoError(t, err)
		assert.True(t, toggled)
		assert.False(t, res.ActiveDefragEnabled)

		mgr.mu.RLock()
		assert.Equal(t, []bool{false}, mgr.defragToggled)
		mgr.mu.RUnlock()
	})

	t.Run("no toggle when ratio is normal (1.25)", func(t *testing.T) {
		status, err := domain.NewRedisFragmentationStatus(1000, 1250, false, false) // 1.25 ratio
		require.NoError(t, err)

		mgr := &fakeDatabaseSREManager{redisStatus: status}
		audit := &fakeAuditRecorder{}
		uc, err := usecase.NewDatabaseSREUsecase(mgr, audit, zap.NewNop())
		require.NoError(t, err)

		res, toggled, err := uc.EvaluateRedisDefrag(ctx, "redis-prod")
		require.NoError(t, err)
		assert.False(t, toggled)
		assert.False(t, res.ActiveDefragEnabled)
		assert.Empty(t, mgr.defragToggled)
	})
}

func TestDatabaseSREUsecase_AutoRemediateDatabase(t *testing.T) {
	ctx := context.Background()
	tx1, err := domain.NewIdleTransactionRecord(555, "db1", "u1", "127.0.0.1", "web", 20*time.Second, 20*time.Second, "SELECT 1")
	require.NoError(t, err)
	rep1, err := domain.NewTableBloatReport("audit_logs", 3*1024*1024*1024, 600000, 400000)
	require.NoError(t, err)
	redisStat, err := domain.NewRedisFragmentationStatus(1000, 1500, false, false)
	require.NoError(t, err)

	mgr := &fakeDatabaseSREManager{
		idleTransactions: []domain.IdleTransactionRecord{*tx1},
		tableReports:     []domain.TableBloatReport{*rep1},
		redisStatus:      redisStat,
	}
	audit := &fakeAuditRecorder{}
	uc, err := usecase.NewDatabaseSREUsecase(mgr, audit, zap.NewNop())
	require.NoError(t, err)

	summary, err := uc.AutoRemediateDatabase(ctx, "cluster-prod", "db1")
	require.NoError(t, err)
	assert.Equal(t, "cluster-prod", summary.ClusterID)
	assert.Equal(t, "db1", summary.DatabaseName)
	assert.Len(t, summary.TerminatedTransactions, 1)
	assert.Len(t, summary.CriticalBloatTables, 1)
	assert.True(t, summary.RedisDefragToggled)
	assert.True(t, summary.DurationMs >= 0)
}

func TestDatabaseSREUsecase_ConcurrencySafe(t *testing.T) {
	ctx := context.Background()
	redisStat, err := domain.NewRedisFragmentationStatus(1000, 1500, false, false)
	require.NoError(t, err)

	mgr := &fakeDatabaseSREManager{redisStatus: redisStat}
	audit := &fakeAuditRecorder{}
	uc, err := usecase.NewDatabaseSREUsecase(mgr, audit, zap.NewNop())
	require.NoError(t, err)

	var wg sync.WaitGroup
	workers := 10
	iterations := 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _, _ = uc.EvaluateRedisDefrag(ctx, "redis-prod")
				_ = uc.TerminateIdleTransaction(ctx, "cluster-1", "db1", 1000+workerID*100+j, "concurrency test")
			}
		}(i)
	}

	wg.Wait()
	mgr.mu.RLock()
	assert.Equal(t, workers*iterations, len(mgr.terminatedPIDs))
	mgr.mu.RUnlock()
}
