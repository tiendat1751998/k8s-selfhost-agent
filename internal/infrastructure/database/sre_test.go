package database_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/database"
	infra "github.com/datdt/k8sselfhost/internal/infrastructure/database"
)

// fakeDBExecutor implements infra.DBExecutor for PostgreSQL testing.
type fakeDBExecutor struct {
	mu           sync.Mutex
	queries      []string
	queryArgs    [][]any
	queryRows    [][]any
	queryRowFn   func(sql string, args ...any) pgx.Row
	execTag      string
	execErr      error
	queryErr     error
}

func (f *fakeDBExecutor) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, sql)
	f.queryArgs = append(f.queryArgs, arguments)
	if f.execErr != nil {
		return pgconn.CommandTag{}, f.execErr
	}
	return pgconn.NewCommandTag(f.execTag), nil
}

func (f *fakeDBExecutor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, sql)
	f.queryArgs = append(f.queryArgs, args)
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return &testRows{rows: f.queryRows}, nil
}

func (f *fakeDBExecutor) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, sql)
	f.queryArgs = append(f.queryArgs, args)
	if f.queryRowFn != nil {
		return f.queryRowFn(sql, args...)
	}
	return &testRow{err: pgx.ErrNoRows}
}

type testRows struct {
	rows [][]any
	idx  int
}

func (r *testRows) Close()                                       {}
func (r *testRows) Err() error                                   { return nil }
func (r *testRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *testRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *testRows) Next() bool {
	r.idx++
	return r.idx <= len(r.rows)
}
func (r *testRows) Scan(dest ...any) error {
	if r.idx < 1 || r.idx > len(r.rows) {
		return pgx.ErrNoRows
	}
	curr := r.rows[r.idx-1]
	for i, d := range dest {
		if i < len(curr) {
			if err := assignDest(d, curr[i]); err != nil {
				return err
			}
		}
	}
	return nil
}
func (r *testRows) Values() ([]any, error) { return nil, nil }
func (r *testRows) RawValues() [][]byte    { return nil }
func (r *testRows) Conn() *pgx.Conn        { return nil }

type testRow struct {
	values []any
	err    error
}

func (r *testRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		if i < len(r.values) {
			if err := assignDest(d, r.values[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func assignDest(dest any, val any) error {
	switch d := dest.(type) {
	case *int:
		if v, ok := val.(int); ok {
			*d = v
		}
	case *int64:
		if v, ok := val.(int64); ok {
			*d = v
		}
	case *float64:
		if v, ok := val.(float64); ok {
			*d = v
		}
	case *string:
		if v, ok := val.(string); ok {
			*d = v
		}
	case *bool:
		if v, ok := val.(bool); ok {
			*d = v
		}
	default:
		return fmt.Errorf("unsupported dest type %T", dest)
	}
	return nil
}

// fakeRedisClient implements infra.RedisClient for testing.
type fakeRedisClient struct {
	mu           sync.Mutex
	infoOutput   string
	infoErr      error
	configGetVal map[string]string
	configGetErr error
	configSets   map[string]string
	configSetErr error
}

func newFakeRedisClient(info string, activedefrag string) *fakeRedisClient {
	return &fakeRedisClient{
		infoOutput:   info,
		configGetVal: map[string]string{"activedefrag": activedefrag},
		configSets:   make(map[string]string),
	}
}

func (f *fakeRedisClient) Info(ctx context.Context, sections ...string) *redis.StringCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	return redis.NewStringResult(f.infoOutput, f.infoErr)
}

func (f *fakeRedisClient) ConfigGet(ctx context.Context, parameter string) *redis.MapStringStringCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	return redis.NewMapStringStringResult(f.configGetVal, f.configGetErr)
}

func (f *fakeRedisClient) ConfigSet(ctx context.Context, parameter, value string) *redis.StatusCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.configSetErr != nil {
		return redis.NewStatusResult("", f.configSetErr)
	}
	f.configSets[parameter] = value
	return redis.NewStatusResult("OK", nil)
}

func TestPostgres_InspectIdleTransactions(t *testing.T) {
	ctx := context.Background()
	mockDB := &fakeDBExecutor{
		queryRows: [][]any{
			{101, "prod_db", "app_user", "10.0.0.1", "backend-service", 25.5, 12.0, "SELECT * FROM orders"},
			{102, "prod_db", "report_user", "10.0.0.2", "reporter", 30.0, 15.0, "SELECT * FROM users"},
		},
	}

	mgr := infra.NewPostgresSREManager(mockDB, nil, zap.NewNop())
	records, err := mgr.InspectIdleTransactions(ctx, "cluster-1", "prod_db")
	require.NoError(t, err)
	require.Len(t, records, 2)

	assert.Equal(t, 101, records[0].PID)
	assert.Equal(t, "prod_db", records[0].Database)
	assert.Equal(t, "app_user", records[0].Username)
	assert.Equal(t, "10.0.0.1", records[0].ClientIP)
	assert.Equal(t, "backend-service", records[0].ApplicationName)
	assert.Equal(t, 25500*time.Millisecond, records[0].Duration)
	assert.Equal(t, 12*time.Second, records[0].StateChangeAge)
	assert.Equal(t, "SELECT * FROM orders", records[0].CurrentQuery)
}

func TestPostgres_TerminateIdleTransaction(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid pid <= 0", func(t *testing.T) {
		mgr := infra.NewPostgresSREManager(&fakeDBExecutor{}, nil, zap.NewNop())
		err := mgr.TerminateIdleTransaction(ctx, "cluster-1", "db", 0)
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrDatabaseSREInvalidInput))
	})

	t.Run("active transaction protected from termination", func(t *testing.T) {
		mockDB := &fakeDBExecutor{
			queryRowFn: func(sql string, args ...any) pgx.Row {
				return &testRow{values: []any{"active", "SELECT pg_sleep(30)"}}
			},
		}

		mgr := infra.NewPostgresSREManager(mockDB, nil, zap.NewNop())
		err := mgr.TerminateIdleTransaction(ctx, "cluster-1", "prod_db", 202)
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrActiveTransactionProtected))
	})

	t.Run("transaction not found", func(t *testing.T) {
		mockDB := &fakeDBExecutor{
			queryRowFn: func(sql string, args ...any) pgx.Row {
				return &testRow{err: pgx.ErrNoRows}
			},
		}

		mgr := infra.NewPostgresSREManager(mockDB, nil, zap.NewNop())
		err := mgr.TerminateIdleTransaction(ctx, "cluster-1", "prod_db", 9999)
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrTransactionNotFound))
	})

	t.Run("idle transaction terminated successfully", func(t *testing.T) {
		callCount := 0
		mockDB := &fakeDBExecutor{
			queryRowFn: func(sql string, args ...any) pgx.Row {
				callCount++
				if callCount == 1 {
					// State check
					return &testRow{values: []any{"idle in transaction", "COMMIT"}}
				}
				// Terminate backend query
				return &testRow{values: []any{true}}
			},
		}

		mgr := infra.NewPostgresSREManager(mockDB, nil, zap.NewNop())
		err := mgr.TerminateIdleTransaction(ctx, "cluster-1", "prod_db", 303)
		require.NoError(t, err)
		assert.Equal(t, 2, callCount)
	})
}

func TestPostgres_AnalyzeTableBloat_CalculatesCorrectly(t *testing.T) {
	ctx := context.Background()
	// Simulate 100,000 live and 80,000 dead tuples -> 80k / 180k = 44.44% bloat ratio > 40%
	mockDB := &fakeDBExecutor{
		queryRows: [][]any{
			{"large_orders", int64(2 * 1024 * 1024 * 1024), int64(80000), int64(100000)},
			{"small_users", int64(50 * 1024 * 1024), int64(2500), int64(7500)},
			{"clean_logs", int64(10 * 1024 * 1024), int64(100), int64(9900)},
		},
	}

	mgr := infra.NewPostgresSREManager(mockDB, nil, zap.NewNop())
	reports, err := mgr.AnalyzeTableBloat(ctx, "cluster-1", "prod_db")
	require.NoError(t, err)
	require.Len(t, reports, 3)

	// large_orders: 44.44% bloat, >1GB -> pg_repack
	assert.Equal(t, "large_orders", reports[0].TableName)
	assert.InDelta(t, 44.44, reports[0].BloatPercentage, 0.1)
	assert.True(t, reports[0].IsCritical())
	assert.Contains(t, reports[0].RecommendedFix, "pg_repack")

	// small_users: 25.0% bloat, <1GB -> VACUUM FULL
	assert.Equal(t, "small_users", reports[1].TableName)
	assert.InDelta(t, 25.0, reports[1].BloatPercentage, 0.1)
	assert.False(t, reports[1].IsCritical())
	assert.Contains(t, reports[1].RecommendedFix, "VACUUM FULL")

	// clean_logs: 1.0% bloat -> VACUUM ANALYZE
	assert.Equal(t, "clean_logs", reports[2].TableName)
	assert.InDelta(t, 1.0, reports[2].BloatPercentage, 0.1)
	assert.Contains(t, reports[2].RecommendedFix, "VACUUM ANALYZE")
}

func TestRedis_ActiveDefrag_AutoToggle(t *testing.T) {
	ctx := context.Background()
	infoStr := `# Memory
used_memory:1073741824
used_memory_rss:1717986918
mem_fragmentation_ratio:1.60
active_defrag_running:0
`
	fakeClient := newFakeRedisClient(infoStr, "no")
	mgr := infra.NewRedisSREManager(fakeClient, nil, zap.NewNop())

	// 1. Check fragmentation metrics
	status, err := mgr.GetRedisFragmentation(ctx, "cluster-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1073741824), status.UsedMemoryBytes)
	assert.Equal(t, int64(1717986918), status.RSSMemoryBytes)
	assert.InDelta(t, 1.60, status.FragmentationRatio, 0.01)
	assert.False(t, status.ActiveDefragEnabled)
	assert.True(t, status.NeedsDefrag())

	// 2. Toggle active defrag ON
	err = mgr.ToggleRedisActiveDefrag(ctx, "cluster-1", true)
	require.NoError(t, err)

	fakeClient.mu.Lock()
	assert.Equal(t, "yes", fakeClient.configSets["activedefrag"])
	assert.Equal(t, "100mb", fakeClient.configSets["active-defrag-ignore-bytes"])
	assert.Equal(t, "10", fakeClient.configSets["active-defrag-threshold-lower"])
	fakeClient.configGetVal["activedefrag"] = "yes"
	fakeClient.mu.Unlock()

	// 3. Fragmentation drops below 1.15
	droppedInfo := `# Memory
used_memory:1073741824
used_memory_rss:1181116006
mem_fragmentation_ratio:1.10
active_defrag_running:0
`
	fakeClient.mu.Lock()
	fakeClient.infoOutput = droppedInfo
	fakeClient.mu.Unlock()

	status2, err := mgr.GetRedisFragmentation(ctx, "cluster-1")
	require.NoError(t, err)
	assert.InDelta(t, 1.10, status2.FragmentationRatio, 0.01)
	assert.True(t, status2.ActiveDefragEnabled)
	assert.True(t, status2.ShouldDisableDefrag())

	// 4. Toggle active defrag OFF
	err = mgr.ToggleRedisActiveDefrag(ctx, "cluster-1", false)
	require.NoError(t, err)

	fakeClient.mu.Lock()
	assert.Equal(t, "no", fakeClient.configSets["activedefrag"])
	fakeClient.mu.Unlock()
}

func TestCompositeDatabaseSREManager_Delegation(t *testing.T) {
	ctx := context.Background()
	mockDB := &fakeDBExecutor{
		queryRows: [][]any{
			{101, "db", "user", "ip", "app", 10.0, 10.0, "query"},
		},
	}
	infoStr := `used_memory:1000
used_memory_rss:1200
mem_fragmentation_ratio:1.20
`
	fakeClient := newFakeRedisClient(infoStr, "no")

	pgMgr := infra.NewPostgresSREManager(mockDB, nil, zap.NewNop())
	redisMgr := infra.NewRedisSREManager(fakeClient, nil, zap.NewNop())
	composite := infra.NewCompositeDatabaseSREManager(pgMgr, redisMgr)

	// Test delegation of all 5 methods
	txs, err := composite.InspectIdleTransactions(ctx, "c1", "db")
	require.NoError(t, err)
	assert.Len(t, txs, 1)

	reports, err := composite.AnalyzeTableBloat(ctx, "c1", "db")
	require.NoError(t, err)
	assert.Empty(t, reports)

	rStat, err := composite.GetRedisFragmentation(ctx, "c1")
	require.NoError(t, err)
	assert.NotNil(t, rStat)

	err = composite.ToggleRedisActiveDefrag(ctx, "c1", false)
	require.NoError(t, err)

	// Test nil unconfigured manager
	emptyComposite := infra.NewCompositeDatabaseSREManager(nil, nil)
	_, err = emptyComposite.InspectIdleTransactions(ctx, "c1", "db")
	require.Error(t, err)
	err = emptyComposite.TerminateIdleTransaction(ctx, "c1", "db", 1)
	require.Error(t, err)
	_, err = emptyComposite.AnalyzeTableBloat(ctx, "c1", "db")
	require.Error(t, err)
	_, err = emptyComposite.GetRedisFragmentation(ctx, "c1")
	require.Error(t, err)
	err = emptyComposite.ToggleRedisActiveDefrag(ctx, "c1", true)
	require.Error(t, err)
}
