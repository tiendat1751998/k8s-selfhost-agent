package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

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
}
