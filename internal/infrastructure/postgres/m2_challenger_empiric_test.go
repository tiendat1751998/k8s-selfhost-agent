package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/datdt/k8sselfhost/internal/domain/agent"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

// recordingDBTX implements postgres.DBTX for testing query routing.
type recordingDBTX struct {
	name          string
	execCount     int
	queryCount    int
	queryRowCount int
	lastSQL       string
}

func (r *recordingDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	r.execCount++
	r.lastSQL = sql
	return pgconn.NewCommandTag("EXEC 1"), nil
}

func (r *recordingDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	r.queryCount++
	r.lastSQL = sql
	return nil, nil
}

func (r *recordingDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	r.queryRowCount++
	r.lastSQL = sql
	return &dummyRow{}
}

type dummyRow struct{}

func (d *dummyRow) Scan(dest ...any) error {
	return pgx.ErrNoRows
}

// TestEmpirical_DBTX_ContextExtraction verifies InjectTx, ExtractTx, and HasTx
func TestEmpirical_DBTX_ContextExtraction(t *testing.T) {
	poolDB := &recordingDBTX{name: "pool"}
	txDB := &recordingDBTX{name: "tx"}
	ctx := context.Background()

	// 1. Un-injected context
	if postgres.HasTx(ctx) {
		t.Fatalf("expected HasTx(ctx) == false for plain context")
	}
	extracted := postgres.ExtractTx(ctx, poolDB)
	if extracted != poolDB {
		t.Fatalf("expected fallback poolDB when context has no transaction")
	}

	// 2. Injected context
	txCtx := postgres.InjectTx(ctx, txDB)
	if !postgres.HasTx(txCtx) {
		t.Fatalf("expected HasTx(txCtx) == true after InjectTx")
	}
	extractedTx := postgres.ExtractTx(txCtx, poolDB)
	if extractedTx != txDB {
		t.Fatalf("expected txDB from ExtractTx on injected context")
	}

	// 3. Nil tx injection handling
	nilCtx := postgres.InjectTx(ctx, nil)
	if postgres.HasTx(nilCtx) {
		t.Fatalf("expected HasTx to be false when injected tx is nil")
	}
	if postgres.ExtractTx(nilCtx, poolDB) != poolDB {
		t.Fatalf("expected fallback poolDB when injected tx is nil")
	}
}

// TestEmpirical_TxManager_NestingAndErrorPropagation tests nested RunInTx behavior
func TestEmpirical_TxManager_NestingAndErrorPropagation(t *testing.T) {
	txDB := &recordingDBTX{name: "active_tx"}
	ctxWithTx := postgres.InjectTx(context.Background(), txDB)

	txMgr := postgres.NewTxManager(nil)

	// 1. Nested call should reuse existing transaction context without error
	executed := false
	err := txMgr.RunInTx(ctxWithTx, func(ctx context.Context) error {
		executed = true
		currentTx := postgres.ExtractTx(ctx, nil)
		if currentTx != txDB {
			t.Errorf("expected active transaction txDB to be preserved in nested call")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error in nested RunInTx: %v", err)
	}
	if !executed {
		t.Fatalf("expected nested function to execute")
	}

	// 2. Nested call error propagation
	expectedErr := errors.New("business logic failure inside transaction")
	err = txMgr.RunInTx(ctxWithTx, func(ctx context.Context) error {
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

// TestEmpirical_RepositoryQueryRouting tests tx vs pool context execution across repositories
func TestEmpirical_RepositoryQueryRouting(t *testing.T) {
	poolDB := &recordingDBTX{name: "pool"}
	txDB := &recordingDBTX{name: "tx"}

	t.Run("UserRepo routes queries to tx when tx in ctx, otherwise pool", func(t *testing.T) {
		repo := postgres.NewUserRepo(poolDB)
		ctxPlain := context.Background()
		ctxTx := postgres.InjectTx(ctxPlain, txDB)

		// Call GetByEmail under plain context
		_, _ = repo.GetByEmail(ctxPlain, "usr-1@test.com")
		if poolDB.queryRowCount != 1 || txDB.queryRowCount != 0 {
			t.Errorf("expected poolDB queryRowCount 1, txDB 0; got pool=%d, tx=%d", poolDB.queryRowCount, txDB.queryRowCount)
		}

		// Call GetByEmail under tx context
		_, _ = repo.GetByEmail(ctxTx, "usr-1@test.com")
		if txDB.queryRowCount != 1 {
			t.Errorf("expected txDB queryRowCount 1; got tx=%d", txDB.queryRowCount)
		}
	})

	t.Run("AgentRepo routes Exec to tx when tx in ctx", func(t *testing.T) {
		poolDB := &recordingDBTX{name: "pool"}
		txDB := &recordingDBTX{name: "tx"}
		repo := postgres.NewAgentRepo(poolDB)

		ctxTx := postgres.InjectTx(context.Background(), txDB)
		task := &agent.Task{
			ID:        "task-1",
			Phase:     "m2",
			Module:    "postgres",
			Feature:   "tx",
			Title:     "test",
			Status:    agent.TaskPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := repo.CreateTask(ctxTx, task)
		if err != nil {
			t.Fatalf("unexpected error creating agent task: %v", err)
		}

		if txDB.execCount == 0 {
			t.Errorf("expected txDB.execCount > 0, got %d", txDB.execCount)
		}
		if poolDB.execCount != 0 {
			t.Errorf("expected poolDB.execCount == 0 when tx in context, got %d", poolDB.execCount)
		}
	})

	t.Run("IncidentRepo routes Exec and Query to active tx", func(t *testing.T) {
		poolDB := &recordingDBTX{name: "pool"}
		txDB := &recordingDBTX{name: "tx"}
		repo := postgres.NewIncidentRepo(poolDB)

		ctxTx := postgres.InjectTx(context.Background(), txDB)
		inc := &incident.Incident{
			ID:          "inc-101",
			ClusterName: "c1",
			Namespace:   "default",
			PodName:     "p1",
			Type:        incident.TypeCrashLoopBackOff,
			Status:      incident.StatusDetected,
			Severity:    incident.SeverityHigh,
			Message:     "error",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		_ = repo.Create(ctxTx, inc)
		if txDB.queryRowCount != 1 || poolDB.queryRowCount != 0 {
			t.Errorf("expected incident Create on txDB queryRowCount=1, pool=0; got tx=%d, pool=%d", txDB.queryRowCount, poolDB.queryRowCount)
		}
	})
}
