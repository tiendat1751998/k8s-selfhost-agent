package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSearchRepo_ConnectionLeaks(t *testing.T) {
	// The database DSN from config.yaml / check_db script
	dsn := "postgres://myuser:mysecretpassword@10.10.10.133:5432/mydatabase?sslmode=disable"
	ctx := context.Background()

	// Parse and configure pool to have a small MaxConns to easily trigger leaks/exhaustion
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("failed to parse DSN: %v", err)
	}
	poolCfg.MaxConns = 5
	poolCfg.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	defer pool.Close()

	repo := NewSearchRepo(pool)

	// Ensure the pool is working
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping test database: %v", err)
	}

	initialAcquired := pool.Stat().AcquiredConns()
	t.Logf("Initial acquired connections: %d", initialAcquired)

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			var runCtx context.Context
			var cancel context.CancelFunc

			// Alternate contexts: some succeed, some cancel immediately, some time out
			switch id % 3 {
			case 0:
				// Successful context
				runCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			case 1:
				// Immediate cancellation
				runCtx, cancel = context.WithCancel(context.Background())
				cancel() // cancel immediately
			case 2:
				// Short timeout (which might expire during query)
				runCtx, cancel = context.WithTimeout(context.Background(), 2*time.Millisecond)
			}
			defer cancel()

			_, _ = repo.Search(runCtx, "prod", "all")
		}(i)
	}

	wg.Wait()

	// Wait a moment for connection pool to settle and reclaim closed connections
	time.Sleep(100 * time.Millisecond)

	finalAcquired := pool.Stat().AcquiredConns()
	t.Logf("Final acquired connections: %d", finalAcquired)

	if finalAcquired > initialAcquired {
		t.Errorf("DB Connection Leak Detected! Initial acquired: %d, final acquired: %d", initialAcquired, finalAcquired)
	} else {
		t.Log("No connection leak detected under concurrency and cancellation stress.")
	}
}
