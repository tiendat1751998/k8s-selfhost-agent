package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type backupMockDBTX struct {
	lastSQL  string
	lastArgs []any
}

func (m *backupMockDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	m.lastSQL = sql
	m.lastArgs = arguments
	return pgconn.NewCommandTag("UPDATE 2"), nil
}

func (m *backupMockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.lastSQL = sql
	m.lastArgs = args
	return nil, nil
}

func (m *backupMockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.lastSQL = sql
	m.lastArgs = args
	return nil
}

func TestBackupRepo_ReconcileStaleJobs_Parameterized(t *testing.T) {
	t.Run("executes parameterized update for platform admin or startup", func(t *testing.T) {
		mock := &backupMockDBTX{}
		repo := NewBackupRepo(mock)

		ctx := tenancy.WithUserRole(context.Background(), "platform_admin")
		err := repo.ReconcileStaleJobs(ctx)
		require.NoError(t, err)

		// SQL must be parameterized
		assert.Contains(t, mock.lastSQL, "UPDATE backup_jobs")
		assert.Contains(t, mock.lastSQL, "SET status = $1, error_message = $2, updated_at = $3")
		assert.Contains(t, mock.lastSQL, "WHERE status = $4")
		assert.NotContains(t, mock.lastSQL, "'interrupted by restart'", "must not contain string literal in raw query")
		assert.NotContains(t, mock.lastSQL, "'running'", "must not contain string literal in raw query")

		// Verify parameters
		require.Len(t, mock.lastArgs, 4)
		assert.Equal(t, "failed", mock.lastArgs[0])
		assert.Equal(t, "interrupted by restart", mock.lastArgs[1])
		assert.IsType(t, time.Time{}, mock.lastArgs[2])
		assert.Equal(t, "running", mock.lastArgs[3])
	})

	t.Run("executes with tenant scoping when tenant present in context", func(t *testing.T) {
		mock := &backupMockDBTX{}
		repo := NewBackupRepo(mock)

		ctx := tenancy.WithTenantID(context.Background(), "tenant-alpha")
		err := repo.ReconcileStaleJobs(ctx)
		require.NoError(t, err)

		assert.Contains(t, strings.ToUpper(mock.lastSQL), "UPDATE BACKUP_JOBS")
		// Verify tenant_id was parameterized
		hasTenantArg := false
		for _, arg := range mock.lastArgs {
			if arg == "tenant-alpha" {
				hasTenantArg = true
				break
			}
		}
		assert.True(t, hasTenantArg, "tenant-alpha must be parameterized in query args")
	})
}
