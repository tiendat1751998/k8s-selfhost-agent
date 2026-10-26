package postgres_test

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/domain/automation"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockRow struct {
	scanFn func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.scanFn != nil {
		return m.scanFn(dest...)
	}
	return nil
}

type recordingAutomationDB struct {
	lastSQL  string
	lastArgs []any
	scanID   string
}

func (r *recordingAutomationDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (r *recordingAutomationDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (r *recordingAutomationDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	r.lastSQL = sql
	r.lastArgs = args
	return &mockRow{
		scanFn: func(dest ...any) error {
			if len(dest) > 0 {
				if idPtr, ok := dest[0].(*string); ok {
					if r.scanID != "" {
						*idPtr = r.scanID
					} else if len(args) > 0 {
						if idArg, ok := args[0].(string); ok {
							*idPtr = idArg
						}
					}
				}
			}
			return nil
		},
	}
}

func TestAutomationRepo_CreateExecution_UUIDAndTenantHandling(t *testing.T) {
	ruleID := "e1000001-0000-0000-0000-000000000001"

	t.Run("generates valid UUID for Execution.ID and defaults tenant_id to default-tenant when empty", func(t *testing.T) {
		db := &recordingAutomationDB{}
		repo := postgres.NewAutomationRepo(db)

		exec := &automation.Execution{
			RuleID:       ruleID,
			RuleName:     "Auto-Reap Zombie Processes",
			TriggerEvent: "manual_trigger",
			ActionTaken:  "reap_zombies",
			Result:       "success",
		}

		ctx := context.Background() // empty tenant
		err := repo.CreateExecution(ctx, exec)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify Execution.ID was generated as a valid UUID
		if exec.ID == "" {
			t.Fatal("expected exec.ID to be generated, got empty")
		}
		if _, err := uuid.Parse(exec.ID); err != nil {
			t.Fatalf("expected valid UUID for exec.ID, got %q: %v", exec.ID, err)
		}

		// Verify SQL includes id column in INSERT
		if !strings.Contains(db.lastSQL, "INSERT INTO automation_executions (id, rule_id") {
			t.Fatalf("expected SQL to insert id column explicitly, got: %s", db.lastSQL)
		}

		// Verify lastArgs count is 9: id, rule_id, rule_name, trigger_event, action_taken, result, error_detail, created_at, tenant_id
		if len(db.lastArgs) != 9 {
			t.Fatalf("expected 9 arguments, got %d: %v", len(db.lastArgs), db.lastArgs)
		}

		// Verify arg[0] is the valid UUID
		if db.lastArgs[0] != exec.ID {
			t.Fatalf("expected arg[0] to match exec.ID %q, got: %v", exec.ID, db.lastArgs[0])
		}

		// Verify error_detail (arg[6]) is non-nil string (TEXT NOT NULL DEFAULT '')
		errDetailStr, ok := db.lastArgs[6].(string)
		if !ok || errDetailStr != "" {
			t.Fatalf("expected non-nil string '' for error_detail, got: %v (%T)", db.lastArgs[6], db.lastArgs[6])
		}

		// Verify tenant_id (arg[8]) is "default-tenant" string (VARCHAR(255) NOT NULL)
		tenantStr, ok := db.lastArgs[8].(string)
		if !ok || tenantStr != "default-tenant" {
			t.Fatalf("expected string 'default-tenant' for empty tenant context, got: %v (%T)", db.lastArgs[8], db.lastArgs[8])
		}
	})

	t.Run("passes string default-tenant for default-tenant context", func(t *testing.T) {
		db := &recordingAutomationDB{}
		repo := postgres.NewAutomationRepo(db)

		exec := &automation.Execution{
			RuleID:       ruleID,
			RuleName:     "Auto-Reap Zombie Processes",
			TriggerEvent: "manual_trigger",
			ActionTaken:  "reap_zombies",
			Result:       "success",
		}

		ctx := tenancy.WithTenantID(context.Background(), "default-tenant")
		err := repo.CreateExecution(ctx, exec)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(db.lastArgs) != 9 {
			t.Fatalf("expected 9 arguments, got %d: %v", len(db.lastArgs), db.lastArgs)
		}

		// Verify tenant_id (arg[8]) is "default-tenant" string
		tenantStr, ok := db.lastArgs[8].(string)
		if !ok || tenantStr != "default-tenant" {
			t.Fatalf("expected string 'default-tenant' for default-tenant context, got: %v (%T)", db.lastArgs[8], db.lastArgs[8])
		}
	})

	t.Run("passes custom tenant string when tenant is provided", func(t *testing.T) {
		db := &recordingAutomationDB{}
		repo := postgres.NewAutomationRepo(db)

		validTenantUUID := "a0000000-0000-0000-0000-000000000001"
		exec := &automation.Execution{
			RuleID:       ruleID,
			RuleName:     "Auto-Reap Zombie Processes",
			TriggerEvent: "manual_trigger",
			ActionTaken:  "reap_zombies",
			Result:       "success",
		}

		ctx := tenancy.WithTenantID(context.Background(), validTenantUUID)
		err := repo.CreateExecution(ctx, exec)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(db.lastArgs) != 9 {
			t.Fatalf("expected 9 arguments, got %d: %v", len(db.lastArgs), db.lastArgs)
		}

		tenantStr, ok := db.lastArgs[8].(string)
		if !ok || tenantStr != validTenantUUID {
			t.Fatalf("expected string %q for tenant context, got: %v (%T)", validTenantUUID, db.lastArgs[8], db.lastArgs[8])
		}
	})

	t.Run("preserves pre-set Execution.ID", func(t *testing.T) {
		db := &recordingAutomationDB{}
		repo := postgres.NewAutomationRepo(db)

		customID := "f0000000-0000-0000-0000-000000000042"
		exec := &automation.Execution{
			ID:           customID,
			RuleID:       ruleID,
			RuleName:     "Auto-Reap Zombie Processes",
			TriggerEvent: "manual_trigger",
			ActionTaken:  "reap_zombies",
			Result:       "success",
			CreatedAt:    time.Now().UTC(),
		}

		err := repo.CreateExecution(context.Background(), exec)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if exec.ID != customID {
			t.Fatalf("expected exec.ID to remain %q, got %q", customID, exec.ID)
		}
		if len(db.lastArgs) > 0 && db.lastArgs[0] != customID {
			t.Fatalf("expected arg[0] to be %q, got: %v", customID, db.lastArgs[0])
		}
	})

	t.Run("passes error_detail string when execution has error", func(t *testing.T) {
		db := &recordingAutomationDB{}
		repo := postgres.NewAutomationRepo(db)

		expectedErr := "pod restart timed out after 30s"
		exec := &automation.Execution{
			RuleID:       ruleID,
			RuleName:     "Auto-Reap Zombie Processes",
			TriggerEvent: "manual_trigger",
			ActionTaken:  "reap_zombies",
			Result:       "failed",
			ErrorDetail:  expectedErr,
		}

		err := repo.CreateExecution(context.Background(), exec)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		errDetailStr, ok := db.lastArgs[6].(string)
		if !ok || errDetailStr != expectedErr {
			t.Fatalf("expected string %q for error_detail, got: %v (%T)", expectedErr, db.lastArgs[6], db.lastArgs[6])
		}
	})
}

func getLiveAutomationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	// Read .env if direct env vars are empty (never log or print secrets)
	if os.Getenv("POSTGRES_PASSWORD") == "" && os.Getenv("K8S_POSTGRES_PASSWORD") == "" {
		if f, err := os.Open(`d:\project\k8sseflhost\.env`); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					k := strings.TrimSpace(parts[0])
					v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					if os.Getenv(k) == "" {
						os.Setenv(k, v)
					}
				}
			}
			f.Close()
		}
	}

	host := os.Getenv("K8S_POSTGRES_HOST")
	if host == "" {
		host = "10.10.10.133"
	}
	port := os.Getenv("K8S_POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("K8S_POSTGRES_USER")
	if user == "" {
		user = "myuser"
	}
	pass := os.Getenv("POSTGRES_PASSWORD")
	if pass == "" {
		pass = os.Getenv("K8S_POSTGRES_PASSWORD")
	}
	if pass == "" {
		t.Skip("Skipping live database test: POSTGRES_PASSWORD not set")
		return nil
	}
	dbname := os.Getenv("K8S_POSTGRES_DBNAME")
	if dbname == "" {
		dbname = "mydatabase"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbname)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Skipping live database test: unable to connect: %v", err)
		return nil
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("Skipping live database test: unable to ping: %v", err)
		return nil
	}
	return pool
}

func TestAutomationRepo_LiveDB_CreateAndListExecution(t *testing.T) {
	pool := getLiveAutomationPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	repo := postgres.NewAutomationRepo(pool)
	ruleID := "e1000001-0000-0000-0000-000000000001"
	ctx := tenancy.WithTenantID(context.Background(), "default-tenant")

	exec := &automation.Execution{
		ID:           uuid.NewString(),
		RuleID:       ruleID,
		RuleName:     "Auto-Reap Zombie Processes",
		TriggerEvent: "Manual SRE 1-Click trigger live test",
		ActionTaken:  "Dispatched automated self-healing action: restart_pod",
		Result:       "success",
		ErrorDetail:  "", // EMPTY string (verifies error_detail TEXT NOT NULL constraint)
	}

	err := repo.CreateExecution(ctx, exec)
	if err != nil {
		t.Fatalf("CreateExecution failed against live PostgreSQL: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM automation_executions WHERE id = $1", exec.ID)
	}()

	if exec.ID == "" {
		t.Fatal("expected exec.ID to be non-empty")
	}

	execs, total, err := repo.ListExecutions(ctx, 20, 0)
	if err != nil {
		t.Fatalf("ListExecutions failed against live PostgreSQL: %v", err)
	}
	if total == 0 {
		t.Fatal("expected total executions count to be > 0")
	}

	found := false
	for _, e := range execs {
		if e.ID == exec.ID {
			found = true
			if e.RuleID != ruleID {
				t.Errorf("expected RuleID %s, got %s", ruleID, e.RuleID)
			}
			if e.ErrorDetail != "" {
				t.Errorf("expected empty ErrorDetail, got %q", e.ErrorDetail)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected created execution %s in ListExecutions result", exec.ID)
	}
}
