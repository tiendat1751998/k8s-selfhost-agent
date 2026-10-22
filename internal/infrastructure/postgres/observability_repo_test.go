package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockObservabilityDBTX struct {
	queries []string
	args    [][]any
}

func (m *mockObservabilityDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("OK"), nil
}

func (m *mockObservabilityDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.queries = append(m.queries, sql)
	m.args = append(m.args, args)

	if len(m.queries) == 1 {
		// First query fails as if tenant_id or alert_threshold column does not exist
		return nil, errors.New(`ERROR: column "tenant_id" does not exist`)
	}

	// Second query (fallback) should be received without tenant_id clause
	return &emptyRows{}, nil
}

func (m *mockObservabilityDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
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
	mock := &mockObservabilityDBTX{}
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
