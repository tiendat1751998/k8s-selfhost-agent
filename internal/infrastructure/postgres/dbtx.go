package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX defines the database execution interface common to *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Compile-time interface assertion
var (
	_ DBTX = (*pgxpool.Pool)(nil)
	_ DBTX = (pgx.Tx)(nil)
)

type txKey struct{}

// InjectTx returns a new context containing the active transaction.
func InjectTx(ctx context.Context, tx DBTX) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// ExtractTx returns the active transaction from context if present; otherwise returns fallback.
func ExtractTx(ctx context.Context, fallback DBTX) DBTX {
	if tx, ok := ctx.Value(txKey{}).(DBTX); ok && tx != nil {
		return tx
	}
	return fallback
}

// HasTx checks whether context currently carries an active transaction.
func HasTx(ctx context.Context) bool {
	tx, ok := ctx.Value(txKey{}).(DBTX)
	return ok && tx != nil
}
