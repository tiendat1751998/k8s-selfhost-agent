package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
