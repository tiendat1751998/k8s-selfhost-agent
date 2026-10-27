package postgres

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockObservabilityDBTX struct {
	queries        []string
	args           [][]any
	failFirstQuery bool
}

func (m *mockObservabilityDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	m.queries = append(m.queries, sql)
	m.args = append(m.args, arguments)
	return pgconn.NewCommandTag("OK"), nil
}

func (m *mockObservabilityDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.queries = append(m.queries, sql)
	m.args = append(m.args, args)

	if m.failFirstQuery && len(m.queries) == 1 {
		// First query fails as if tenant_id or alert_threshold column does not exist
		return nil, errors.New(`ERROR: column "tenant_id" does not exist`)
	}

	// Fallback or normal query succeeds
	return &emptyRows{}, nil
}

func (m *mockObservabilityDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.queries = append(m.queries, sql)
	m.args = append(m.args, args)
	return &emptyRows{}
}

type emptyRows struct{}

func (e *emptyRows) Close()                                       {}
func (e *emptyRows) Err() error                                   { return nil }
func (e *emptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (e *emptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (e *emptyRows) Next() bool                                   { return false }
func (e *emptyRows) Scan(dest ...any) error                       { return nil }
func (e *emptyRows) Values() ([]any, error)                       { return nil, nil }
func (e *emptyRows) RawValues() [][]byte                          { return nil }
func (e *emptyRows) Conn() *pgx.Conn                              { return nil }

func TestObservabilityRepo_ListSLODefinitions_FallbackDirect(t *testing.T) {
	mock := &mockObservabilityDBTX{failFirstQuery: true}
	repo := NewObservabilityRepo(mock)

	ctx := tenancy.WithTenantID(context.Background(), "test-tenant-123")
	defs, err := repo.ListSLODefinitions(ctx)
	require.NoError(t, err)
	assert.Empty(t, defs)

	require.Len(t, mock.queries, 2, "expected primary query followed by fallback query")
	assert.Contains(t, mock.queries[0], "tenant_id")
	// Verify the fallback query did NOT get wrapped with BuildTenantQuery
	assert.NotContains(t, mock.queries[1], "tenant_id", "fallback query must not contain tenant_id")
	assert.Empty(t, mock.args[1], "fallback query must have no tenant args")
}

func TestObservabilityRepo_BackgroundEvaluator_BypassesTenantScoping(t *testing.T) {
	mock := &mockObservabilityDBTX{}
	repo := NewObservabilityRepo(mock)

	// Background worker context has no user role and no tenant ID
	ctx := context.Background()
	defs, err := repo.ListSLODefinitions(ctx)
	require.NoError(t, err)
	assert.Empty(t, defs)

	require.NotEmpty(t, mock.queries)
	// Query must NOT contain tenant_id = '' or any tenant_id filter
	assert.NotContains(t, mock.queries[0], "tenant_id", "background worker must query across all tenants")
	assert.Empty(t, mock.args[0], "background worker must not pass tenant args")
}

func TestObservabilityRepo_NonAdmin_EmptyTenant_FailsClosedWithNilUUID(t *testing.T) {
	mock := &mockObservabilityDBTX{}
	repo := NewObservabilityRepo(mock)

	// Non-admin user with empty tenant
	ctx := tenancy.WithUserRole(context.Background(), "user")
	_, err := repo.ListSLODefinitions(ctx)
	require.NoError(t, err)

	require.NotEmpty(t, mock.queries)
	assert.Contains(t, mock.queries[0], "tenant_id", "non-admin must have tenant filter")
	require.NotEmpty(t, mock.args[0])
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", mock.args[0][0], "must pass nil UUID, never empty string")
}

func TestObservabilityRepo_InvalidTenant_NormalizedToNilUUID(t *testing.T) {
	mock := &mockObservabilityDBTX{}
	repo := NewObservabilityRepo(mock)

	ctx := tenancy.WithTenantID(context.Background(), "default-tenant")
	_, err := repo.ListSLODefinitions(ctx)
	require.NoError(t, err)

	require.NotEmpty(t, mock.queries)
	assert.Contains(t, mock.queries[0], "tenant_id")
	require.NotEmpty(t, mock.args[0])
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", mock.args[0][0], "default-tenant must be normalized to nil UUID")
}

func TestObservabilityRepo_ValidTenantUUID_Preserved(t *testing.T) {
	mock := &mockObservabilityDBTX{}
	repo := NewObservabilityRepo(mock)

	validUUID := "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d"
	ctx := tenancy.WithTenantID(context.Background(), validUUID)
	_, err := repo.ListSLODefinitions(ctx)
	require.NoError(t, err)

	require.NotEmpty(t, mock.queries)
	assert.Contains(t, mock.queries[0], "tenant_id")
	require.NotEmpty(t, mock.args[0])
	assert.Equal(t, validUUID, mock.args[0][0])
}

func TestObservabilityRepo_PlatformAdmin_BypassesTenantScoping(t *testing.T) {
	mock := &mockObservabilityDBTX{}
	repo := NewObservabilityRepo(mock)

	ctx := tenancy.WithUserRole(context.Background(), "platform_admin")
	_, err := repo.ListSLODefinitions(ctx)
	require.NoError(t, err)

	require.NotEmpty(t, mock.queries)
	assert.NotContains(t, mock.queries[0], "tenant_id", "platform_admin must bypass tenant scoping")
	assert.Empty(t, mock.args[0])
}

func TestObservabilityRepo_CreateSLODefinition_NeverInsertsNonUUID(t *testing.T) {
	mock := &mockObservabilityDBTX{}
	repo := NewObservabilityRepo(mock)

	// Case 1: Empty context
	err := repo.CreateSLODefinition(context.Background(), &observability.SLODefinition{Service: "payment"})
	require.NoError(t, err)
	require.NotEmpty(t, mock.args)
	lastArgs := mock.args[len(mock.args)-1]
	require.Len(t, lastArgs, 10)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", lastArgs[9], "tenant_id must be valid UUID, not 'default-tenant'")

	// Case 2: default-tenant
	ctxDefault := tenancy.WithTenantID(context.Background(), "default-tenant")
	err = repo.CreateSLODefinition(ctxDefault, &observability.SLODefinition{Service: "order"})
	require.NoError(t, err)
	lastArgs = mock.args[len(mock.args)-1]
	require.Len(t, lastArgs, 10)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", lastArgs[9], "default-tenant must normalize to nil UUID")
}

func TestObservabilityRepo_CreateSLOSnapshot_NeverInsertsNonUUID(t *testing.T) {
	mock := &mockObservabilityDBTX{}
	repo := NewObservabilityRepo(mock)

	err := repo.CreateSLOSnapshot(context.Background(), &observability.SLOSnapshot{Service: "payment"})
	require.NoError(t, err)
	require.NotEmpty(t, mock.args)
	lastArgs := mock.args[len(mock.args)-1]
	require.Len(t, lastArgs, 10)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", lastArgs[9], "tenant_id must be valid UUID, not 'default-tenant'")
}

func TestObservabilityRepo_ComputeSLI_ZeroSamplesReturnsSentinelNegativeOne(t *testing.T) {
	mock := &mockObservabilityDBTX{}
	repo := NewObservabilityRepo(mock)

	sli, err := repo.ComputeSLI(context.Background(), "k8s-agent-fleet", 7*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, -1.0, sli, "ComputeSLI must return -1.0 sentinel when totalCount == 0 (zero samples)")
}

func TestObservabilityRepo_ComputeSLI_NormalizesTenantUUID(t *testing.T) {
	mock := &mockObservabilityDBTX{}
	repo := NewObservabilityRepo(mock)

	// default-tenant should normalize to nil UUID
	ctx := tenancy.WithTenantID(context.Background(), "default-tenant")
	ctx = tenancy.WithUserRole(ctx, "operator")
	_, err := repo.ComputeSLI(ctx, "k8s-agent-fleet", 7*24*time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, mock.args)
	lastArgs := mock.args[len(mock.args)-1]
	require.Len(t, lastArgs, 3)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", lastArgs[2], "ComputeSLI must normalize 'default-tenant' to nil UUID")
}

func TestObservabilityRepo_RecordHealthSample_NormalizesDefaultTenant(t *testing.T) {
	mock := &mockObservabilityDBTX{}
	repo := NewObservabilityRepo(mock)

	latency := 12
	err := repo.RecordHealthSample(context.Background(), "default-tenant", "k8s-agent-fleet", 3, 3, true, &latency)
	require.NoError(t, err)
	require.NotEmpty(t, mock.args)
	lastArgs := mock.args[len(mock.args)-1]
	require.Len(t, lastArgs, 7)
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", lastArgs[1], "RecordHealthSample must normalize 'default-tenant' to nil UUID")
}

func TestObservabilityRepo_CreateAndUpdateSLOSnapshot_ClampsBurnRateToNumericMax(t *testing.T) {
	mock := &mockObservabilityDBTX{}
	repo := NewObservabilityRepo(mock)

	// Create with out-of-bounds burn rate (e.g. 10000.0)
	snap := &observability.SLOSnapshot{
		Service:      "test-down-service",
		Actual:       0.0,
		BurnRate:     10000.0,
		ErrorBudget:  -10.0,
		BudgetStatus: "critical",
	}
	err := repo.CreateSLOSnapshot(context.Background(), snap)
	require.NoError(t, err)
	require.NotEmpty(t, mock.args)
	lastArgs := mock.args[len(mock.args)-1]
	require.Len(t, lastArgs, 10)
	// $6 is burn_rate, $7 is error_budget
	assert.Equal(t, 999.99, lastArgs[5], "CreateSLOSnapshot must clamp burn_rate to 999.99")
	assert.Equal(t, 0.0, lastArgs[6], "CreateSLOSnapshot must clamp error_budget >= 0.0")

	// Update with out-of-bounds burn rate
	snap.BurnRate = 2000.0
	err = repo.UpdateSLOSnapshot(context.Background(), snap)
	require.NoError(t, err)
	lastArgs = mock.args[len(mock.args)-1]
	require.Len(t, lastArgs, 6)
	// UPDATE slo_snapshots SET actual = $1, burn_rate = $2, error_budget = $3, ...
	assert.Equal(t, 999.99, lastArgs[1], "UpdateSLOSnapshot must clamp burn_rate to 999.99")
}

func getLiveObservabilityPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// Read .env if direct env vars are empty (never log or print secrets)
	if os.Getenv("POSTGRES_PASSWORD") == "" && os.Getenv("K8S_POSTGRES_PASSWORD") == "" {
		if f, err := os.Open(`d:\project\k8sseflhost\.env`); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					k := strings.TrimSpace(parts[0])
					v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					if os.Getenv(k) == "" {
						os.Setenv(k, v)
					}
				}
			}
			f.Close()
		}
	}

	host := os.Getenv("K8S_POSTGRES_HOST")
	if host == "" {
		host = "10.10.10.133"
	}
	port := os.Getenv("K8S_POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("K8S_POSTGRES_USER")
	if user == "" {
		user = "myuser"
	}
	pass := os.Getenv("POSTGRES_PASSWORD")
	if pass == "" {
		pass = os.Getenv("K8S_POSTGRES_PASSWORD")
	}
	if pass == "" {
		t.Skip("Skipping live database test: POSTGRES_PASSWORD not set")
		return nil
	}
	dbname := os.Getenv("K8S_POSTGRES_DBNAME")
	if dbname == "" {
		dbname = "mydatabase"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbname)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Skipping live database test: unable to connect: %v", err)
		return nil
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("Skipping live database test: unable to ping: %v", err)
		return nil
	}
	return pool
}

func TestObservabilityRepo_LiveDB_SaveDownAndUnmeasuredSnapshot(t *testing.T) {
	pool := getLiveObservabilityPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	repo := NewObservabilityRepo(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testSLOID := uuid.NewString()
	testSnapID1 := uuid.NewString()
	testSnapID2 := uuid.NewString()

	// Clean up any test rows on defer
	defer func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_, _ = pool.Exec(cleanCtx, "DELETE FROM slo_snapshots WHERE id IN ($1, $2) OR slo_id = $3", testSnapID1, testSnapID2, testSLOID)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM slo_definitions WHERE id = $1", testSLOID)
	}()

	// Create test SLO definition
	testDef := &observability.SLODefinition{
		ID:             testSLOID,
		Service:        "test-down-service-" + testSLOID[:8],
		Target:         99.99,
		IndicatorType:  "availability",
		Window:         "30d",
		AlertThreshold: 1.0,
	}
	err := repo.CreateSLODefinition(ctx, testDef)
	require.NoError(t, err, "failed to create test SLO definition")

	// 1. Down Service Snapshot: Actual: 0.0, BurnRate: 10000.0 (clamps to 999.99), BudgetStatus: "critical"
	downSnap := &observability.SLOSnapshot{
		ID:           testSnapID1,
		SLOID:        testSLOID,
		Service:      testDef.Service,
		Target:       testDef.Target,
		Actual:       0.0,
		BurnRate:     10000.0, // Out-of-bounds -> must clamp to 999.99 without SQLSTATE 22003
		ErrorBudget:  0.0,
		BudgetStatus: "critical",
	}
	err = repo.CreateSLOSnapshot(ctx, downSnap)
	require.NoError(t, err, "saving down snapshot with high burn rate must succeed without SQLSTATE 22003")

	// Verify it was persisted and read back
	savedDown, err := repo.GetSLOSnapshotBySLOID(ctx, testSLOID)
	require.NoError(t, err)
	require.NotNil(t, savedDown)
	assert.Equal(t, 0.0, savedDown.Actual)
	assert.Equal(t, 999.99, savedDown.BurnRate, "burn_rate in PostgreSQL must be clamped to 999.99")
	assert.Equal(t, "critical", savedDown.BudgetStatus)

	// 2. Unmeasured Service Snapshot: Actual: -1.0, BurnRate: 0.0, BudgetStatus: "unmeasured"
	unmeasuredSnap := &observability.SLOSnapshot{
		ID:           testSnapID2,
		SLOID:        testSLOID,
		Service:      testDef.Service,
		Target:       testDef.Target,
		Actual:       -1.0, // Sentinel value for 0 samples
		BurnRate:     0.0,
		ErrorBudget:  0.0,
		BudgetStatus: "unmeasured",
	}
	err = repo.CreateSLOSnapshot(ctx, unmeasuredSnap)
	require.NoError(t, err, "saving unmeasured snapshot with -1.0 actual must succeed without SQLSTATE 22003")

	// 3. Update existing snapshot with out-of-bounds burn rate
	savedDown.BurnRate = 2000.0
	err = repo.UpdateSLOSnapshot(ctx, savedDown)
	require.NoError(t, err, "updating snapshot with out-of-bounds burn rate must succeed without SQLSTATE 22003")
}


