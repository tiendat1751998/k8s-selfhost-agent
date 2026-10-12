package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/datdt/k8sselfhost/internal/domain/capacity"
	"github.com/datdt/k8sselfhost/internal/domain/changes"
	"github.com/datdt/k8sselfhost/internal/domain/compliance"
	"github.com/datdt/k8sselfhost/internal/domain/correlation"
	"github.com/datdt/k8sselfhost/internal/domain/drift"
	"github.com/datdt/k8sselfhost/internal/domain/notification"
	"github.com/datdt/k8sselfhost/internal/domain/observability"
	"github.com/datdt/k8sselfhost/internal/domain/promotion"
	"github.com/datdt/k8sselfhost/internal/domain/reporting"
	"github.com/datdt/k8sselfhost/internal/domain/tagging"
	domaintenancy "github.com/datdt/k8sselfhost/internal/domain/tenancy"
	"github.com/datdt/k8sselfhost/internal/domain/timeline"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	pkgtenancy "github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func assertHasArg(t *testing.T, args []any, expected any, msg string) {
	t.Helper()
	for _, a := range args {
		if a == expected {
			return
		}
	}
	t.Fatalf("%s: expected arg %v in %v", msg, expected, args)
}

func assertNotHasArg(t *testing.T, args []any, unexpected any, msg string) {
	t.Helper()
	for _, a := range args {
		if a == unexpected {
			t.Fatalf("%s: unexpected arg %v in %v", msg, unexpected, args)
		}
	}
}

func TestCrossTenantAccess_StrictIsolation(t *testing.T) {
	pool := &recordingDBTX{name: "isolation_test_pool"}

	ctxTenantA := pkgtenancy.WithTenantID(context.Background(), "tenant-alpha-001")
	ctxTenantA = pkgtenancy.WithUserRole(ctxTenantA, "user")

	ctxTenantB := pkgtenancy.WithTenantID(context.Background(), "tenant-beta-002")
	ctxTenantB = pkgtenancy.WithUserRole(ctxTenantB, "user")

	t.Run("CorrelationRepo Isolation", func(t *testing.T) {
		repo := postgres.NewCorrelationRepo(pool)

		// 1. List under Tenant A
		_, _, _ = repo.ListCorrelated(ctxTenantA, "active", 10, 0)
		if !strings.Contains(pool.lastSQL, "tenant_id") {
			t.Fatalf("ListCorrelated expected tenant_id filter, got: %s", pool.lastSQL)
		}
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A list")
		assertNotHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant A list")

		// 2. GetByID under Tenant B
		_, _ = repo.GetByID(ctxTenantB, "event-001")
		if !strings.Contains(pool.lastSQL, "tenant_id") {
			t.Fatalf("GetByID expected tenant_id filter, got: %s", pool.lastSQL)
		}
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B get")
		assertNotHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant B get")

		// 3. Create under Tenant A
		_ = repo.Create(ctxTenantA, &correlation.CorrelatedEvent{Title: "Issue", Severity: "high"})
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A create")
		assertNotHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant A create")

		// 4. Resolve under Tenant B
		_ = repo.Resolve(ctxTenantB, "event-001")
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B resolve")
		assertNotHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant B resolve")
	})

	t.Run("ChangesRepo Isolation", func(t *testing.T) {
		repo := postgres.NewChangesRepo(pool)

		// List under Tenant A
		st := changes.StatusPending
		_, _, _ = repo.ListRequests(ctxTenantA, &st, 10, 0)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A changes list")

		// GetRequest under Tenant B
		_, _ = repo.GetRequest(ctxTenantB, "cr-001")
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B changes get")
		assertNotHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant B changes get")

		// CreateRequest under Tenant A
		_ = repo.CreateRequest(ctxTenantA, &changes.ChangeRequest{Title: "Patch", Description: "desc", Type: changes.TypeStandard, Status: changes.StatusPending, Requester: "admin", Cluster: "cls-1", Namespace: "default", Resource: "pod-1"})
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A changes create")

		// Windows under Tenant B
		_, _ = repo.ListWindows(ctxTenantB)
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B windows list")
	})

	t.Run("ComplianceRepo Isolation", func(t *testing.T) {
		repo := postgres.NewComplianceRepo(pool)

		_, _ = repo.ListFrameworks(ctxTenantA)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A compliance list")

		_, _ = repo.GetFramework(ctxTenantB, "fw-001")
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B compliance get")

		_ = repo.UpsertFramework(ctxTenantA, &compliance.Framework{Name: "SOC2"})
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A compliance upsert")

		sev := compliance.SeverityHigh
		_, _, _ = repo.ListViolations(ctxTenantB, &sev, 10, 0)
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B violation list")
	})

	t.Run("CapacityRepo Isolation", func(t *testing.T) {
		repo := postgres.NewCapacityRepo(pool)

		_, _ = repo.List(ctxTenantA, "cluster-1")
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A capacity list")

		_ = repo.Record(ctxTenantB, &capacity.Forecast{Cluster: "cluster-2", ResourceType: "cpu"})
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B capacity record")
		assertNotHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant B capacity record")
	})

	t.Run("CostRepo Isolation", func(t *testing.T) {
		repo := postgres.NewCostRepo(pool)

		_, _ = repo.GetClusterCosts(ctxTenantA)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A cluster costs")

		_, _ = repo.GetNamespaceCosts(ctxTenantB)
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B namespace costs")

		_, _ = repo.GetResourceWaste(ctxTenantA)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A resource waste")
	})

	t.Run("DriftRepo Isolation", func(t *testing.T) {
		repo := postgres.NewDriftRepo(pool)

		_, _, _ = repo.List(ctxTenantA, "cluster-1", nil, 10, 0)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A drift list")

		_ = repo.Resolve(ctxTenantB, "drift-001")
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B drift resolve")

		_ = repo.Create(ctxTenantA, &drift.DriftRecord{Cluster: "cluster-1", Resource: "svc-1"})
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A drift create")
	})

	t.Run("NotificationRepo Isolation", func(t *testing.T) {
		repo := postgres.NewNotificationRepo(pool)

		_, _, _ = repo.ListNotifications(ctxTenantA, 10, 0)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A notif list")

		_, _, _ = repo.ListHistory(ctxTenantB, 10, 0)
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B notif history")

		_ = repo.MarkRead(ctxTenantA, "notif-001")
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A notif mark read")

		_ = repo.CreateNotification(ctxTenantB, &notification.Notification{Title: "Alert"})
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B notif create")
	})

	t.Run("ObservabilityRepo Isolation", func(t *testing.T) {
		repo := postgres.NewObservabilityRepo(pool)

		_, _ = repo.ListSLODefinitions(ctxTenantA)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A slo defs")

		_, _ = repo.GetSLODefinition(ctxTenantB, "slo-001")
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B slo get")

		_ = repo.CreateSLODefinition(ctxTenantA, &observability.SLODefinition{Service: "auth"})
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A slo create")

		_, _ = repo.ListSLOSnapshots(ctxTenantB)
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B snapshot list")
	})

	t.Run("PromotionRepo Isolation", func(t *testing.T) {
		repo := postgres.NewPromotionRepo(pool)

		_, _, _ = repo.List(ctxTenantA, "pending", 10, 0)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A promo list")

		_, _ = repo.GetByID(ctxTenantB, "promo-001")
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B promo get")

		_ = repo.Create(ctxTenantA, &promotion.Promotion{Service: "cart", Version: "v1.2"})
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A promo create")

		_ = repo.Approve(ctxTenantB, "promo-001", "admin")
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B promo approve")
	})

	t.Run("ReportingRepo Isolation", func(t *testing.T) {
		repo := postgres.NewReportingRepo(pool)

		_, _, _ = repo.ListReports(ctxTenantA, 10, 0)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A report list")

		_, _ = repo.GetReport(ctxTenantB, "rep-001")
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B report get")

		_ = repo.CreateReport(ctxTenantA, &reporting.Report{Title: "Q3 Audit"})
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A report create")

		_ = repo.DeleteReport(ctxTenantB, "rep-001")
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B report delete")
	})

	t.Run("TaggingRepo Isolation", func(t *testing.T) {
		repo := postgres.NewTaggingRepo(pool)

		_, _ = repo.ListTags(ctxTenantA)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A tags list")

		_ = repo.CreateTag(ctxTenantB, &tagging.Tag{Key: "env", Value: "prod"})
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B tag create")

		_ = repo.DeleteTag(ctxTenantA, "tag-001")
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A tag delete")

		_ = repo.TagResource(ctxTenantB, "res-001", "tag-001")
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B tag resource")

		_, _ = repo.GetResourceTags(ctxTenantA, "res-001")
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A resource tags")
	})

	t.Run("TimelineRepo Isolation", func(t *testing.T) {
		repo := postgres.NewTimelineRepo(pool)

		_, _, _ = repo.List(ctxTenantA, nil, time.Now().Add(-24*time.Hour), 10, 0)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A timeline list")

		_, _ = repo.GetByID(ctxTenantB, "event-001")
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B timeline get")

		_ = repo.Create(ctxTenantA, &timeline.Event{Title: "Deployment started"})
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A timeline create")
	})

	t.Run("TenancyRepo Isolation and Platform Exemption", func(t *testing.T) {
		repo := postgres.NewTenancyRepo(pool)

		// Tenant-scoped projects
		_, _ = repo.GetProjects(ctxTenantA)
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A projects list")

		// Tenant-scoped members
		_, _ = repo.GetMembers(ctxTenantB)
		assertHasArg(t, pool.lastArgs, "tenant-beta-002", "Tenant B members list")

		// Project creation inherits tenant
		_ = repo.CreateProject(ctxTenantA, domaintenancy.Project{ID: "proj-1", Name: "Backend"})
		assertHasArg(t, pool.lastArgs, "tenant-alpha-001", "Tenant A project create")

		// Platform-wide tables are NOT tenant-scoped
		_, _ = repo.GetOrganizations(ctxTenantA)
		if strings.Contains(pool.lastSQL, "tenant_id") {
			t.Fatalf("GetOrganizations should be global, got tenant_id filter: %s", pool.lastSQL)
		}

		_, _ = repo.GetRBAC(ctxTenantB)
		if strings.Contains(pool.lastSQL, "tenant_id") {
			t.Fatalf("GetRBAC should be global, got tenant_id filter: %s", pool.lastSQL)
		}
	})
}

type tenantSimulatedDBTX struct {
	records map[string]string // map[id]tenantID
}

func (db *tenantSimulatedDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("EXEC 1"), nil
}

func (db *tenantSimulatedDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &emptyRows{}, nil
}

func (db *tenantSimulatedDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if len(args) >= 2 {
		targetID, ok1 := args[0].(string)
		tenantID, ok2 := args[len(args)-1].(string)
		if ok1 && ok2 {
			if ownerTenant, exists := db.records[targetID]; exists {
				if ownerTenant == tenantID {
					return &mockSingleValueRow{val: targetID}
				}
			}
		}
	}
	return &emptyRow{}
}

type mockSingleValueRow struct {
	val string
}

func (r *mockSingleValueRow) Scan(dest ...any) error {
	for _, d := range dest {
		if ptr, ok := d.(*string); ok && ptr != nil {
			*ptr = r.val
		}
	}
	return nil
}

func TestCrossTenantLookup_ReturnsErrNoRows(t *testing.T) {
	simDB := &tenantSimulatedDBTX{
		records: map[string]string{
			"alpha-item-001": "tenant-alpha-001",
			"beta-item-002":  "tenant-beta-002",
		},
	}

	ctxTenantA := pkgtenancy.WithTenantID(context.Background(), "tenant-alpha-001")
	ctxTenantA = pkgtenancy.WithUserRole(ctxTenantA, "user")

	ctxTenantB := pkgtenancy.WithTenantID(context.Background(), "tenant-beta-002")
	ctxTenantB = pkgtenancy.WithUserRole(ctxTenantB, "user")

	t.Run("Promotion cross-tenant lookup returns not found", func(t *testing.T) {
		repo := postgres.NewPromotionRepo(simDB)

		// Tenant A accessing Tenant A's record: succeeds
		promoA, err := repo.GetByID(ctxTenantA, "alpha-item-001")
		if err != nil || promoA == nil {
			t.Fatalf("Tenant A accessing own item failed: %v", err)
		}

		// Tenant B accessing Tenant A's record: fails with not found error
		promoCross, err := repo.GetByID(ctxTenantB, "alpha-item-001")
		if err == nil {
			t.Fatalf("expected Tenant B to get error accessing Tenant A's record, got promo: %v", promoCross)
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})

	t.Run("Reporting cross-tenant lookup returns not found", func(t *testing.T) {
		repo := postgres.NewReportingRepo(simDB)

		// Tenant A accessing own report: succeeds
		repA, err := repo.GetReport(ctxTenantA, "alpha-item-001")
		if err != nil || repA == nil {
			t.Fatalf("Tenant A accessing own report failed: %v", err)
		}

		// Tenant B accessing Tenant A's report: fails with not found error
		repCross, err := repo.GetReport(ctxTenantB, "alpha-item-001")
		if err == nil {
			t.Fatalf("expected Tenant B to get error accessing Tenant A's report, got report: %v", repCross)
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})

	t.Run("Timeline cross-tenant lookup returns not found", func(t *testing.T) {
		repo := postgres.NewTimelineRepo(simDB)

		// Tenant A accessing own event: succeeds
		eventA, err := repo.GetByID(ctxTenantA, "alpha-item-001")
		if err != nil || eventA == nil {
			t.Fatalf("Tenant A accessing own event failed: %v", err)
		}

		// Tenant B accessing Tenant A's event: fails with not found error
		eventCross, err := repo.GetByID(ctxTenantB, "alpha-item-001")
		if err == nil {
			t.Fatalf("expected Tenant B to get error accessing Tenant A's event, got: %v", eventCross)
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})

	t.Run("Changes cross-tenant lookup returns not found", func(t *testing.T) {
		repo := postgres.NewChangesRepo(simDB)

		// Tenant A accessing own request: succeeds
		reqA, err := repo.GetRequest(ctxTenantA, "alpha-item-001")
		if err != nil || reqA == nil {
			t.Fatalf("Tenant A accessing own change request failed: %v", err)
		}

		// Tenant B accessing Tenant A's request: fails with not found error
		reqCross, err := repo.GetRequest(ctxTenantB, "alpha-item-001")
		if err == nil {
			t.Fatalf("expected Tenant B to get error accessing Tenant A's change request, got: %v", reqCross)
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})
}
