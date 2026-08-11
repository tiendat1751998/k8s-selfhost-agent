package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

// trackingMockDBTX tracks calls and simulates DB behavior
type trackingMockDBTX struct {
	id             string
	execCount      int
	queryCount     int
	queryRowCount  int
	execErr        error
	queryErr       error
	mu             sync.Mutex
}

func (m *trackingMockDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	m.mu.Lock()
	m.execCount++
	err := m.execErr
	m.mu.Unlock()
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	return pgconn.NewCommandTag("EXEC 1"), nil
}

func (m *trackingMockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.mu.Lock()
	m.queryCount++
	err := m.queryErr
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &emptyRows{}, nil
}

func (m *trackingMockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.mu.Lock()
	m.queryRowCount++
	m.mu.Unlock()
	return &emptyRow{}
}

type emptyRows struct{}

func (e *emptyRows) Close()                                       {}
func (e *emptyRows) Err() error                                   { return nil }
func (e *emptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (e *emptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (e *emptyRows) Next() bool                                   { return false }
func (e *emptyRows) Scan(dest ...any) error                       { return pgx.ErrNoRows }
func (e *emptyRows) Values() ([]any, error)                       { return nil, nil }
func (e *emptyRows) RawValues() [][]byte                          { return nil }
func (e *emptyRows) Conn() *pgx.Conn                              { return nil }

type emptyRow struct{}

func (e *emptyRow) Scan(dest ...any) error { return pgx.ErrNoRows }

// 1. Test Nil Context Handling
func TestNilContextHandling(t *testing.T) {
	t.Run("HasTx with nil context", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("CONFIRMED PANIC on HasTx(nil): %v", r)
			} else {
				t.Errorf("expected HasTx(nil) to panic, but it did not")
			}
		}()
		_ = postgres.HasTx(nil)
	})

	t.Run("ExtractTx with nil context", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("CONFIRMED PANIC on ExtractTx(nil, fallback): %v", r)
			} else {
				t.Errorf("expected ExtractTx(nil, fallback) to panic, but it did not")
			}
		}()
		fallback := &trackingMockDBTX{id: "fallback"}
		_ = postgres.ExtractTx(nil, fallback)
	})

	t.Run("InjectTx with nil context", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("CONFIRMED PANIC on InjectTx(nil, tx): %v", r)
			} else {
				t.Errorf("expected InjectTx(nil, tx) to panic, but it did not")
			}
		}()
		tx := &trackingMockDBTX{id: "tx"}
		_ = postgres.InjectTx(nil, tx)
	})

	t.Run("TxManager.RunInTx with nil context", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("CONFIRMED PANIC on RunInTx(nil, fn): %v", r)
			} else {
				t.Errorf("expected RunInTx(nil, fn) to panic, but it did not")
			}
		}()
		txMgr := postgres.NewTxManager(nil)
		_ = txMgr.RunInTx(nil, func(ctx context.Context) error {
			return nil
		})
	})
}

// 2. Test Cancelled Context Handling in TxManager
func TestCancelledContextHandling(t *testing.T) {
	txMgr := postgres.NewTxManager(nil)

	t.Run("RunInTx with already cancelled context when HasTx is true", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		tx := &trackingMockDBTX{id: "active_tx"}
		txCtx := postgres.InjectTx(ctx, tx)

		executed := false
		err := txMgr.RunInTx(txCtx, func(c context.Context) error {
			executed = true
			if c.Err() == nil {
				t.Errorf("expected context to be cancelled inside fn")
			}
			return c.Err()
		})

		if !executed {
			t.Errorf("expected fn to execute even with cancelled context in nested RunInTx")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}
	})
}

// 3. Test Deeply Nested Transactions & Error Propagation
func TestNestedTransactionsEdgeCases(t *testing.T) {
	txMgr := postgres.NewTxManager(nil)
	tx := &trackingMockDBTX{id: "tx1"}
	ctx := postgres.InjectTx(context.Background(), tx)

	t.Run("3-level nested RunInTx depth", func(t *testing.T) {
		depth := 0
		err := txMgr.RunInTx(ctx, func(c1 context.Context) error {
			depth++
			return txMgr.RunInTx(c1, func(c2 context.Context) error {
				depth++
				return txMgr.RunInTx(c2, func(c3 context.Context) error {
					depth++
					if postgres.ExtractTx(c3, nil) != tx {
						return fmt.Errorf("transaction handle lost at depth 3")
					}
					return nil
				})
			})
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if depth != 3 {
			t.Errorf("expected depth 3, got %d", depth)
		}
	})

	t.Run("Inner transaction error handled by outer transaction", func(t *testing.T) {
		innerErr := errors.New("inner error")
		err := txMgr.RunInTx(ctx, func(c1 context.Context) error {
			errInner := txMgr.RunInTx(c1, func(c2 context.Context) error {
				return innerErr
			})
			if !errors.Is(errInner, innerErr) {
				t.Errorf("expected innerErr, got %v", errInner)
			}
			// Outer decides to ignore inner error
			return nil
		})

		if err != nil {
			t.Errorf("expected outer transaction to succeed when error is handled, got %v", err)
		}
	})

	t.Run("Inner transaction panic caught by outer defer recovery", func(t *testing.T) {
		// When HasTx is true, inner RunInTx does not install panic recovery, panic propagates up.
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("expected panic to propagate up from inner transaction")
			} else {
				t.Logf("Inner panic caught at top level: %v", r)
			}
		}()

		_ = txMgr.RunInTx(ctx, func(c1 context.Context) error {
			return txMgr.RunInTx(c1, func(c2 context.Context) error {
				panic("inner failure panic")
			})
		})
	})
}

// 4. Test Concurrent Access / Race Conditions
func TestContextTxConcurrency(t *testing.T) {
	tx := &trackingMockDBTX{id: "concurrent_tx"}
	ctx := postgres.InjectTx(context.Background(), tx)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				has := postgres.HasTx(ctx)
				if !has {
					t.Errorf("goroutine %d: HasTx returned false", id)
				}
				extracted := postgres.ExtractTx(ctx, nil)
				if extracted != tx {
					t.Errorf("goroutine %d: ExtractTx returned wrong tx", id)
				}
				// Simulate repository getDB call
				extDB := postgres.ExtractTx(ctx, nil)
				_, _ = extDB.Exec(ctx, "SELECT 1")
			}
		}(i)
	}
	wg.Wait()
}

// 5. Test Repository DBTX Extraction for all repositories
func TestAllRepositoriesDBTXExtraction(t *testing.T) {
	fallbackDB := &trackingMockDBTX{id: "fallback"}
	txDB := &trackingMockDBTX{id: "tx"}

	bgCtx := context.Background()
	txCtx := postgres.InjectTx(bgCtx, txDB)

	t.Run("AgentRepo getDB fallback vs tx", func(t *testing.T) {
		repo := postgres.NewAgentRepo(fallbackDB)
		// Call ListTasks with bgCtx (should use fallbackDB)
		_, _ = repo.ListTasks(bgCtx)
		if fallbackDB.queryCount != 1 {
			t.Errorf("expected fallbackDB queryCount=1, got %d", fallbackDB.queryCount)
		}
		if txDB.queryCount != 0 {
			t.Errorf("expected txDB queryCount=0, got %d", txDB.queryCount)
		}

		// Call ListTasks with txCtx (should use txDB)
		_, _ = repo.ListTasks(txCtx)
		if txDB.queryCount != 1 {
			t.Errorf("expected txDB queryCount=1, got %d", txDB.queryCount)
		}
	})

	t.Run("IncidentRepo getDB fallback vs tx", func(t *testing.T) {
		fb := &trackingMockDBTX{id: "fb_inc"}
		tx := &trackingMockDBTX{id: "tx_inc"}
		repo := postgres.NewIncidentRepo(fb)

		_, _, _ = repo.List(bgCtx, incident.Filter{})
		if fb.queryRowCount+fb.queryCount != 1 {
			t.Errorf("expected fb queryCount/queryRowCount=1, got count=%d, rowCount=%d", fb.queryCount, fb.queryRowCount)
		}

		_, _, _ = repo.List(postgres.InjectTx(bgCtx, tx), incident.Filter{})
		if tx.queryRowCount+tx.queryCount != 1 {
			t.Errorf("expected tx queryCount/queryRowCount=1, got count=%d, rowCount=%d", tx.queryCount, tx.queryRowCount)
		}
	})

	t.Run("UserRepo getDB fallback vs tx", func(t *testing.T) {
		fb := &trackingMockDBTX{id: "fb_user"}
		tx := &trackingMockDBTX{id: "tx_user"}
		repo := postgres.NewUserRepo(fb)

		_, _ = repo.GetByEmail(bgCtx, "test@example.com")
		if fb.queryRowCount != 1 {
			t.Errorf("expected fb queryRowCount=1, got %d", fb.queryRowCount)
		}

		_, _ = repo.GetByEmail(postgres.InjectTx(bgCtx, tx), "test@example.com")
		if tx.queryRowCount != 1 {
			t.Errorf("expected tx queryRowCount=1, got %d", tx.queryRowCount)
		}
	})
}
