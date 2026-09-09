package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestWave5_LegacyTablesNowTenantScoped(t *testing.T) {
	ctx := context.WithValue(context.Background(), tenancy.TenantIDKey, "tenant-test-123")
	ctx = context.WithValue(ctx, tenancy.UserRoleKey, "operator")

	tables := []string{
		"agent_tasks",
		"agent_subtasks",
		"agent_executions",
		"agent_project_state",
		"audit_findings",
		"audit_runs",
		"audit_logs",
		"automation_rules",
		"automation_executions",
	}

	for _, tbl := range tables {
		t.Run("Table "+tbl+" is rewritten with tenant_id", func(t *testing.T) {
			rawSQL := "SELECT * FROM " + tbl + " WHERE status = $1"
			rewrittenSQL, args := postgres.BuildTenantQuery(ctx, rawSQL, "active")

			if !strings.Contains(rewrittenSQL, "tenant_id = $2") {
				t.Fatalf("expected tenant_id filter added for table %s, got: %s", tbl, rewrittenSQL)
			}
			if len(args) != 2 || args[1] != "tenant-test-123" {
				t.Fatalf("expected args to include tenantID 'tenant-test-123', got: %v", args)
			}
		})
	}
}

func TestWave5_PlatformAdminBypassesTenantScoping(t *testing.T) {
	ctx := context.WithValue(context.Background(), tenancy.TenantIDKey, "tenant-test-123")
	ctx = context.WithValue(ctx, tenancy.UserRoleKey, "platform_admin")

	rawSQL := "SELECT * FROM audit_findings WHERE status = $1"
	rewrittenSQL, args := postgres.BuildTenantQuery(ctx, rawSQL, "open")

	if strings.Contains(rewrittenSQL, "tenant_id") {
		t.Fatalf("platform_admin should not have tenant_id filter added, got: %s", rewrittenSQL)
	}
	if len(args) != 1 || args[0] != "open" {
		t.Fatalf("expected original args untouched for platform_admin, got: %v", args)
	}
}

func TestWave5_ObservabilityRepoTenantScoping(t *testing.T) {
	pool := &recordingDBTX{name: "obs_test_pool"}
	repo := postgres.NewObservabilityRepo(pool)

	ctxTenant := context.WithValue(context.Background(), tenancy.TenantIDKey, "a0000000-0000-0000-0000-000000000099")
	ctxTenant = context.WithValue(ctxTenant, tenancy.UserRoleKey, "operator")

	// 1. GetHealthSamples under tenant context
	_, _ = repo.GetHealthSamples(ctxTenant, "payment_svc", time.Now().Add(-1*time.Hour))
	if !strings.Contains(pool.lastSQL, "tenant_id = $3") {
		t.Fatalf("expected GetHealthSamples to parameterize tenant_id = $3, got SQL: %s", pool.lastSQL)
	}

	// 2. ComputeSLI under tenant context
	_, _ = repo.ComputeSLI(ctxTenant, "payment_svc", 1*time.Hour)
	if !strings.Contains(pool.lastSQL, "tenant_id = $3") {
		t.Fatalf("expected ComputeSLI to parameterize tenant_id = $3, got SQL: %s", pool.lastSQL)
	}

	// 3. RecordHealthSample extracts tenant from context when empty
	_ = repo.RecordHealthSample(ctxTenant, "", "payment_svc", 3, 3, true, nil)
	if !strings.Contains(pool.lastSQL, "INSERT INTO slo_health_samples") {
		t.Fatalf("expected RecordHealthSample insert SQL, got: %s", pool.lastSQL)
	}

	// 4. Platform admin bypasses tenant filter
	ctxAdmin := context.WithValue(context.Background(), tenancy.TenantIDKey, "a0000000-0000-0000-0000-000000000099")
	ctxAdmin = context.WithValue(ctxAdmin, tenancy.UserRoleKey, "platform_admin")
	_, _ = repo.GetHealthSamples(ctxAdmin, "payment_svc", time.Now().Add(-1*time.Hour))
	if strings.Contains(pool.lastSQL, "tenant_id = $3") {
		t.Fatalf("expected platform_admin to query across all tenants, got SQL: %s", pool.lastSQL)
	}
}

func TestWave5_NodeMetricsRepoTenantScoping(t *testing.T) {
	pool := &recordingDBTX{name: "metrics_test_pool"}
	repo := postgres.NewNodeMetricsRepo(pool)

	ctxTenant := context.WithValue(context.Background(), tenancy.TenantIDKey, "a0000000-0000-0000-0000-000000000099")
	ctxTenant = context.WithValue(ctxTenant, tenancy.UserRoleKey, "operator")

	// 1. QueryHistory 1m under tenant context
	_, _ = repo.QueryHistory(ctxTenant, nodemetrics.NodeHistoryQuery{
		NodeID:     "node-1",
		Resolution: "1m",
	})
	if !strings.Contains(pool.lastSQL, "tenant_id = $2") {
		t.Fatalf("expected QueryHistory 1m to parameterize tenant_id = $2, got SQL: %s", pool.lastSQL)
	}

	// 2. QueryHistory 1h under tenant context
	_, _ = repo.QueryHistory(ctxTenant, nodemetrics.NodeHistoryQuery{
		NodeID:     "node-1",
		Resolution: "1h",
	})
	if !strings.Contains(pool.lastSQL, "tenant_id = $2") {
		t.Fatalf("expected QueryHistory 1h to parameterize tenant_id = $2, got SQL: %s", pool.lastSQL)
	}

	// 3. GetSummary under tenant context
	_, _ = repo.GetSummary(ctxTenant, nodemetrics.NodeHistoryQuery{
		NodeID: "node-1",
	})
	if !strings.Contains(pool.lastSQL, "tenant_id = $2") {
		t.Fatalf("expected GetSummary to parameterize tenant_id = $2, got SQL: %s", pool.lastSQL)
	}

	// 4. Platform admin bypasses tenant filter
	ctxAdmin := context.WithValue(context.Background(), tenancy.TenantIDKey, "a0000000-0000-0000-0000-000000000099")
	ctxAdmin = context.WithValue(ctxAdmin, tenancy.UserRoleKey, "platform_admin")
	_, _ = repo.QueryHistory(ctxAdmin, nodemetrics.NodeHistoryQuery{
		NodeID:     "node-1",
		Resolution: "1m",
	})
	if strings.Contains(pool.lastSQL, "tenant_id = $2") {
		t.Fatalf("expected platform_admin QueryHistory to query without tenant filter, got SQL: %s", pool.lastSQL)
	}
}
