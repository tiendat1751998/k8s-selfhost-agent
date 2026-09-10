package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

func TestBuildTenantQuery_AuditAgentAutomation(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-sec-123")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "operator")

	t.Run("audit_findings SELECT without WHERE", func(t *testing.T) {
		q := "SELECT id, category, severity, description FROM audit_findings ORDER BY detected_at DESC"
		rewritten, args := postgres.BuildTenantQuery(ctx, q)
		if !strings.Contains(rewritten, "WHERE tenant_id = $1 ORDER BY") {
			t.Errorf("expected tenant_id filter before ORDER BY, got: %s", rewritten)
		}
		if len(args) != 1 || args[0] != "tenant-sec-123" {
			t.Errorf("expected ['tenant-sec-123'], got: %v", args)
		}
	})

	t.Run("audit_findings SELECT with WHERE", func(t *testing.T) {
		q := "SELECT id, category FROM audit_findings WHERE status = $1 ORDER BY detected_at DESC"
		rewritten, args := postgres.BuildTenantQuery(ctx, q, "open")
		if !strings.Contains(rewritten, "AND tenant_id = $2 ORDER BY") {
			t.Errorf("expected tenant_id filter after WHERE and before ORDER BY, got: %s", rewritten)
		}
		if len(args) != 2 || args[1] != "tenant-sec-123" {
			t.Errorf("expected args ['open', 'tenant-sec-123'], got: %v", args)
		}
	})

	t.Run("audit_findings UPDATE", func(t *testing.T) {
		q := "UPDATE audit_findings SET status = 'resolved' WHERE id = $1"
		rewritten, args := postgres.BuildTenantQuery(ctx, q, "find-1")
		if !strings.Contains(rewritten, "AND tenant_id = $2") {
			t.Errorf("expected AND tenant_id = $2, got: %s", rewritten)
		}
		if len(args) != 2 || args[1] != "tenant-sec-123" {
			t.Errorf("expected args ['find-1', 'tenant-sec-123'], got: %v", args)
		}
	})

	t.Run("audit_runs SELECT", func(t *testing.T) {
		q := "SELECT id, status FROM audit_runs ORDER BY start_time DESC LIMIT 1"
		rewritten, args := postgres.BuildTenantQuery(ctx, q)
		if !strings.Contains(rewritten, "WHERE tenant_id = $1 ORDER BY") {
			t.Errorf("expected WHERE tenant_id = $1, got: %s", rewritten)
		}
		if len(args) != 1 || args[0] != "tenant-sec-123" {
			t.Errorf("expected args ['tenant-sec-123'], got: %v", args)
		}
	})

	t.Run("agent_tasks SELECT by ID", func(t *testing.T) {
		q := "SELECT id, phase, module FROM agent_tasks WHERE id = $1"
		rewritten, args := postgres.BuildTenantQuery(ctx, q, "task-1")
		if !strings.Contains(rewritten, "AND tenant_id = $2") {
			t.Errorf("expected AND tenant_id = $2, got: %s", rewritten)
		}
		if len(args) != 2 || args[1] != "tenant-sec-123" {
			t.Errorf("expected args ['task-1', 'tenant-sec-123'], got: %v", args)
		}
	})

	t.Run("agent_tasks UPDATE", func(t *testing.T) {
		q := "UPDATE agent_tasks SET phase = $1, status = $2 WHERE id = $3"
		rewritten, args := postgres.BuildTenantQuery(ctx, q, "p1", "done", "task-1")
		if !strings.Contains(rewritten, "AND tenant_id = $4") {
			t.Errorf("expected AND tenant_id = $4, got: %s", rewritten)
		}
		if len(args) != 4 || args[3] != "tenant-sec-123" {
			t.Errorf("expected args len 4 with tenant-sec-123 at end, got: %v", args)
		}
	})

	t.Run("automation_rules DELETE", func(t *testing.T) {
		q := "DELETE FROM automation_rules WHERE id = $1"
		rewritten, args := postgres.BuildTenantQuery(ctx, q, "rule-1")
		if !strings.Contains(rewritten, "AND tenant_id = $2") {
			t.Errorf("expected AND tenant_id = $2, got: %s", rewritten)
		}
		if len(args) != 2 || args[1] != "tenant-sec-123" {
			t.Errorf("expected args ['rule-1', 'tenant-sec-123'], got: %v", args)
		}
	})

	t.Run("automation_executions COUNT", func(t *testing.T) {
		q := "SELECT COUNT(*) FROM automation_executions"
		rewritten, args := postgres.BuildTenantQuery(ctx, q)
		if !strings.Contains(rewritten, "WHERE tenant_id = $1") {
			t.Errorf("expected WHERE tenant_id = $1, got: %s", rewritten)
		}
		if len(args) != 1 || args[0] != "tenant-sec-123" {
			t.Errorf("expected args ['tenant-sec-123'], got: %v", args)
		}
	})
}
