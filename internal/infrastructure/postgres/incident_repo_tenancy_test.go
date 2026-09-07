package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type capturingDBTX struct {
	lastQuery string
	lastArgs  []any
}

func (c *capturingDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	c.lastQuery = sql
	c.lastArgs = arguments
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (c *capturingDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	c.lastQuery = sql
	c.lastArgs = args
	return nil, nil
}

func (c *capturingDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	c.lastQuery = sql
	c.lastArgs = args
	return &dummyRow{}
}

func TestIncidentRepo_GetByPodAndType_TenancyScoping(t *testing.T) {
	t.Run("defaults to default-tenant when context has no tenant and no role", func(t *testing.T) {
		db := &capturingDBTX{}
		repo := postgres.NewIncidentRepo(db)

		ctx := context.Background()
		_, _ = repo.GetByPodAndType(ctx, "infrastructure", "worker1", incident.TypeNodeNotReady)

		if !strings.Contains(db.lastQuery, "tenant_id") {
			t.Fatalf("expected query to contain tenant_id, got: %s", db.lastQuery)
		}

		// The last argument should be 'default-tenant'
		foundDefault := false
		for _, arg := range db.lastArgs {
			if s, ok := arg.(string); ok && s == "default-tenant" {
				foundDefault = true
				break
			}
		}
		if !foundDefault {
			t.Fatalf("expected 'default-tenant' in query arguments, got: %#v", db.lastArgs)
		}
	})

	t.Run("preserves explicit tenant ID", func(t *testing.T) {
		db := &capturingDBTX{}
		repo := postgres.NewIncidentRepo(db)

		ctx := tenancy.WithTenantID(context.Background(), "custom-tenant-42")
		_, _ = repo.GetByPodAndType(ctx, "infrastructure", "worker1", incident.TypeNodeNotReady)

		if !strings.Contains(db.lastQuery, "tenant_id") {
			t.Fatalf("expected query to contain tenant_id, got: %s", db.lastQuery)
		}

		foundCustom := false
		for _, arg := range db.lastArgs {
			if s, ok := arg.(string); ok && s == "custom-tenant-42" {
				foundCustom = true
				break
			}
		}
		if !foundCustom {
			t.Fatalf("expected 'custom-tenant-42' in query arguments, got: %#v", db.lastArgs)
		}
	})

	t.Run("bypasses tenant filter for platform_admin", func(t *testing.T) {
		db := &capturingDBTX{}
		repo := postgres.NewIncidentRepo(db)

		ctx := tenancy.WithUserRole(context.Background(), "platform_admin")
		_, _ = repo.GetByPodAndType(ctx, "infrastructure", "worker1", incident.TypeNodeNotReady)

		if strings.Contains(db.lastQuery, "tenant_id") {
			t.Fatalf("expected query NOT to contain tenant_id for platform_admin, got: %s", db.lastQuery)
		}
	})
}

func TestIncidentRepo_Update_TenancyScoping(t *testing.T) {
	t.Run("defaults to default-tenant when context has no tenant", func(t *testing.T) {
		db := &capturingDBTX{}
		repo := postgres.NewIncidentRepo(db)

		inc, err := incident.New("fleet-1", "infrastructure", "worker1", incident.TypeNodeNotReady, incident.SeverityCritical, "host down")
		if err != nil {
			t.Fatalf("failed to create incident: %v", err)
		}
		inc.ID = "inc-test-uuid"

		ctx := context.Background()
		_ = repo.Update(ctx, inc)

		if !strings.Contains(db.lastQuery, "tenant_id") {
			t.Fatalf("expected update query to contain tenant_id, got: %s", db.lastQuery)
		}

		foundDefault := false
		for _, arg := range db.lastArgs {
			if s, ok := arg.(string); ok && s == "default-tenant" {
				foundDefault = true
				break
			}
		}
		if !foundDefault {
			t.Fatalf("expected 'default-tenant' in update arguments, got: %#v", db.lastArgs)
		}
	})
}
