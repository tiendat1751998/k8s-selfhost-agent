package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TransactionManager defines the contract for executing transactional units of work.
type TransactionManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
	RunInTxWithOpts(ctx context.Context, opts pgx.TxOptions, fn func(ctx context.Context) error) error
}

// TxManager implements TransactionManager using pgxpool.Pool.
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager creates a new TxManager bound to a pgxpool.Pool.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// RunInTx executes fn within a PostgreSQL transaction with default options.
func (m *TxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.RunInTxWithOpts(ctx, pgx.TxOptions{}, fn)
}

// RunInTxWithOpts executes fn within a PostgreSQL transaction using specified TxOptions.
func (m *TxManager) RunInTxWithOpts(ctx context.Context, opts pgx.TxOptions, fn func(ctx context.Context) error) error {
	// Re-use transaction if context already has an active transaction (nesting protection)
	if HasTx(ctx) {
		return fn(ctx)
	}

	tx, err := m.pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txCtx := InjectTx(ctx, tx)

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // re-throw panic after rollback
		}
	}()

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("tx error: %v, rollback error: %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
