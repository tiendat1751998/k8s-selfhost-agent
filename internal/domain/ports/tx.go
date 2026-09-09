package ports

import "context"

// TransactionManager defines the domain interface for executing units of work within transactions.
type TransactionManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}
