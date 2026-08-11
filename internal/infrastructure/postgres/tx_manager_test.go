package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

// mockDBTX implements postgres.DBTX for testing.
type mockDBTX struct {
	execCalled     bool
	queryCalled    bool
	queryRowCalled bool
}

func (m *mockDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	m.execCalled = true
	return pgconn.NewCommandTag("EXEC 1"), nil
}

func (m *mockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.queryCalled = true
	return nil, nil
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.queryRowCalled = true
	return nil
}

func TestDBTX_ContextPropagation(t *testing.T) {
	ctx := context.Background()
	fallback := &mockDBTX{}
	tx := &mockDBTX{}

	if postgres.HasTx(ctx) {
		t.Errorf("expected HasTx(ctx) to be false for empty context")
	}

	got := postgres.ExtractTx(ctx, fallback)
	if got != fallback {
		t.Errorf("expected fallback DBTX when context has no transaction")
	}

	txCtx := postgres.InjectTx(ctx, tx)

	if !postgres.HasTx(txCtx) {
		t.Errorf("expected HasTx(txCtx) to be true")
	}

	gotTx := postgres.ExtractTx(txCtx, fallback)
	if gotTx != tx {
		t.Errorf("expected injected tx DBTX, got different instance")
	}
}

func TestTxManager_NestedRunInTx(t *testing.T) {
	tx := &mockDBTX{}
	ctx := postgres.InjectTx(context.Background(), tx)

	txMgr := postgres.NewTxManager(nil)

	executed := false
	err := txMgr.RunInTx(ctx, func(txCtx context.Context) error {
		executed = true
		if postgres.ExtractTx(txCtx, nil) != tx {
			t.Errorf("expected active transaction to be preserved in nested call")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Errorf("expected nested fn to execute")
	}

	// Nested error propagation
	expectedErr := errors.New("nested failure")
	err = txMgr.RunInTx(ctx, func(txCtx context.Context) error {
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}
