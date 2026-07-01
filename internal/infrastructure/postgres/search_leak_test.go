package postgres_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

func TestSearchRepo_ConnectionLeak(t *testing.T) {
	// Set environment variables to point to the correct DB configuration
	os.Setenv("K8S_POSTGRES_HOST", "10.10.10.133")
	os.Setenv("K8S_POSTGRES_PORT", "5432")
	os.Setenv("K8S_POSTGRES_USER", "myuser")
	os.Setenv("K8S_POSTGRES_PASSWORD", "mysecretpassword")
	os.Setenv("K8S_POSTGRES_DBNAME", "mydatabase")
	os.Setenv("K8S_POSTGRES_SSLMODE", "disable")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config load failed: %v", err)
	}

	// Adjust database pool configuration to have a small number of MaxConns (e.g. 3)
	// to make connection exhaustion easy to detect.
	poolCfg, err := pgxpool.ParseConfig(cfg.Postgres.DSN())
	if err != nil {
		t.Fatalf("Failed to parse DSN: %v", err)
	}
	poolCfg.MaxConns = 3
	poolCfg.MinConns = 1

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	// Ensure database connection is alive
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("database ping failed: %v", err)
	}

	repo := postgres.NewSearchRepo(pool)

	// We will run 30 concurrent search queries.
	// Since MaxConns is 3, if any query leaks a connection, subsequent queries will hang
	// and eventually timeout.
	var wg sync.WaitGroup
	numWorkers := 30
	errChan := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// Some workers will have cancelled contexts to simulate abrupt client disconnection
			var workerCtx context.Context
			var workerCancel context.CancelFunc
			if workerID%5 == 0 {
				workerCtx, workerCancel = context.WithTimeout(ctx, 2*time.Millisecond)
			} else {
				workerCtx, workerCancel = context.WithTimeout(ctx, 3*time.Second)
			}
			defer workerCancel()

			_, err := repo.Search(workerCtx, "test", "all")
			if err != nil {
				// Context cancellation/timeout is expected for some, but not connection exhaustion timeouts
				if workerCtx.Err() == context.DeadlineExceeded || workerCtx.Err() == context.Canceled {
					return
				}
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Worker returned error: %v", err)
	}

	// Verify that the acquired connections return to 0 (or min connections) after all queries finish
	acquired := pool.Stat().AcquiredConns()
	if acquired > 0 {
		t.Errorf("Connection leak detected: %d connections are still acquired in the pool", acquired)
	} else {
		t.Logf("No connection leak detected. Acquired connections: %d", acquired)
	}
}
