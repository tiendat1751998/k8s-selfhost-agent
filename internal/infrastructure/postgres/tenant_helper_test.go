package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/datdt/k8sselfhost/internal/domain/promotion"
	"github.com/datdt/k8sselfhost/internal/domain/timeline"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestEnsureTenantContext(t *testing.T) {
	t.Run("empty context gets default tenant", func(t *testing.T) {
		ctx := context.Background()
		enriched := postgres.EnsureTenantContext(ctx)
		tid := tenancy.TenantIDFromContext(enriched)
		if tid != postgres.DefaultTenantID {
			t.Fatalf("expected tenant %q, got %q", postgres.DefaultTenantID, tid)
		}
	})

	t.Run("context with empty tenant gets default tenant", func(t *testing.T) {
		ctx := tenancy.WithTenantID(context.Background(), "")
		enriched := postgres.EnsureTenantContext(ctx)
		tid := tenancy.TenantIDFromContext(enriched)
		if tid != postgres.DefaultTenantID {
			t.Fatalf("expected tenant %q, got %q", postgres.DefaultTenantID, tid)
		}
	})

	t.Run("context with custom tenant retains custom tenant", func(t *testing.T) {
		const custom = "acme-corp"
		ctx := tenancy.WithTenantID(context.Background(), custom)
		enriched := postgres.EnsureTenantContext(ctx)
		tid := tenancy.TenantIDFromContext(enriched)
		if tid != custom {
			t.Fatalf("expected tenant %q, got %q", custom, tid)
		}
	})
}

func TestTenantIDOrDefault(t *testing.T) {
	t.Run("empty context returns default tenant", func(t *testing.T) {
		tid := postgres.TenantIDOrDefault(context.Background())
		if tid != postgres.DefaultTenantID {
			t.Fatalf("expected tenant %q, got %q", postgres.DefaultTenantID, tid)
		}
	})

	t.Run("context with custom tenant returns custom tenant", func(t *testing.T) {
		const custom = "custom-slug-123"
		ctx := tenancy.WithTenantID(context.Background(), custom)
		tid := postgres.TenantIDOrDefault(ctx)
		if tid != custom {
			t.Fatalf("expected tenant %q, got %q", custom, tid)
		}
	})
}

func TestDeterministicTenantUUID(t *testing.T) {
	t.Run("empty and default slug return default uuid", func(t *testing.T) {
		if got := postgres.DeterministicTenantUUID(""); got != postgres.DefaultTenantUUID {
			t.Fatalf("expected %q, got %q", postgres.DefaultTenantUUID, got)
		}
		if got := postgres.DeterministicTenantUUID(postgres.DefaultTenantID); got != postgres.DefaultTenantUUID {
			t.Fatalf("expected %q, got %q", postgres.DefaultTenantUUID, got)
		}
	})

	t.Run("valid uuid is returned unchanged", func(t *testing.T) {
		raw := "12345678-1234-1234-1234-123456789abc"
		if got := postgres.DeterministicTenantUUID(raw); got != raw {
			t.Fatalf("expected %q, got %q", raw, got)
		}
	})

	t.Run("arbitrary slug produces valid deterministic uuid", func(t *testing.T) {
		slug := "team-alpha"
		u1 := postgres.DeterministicTenantUUID(slug)
		u2 := postgres.DeterministicTenantUUID(slug)
		if u1 != u2 {
			t.Fatalf("expected deterministic UUID, got %q and %q", u1, u2)
		}
		if _, err := uuid.Parse(u1); err != nil {
			t.Fatalf("expected valid UUID, got error: %v", err)
		}
	})
}

func TestDeterministicSlugForUUID(t *testing.T) {
	t.Run("empty and default uuids return default tenant slug", func(t *testing.T) {
		if got := postgres.DeterministicSlugForUUID(""); got != postgres.DefaultTenantID {
			t.Fatalf("expected %q, got %q", postgres.DefaultTenantID, got)
		}
		if got := postgres.DeterministicSlugForUUID(postgres.DefaultTenantUUID); got != postgres.DefaultTenantID {
			t.Fatalf("expected %q, got %q", postgres.DefaultTenantID, got)
		}
		if got := postgres.DeterministicSlugForUUID("00000000-0000-0000-0000-000000000000"); got != postgres.DefaultTenantID {
			t.Fatalf("expected %q, got %q", postgres.DefaultTenantID, got)
		}
	})

	t.Run("custom slug or uuid is returned unchanged", func(t *testing.T) {
		const custom = "custom-org-slug"
		if got := postgres.DeterministicSlugForUUID(custom); got != custom {
			t.Fatalf("expected %q, got %q", custom, got)
		}
	})
}

func TestTenantIDSlugIsolation_AcrossMigratedTables(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	const (
		tenantA = "default-tenant"
		tenantB = "custom-tenant-uuid-slug"
	)

	ctxA := tenancy.WithTenantID(context.Background(), tenantA)
	ctxB := tenancy.WithTenantID(context.Background(), tenantB)

	nonce := time.Now().UnixNano()

	// 1. promotions table via PromotionRepo
	t.Run("promotions table isolation with string slugs", func(t *testing.T) {
		repo := postgres.NewPromotionRepo(pool)
		svcNameA := fmt.Sprintf("promo-slug-a-%d", nonce)
		svcNameB := fmt.Sprintf("promo-slug-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM promotions WHERE service IN ($1, $2)", svcNameA, svcNameB)

		pA, err := promotion.NewPromotion(svcNameA, "v1.0.0", promotion.EnvDev, promotion.EnvQA, "test@k8sselfhost.local")
		if err != nil {
			t.Fatalf("init pA: %v", err)
		}
		pB, err := promotion.NewPromotion(svcNameB, "v1.0.0", promotion.EnvDev, promotion.EnvQA, "test@k8sselfhost.local")
		if err != nil {
			t.Fatalf("init pB: %v", err)
		}

		if err := repo.Create(ctxA, pA); err != nil {
			t.Fatalf("repo.Create(ctxA) failed (possible 22P02?): %v", err)
		}
		if err := repo.Create(ctxB, pB); err != nil {
			t.Fatalf("repo.Create(ctxB) failed (possible 22P02?): %v", err)
		}

		// Tenant A must see pA, but NOT pB
		gotA, err := repo.GetByID(ctxA, pA.ID)
		if err != nil || gotA.Service != svcNameA {
			t.Fatalf("tenantA failed to fetch its own promotion: %v", err)
		}
		if _, err := repo.GetByID(ctxA, pB.ID); err == nil {
			t.Fatalf("tenantA should NOT have access to tenantB's promotion")
		}

		// Tenant B must see pB, but NOT pA
		gotB, err := repo.GetByID(ctxB, pB.ID)
		if err != nil || gotB.Service != svcNameB {
			t.Fatalf("tenantB failed to fetch its own promotion: %v", err)
		}
		if _, err := repo.GetByID(ctxB, pA.ID); err == nil {
			t.Fatalf("tenantB should NOT have access to tenantA's promotion")
		}
	})

	// 2. timeline_events table via TimelineRepo
	t.Run("timeline_events table isolation with string slugs", func(t *testing.T) {
		repo := postgres.NewTimelineRepo(pool)
		titleA := fmt.Sprintf("event-a-%d", nonce)
		titleB := fmt.Sprintf("event-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM timeline_events WHERE title IN ($1, $2)", titleA, titleB)

		eA := &timeline.Event{
			Type:      timeline.EventDeployment,
			Title:     titleA,
			Detail:    "test detail A",
			CreatedAt: time.Now(),
		}
		eB := &timeline.Event{
			Type:      timeline.EventDeployment,
			Title:     titleB,
			Detail:    "test detail B",
			CreatedAt: time.Now(),
		}

		if err := repo.Create(ctxA, eA); err != nil {
			t.Fatalf("create eA failed: %v", err)
		}
		if err := repo.Create(ctxB, eB); err != nil {
			t.Fatalf("create eB failed: %v", err)
		}

		gotA, err := repo.GetByID(ctxA, eA.ID)
		if err != nil || gotA.Title != titleA {
			t.Fatalf("tenantA could not find eA: %v", err)
		}
		if _, err := repo.GetByID(ctxA, eB.ID); err == nil {
			t.Fatalf("tenantA should NOT find eB")
		}
	})

	// 3. change_requests table
	t.Run("change_requests table isolation with string slugs", func(t *testing.T) {
		titleA := fmt.Sprintf("cr-a-%d", nonce)
		titleB := fmt.Sprintf("cr-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM change_requests WHERE title IN ($1, $2)", titleA, titleB)

		insertSQL := `INSERT INTO change_requests (title, description, type, status, requester, cluster, namespace, resource, scheduled_at, created_at, updated_at, tenant_id)
			VALUES ($1, 'desc', 'config', 'pending', 'user1', 'cl-1', 'ns-1', 'res-1', NOW(), NOW(), NOW(), $2)`
		if _, err := pool.Exec(ctxA, insertSQL, titleA, tenantA); err != nil {
			t.Fatalf("inserting crA failed: %v", err)
		}
		if _, err := pool.Exec(ctxB, insertSQL, titleB, tenantB); err != nil {
			t.Fatalf("inserting crB failed: %v", err)
		}

		qA, argsA := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM change_requests WHERE title = $1", titleA)
		var countA int
		if err := pool.QueryRow(ctxA, qA, argsA...).Scan(&countA); err != nil || countA != 1 {
			t.Fatalf("tenantA expected 1 crA, got %d (err: %v)", countA, err)
		}

		qA_cross, argsA_cross := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM change_requests WHERE title = $1", titleB)
		var countCross int
		if err := pool.QueryRow(ctxA, qA_cross, argsA_cross...).Scan(&countCross); err != nil || countCross != 0 {
			t.Fatalf("tenantA cross-tenant leak: expected 0 crB, got %d (err: %v)", countCross, err)
		}
	})

	// 4. maintenance_windows table
	t.Run("maintenance_windows table isolation with string slugs", func(t *testing.T) {
		titleA := fmt.Sprintf("mw-a-%d", nonce)
		titleB := fmt.Sprintf("mw-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM maintenance_windows WHERE title IN ($1, $2)", titleA, titleB)

		insertSQL := `INSERT INTO maintenance_windows (title, cluster, start_at, end_at, tenant_id)
			VALUES ($1, 'cluster-1', NOW(), NOW() + interval '1 hour', $2)`
		if _, err := pool.Exec(ctxA, insertSQL, titleA, tenantA); err != nil {
			t.Fatalf("inserting mwA failed: %v", err)
		}
		if _, err := pool.Exec(ctxB, insertSQL, titleB, tenantB); err != nil {
			t.Fatalf("inserting mwB failed: %v", err)
		}

		qA, argsA := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM maintenance_windows WHERE title = $1", titleA)
		var count int
		if err := pool.QueryRow(ctxA, qA, argsA...).Scan(&count); err != nil || count != 1 {
			t.Fatalf("tenantA expected 1 mwA, got %d (err: %v)", count, err)
		}

		qCross, argsCross := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM maintenance_windows WHERE title = $1", titleB)
		if err := pool.QueryRow(ctxA, qCross, argsCross...).Scan(&count); err != nil || count != 0 {
			t.Fatalf("tenantA leaked mwB: got %d (err: %v)", count, err)
		}
	})

	// 5. compliance_frameworks table
	t.Run("compliance_frameworks table isolation with string slugs", func(t *testing.T) {
		nameA := fmt.Sprintf("fw-a-%d", nonce)
		nameB := fmt.Sprintf("fw-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM compliance_frameworks WHERE name IN ($1, $2)", nameA, nameB)

		insertSQL := `INSERT INTO compliance_frameworks (name, icon, tenant_id) VALUES ($1, '🛡️', $2)`
		if _, err := pool.Exec(ctxA, insertSQL, nameA, tenantA); err != nil {
			t.Fatalf("inserting fwA failed: %v", err)
		}
		if _, err := pool.Exec(ctxB, insertSQL, nameB, tenantB); err != nil {
			t.Fatalf("inserting fwB failed: %v", err)
		}

		qA, argsA := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM compliance_frameworks WHERE name = $1", nameA)
		var count int
		if err := pool.QueryRow(ctxA, qA, argsA...).Scan(&count); err != nil || count != 1 {
			t.Fatalf("tenantA expected 1 fwA, got %d (err: %v)", count, err)
		}

		qCross, argsCross := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM compliance_frameworks WHERE name = $1", nameB)
		if err := pool.QueryRow(ctxA, qCross, argsCross...).Scan(&count); err != nil || count != 0 {
			t.Fatalf("tenantA leaked fwB: got %d (err: %v)", count, err)
		}
	})

	// 6. compliance_violations table
	t.Run("compliance_violations table isolation with string slugs", func(t *testing.T) {
		policyA := fmt.Sprintf("pol-a-%d", nonce)
		policyB := fmt.Sprintf("pol-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM compliance_violations WHERE policy IN ($1, $2)", policyA, policyB)

		insertSQL := `INSERT INTO compliance_violations (severity, policy, resource, tenant_id) VALUES ('high', $1, 'pod-1', $2)`
		if _, err := pool.Exec(ctxA, insertSQL, policyA, tenantA); err != nil {
			t.Fatalf("inserting vioA failed: %v", err)
		}
		if _, err := pool.Exec(ctxB, insertSQL, policyB, tenantB); err != nil {
			t.Fatalf("inserting vioB failed: %v", err)
		}

		qA, argsA := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM compliance_violations WHERE policy = $1", policyA)
		var count int
		if err := pool.QueryRow(ctxA, qA, argsA...).Scan(&count); err != nil || count != 1 {
			t.Fatalf("tenantA expected 1 vioA, got %d (err: %v)", count, err)
		}

		qCross, argsCross := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM compliance_violations WHERE policy = $1", policyB)
		if err := pool.QueryRow(ctxA, qCross, argsCross...).Scan(&count); err != nil || count != 0 {
			t.Fatalf("tenantA leaked vioB: got %d (err: %v)", count, err)
		}
	})

	// 7. capacity_forecasts table
	t.Run("capacity_forecasts table isolation with string slugs", func(t *testing.T) {
		clA := fmt.Sprintf("cl-a-%d", nonce)
		clB := fmt.Sprintf("cl-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM capacity_forecasts WHERE cluster IN ($1, $2)", clA, clB)

		insertSQL := `INSERT INTO capacity_forecasts (cluster, resource_type, tenant_id) VALUES ($1, 'cpu', $2)`
		if _, err := pool.Exec(ctxA, insertSQL, clA, tenantA); err != nil {
			t.Fatalf("inserting capA failed: %v", err)
		}
		if _, err := pool.Exec(ctxB, insertSQL, clB, tenantB); err != nil {
			t.Fatalf("inserting capB failed: %v", err)
		}

		qA, argsA := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM capacity_forecasts WHERE cluster = $1", clA)
		var count int
		if err := pool.QueryRow(ctxA, qA, argsA...).Scan(&count); err != nil || count != 1 {
			t.Fatalf("tenantA expected 1 capA, got %d (err: %v)", count, err)
		}

		qCross, argsCross := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM capacity_forecasts WHERE cluster = $1", clB)
		if err := pool.QueryRow(ctxA, qCross, argsCross...).Scan(&count); err != nil || count != 0 {
			t.Fatalf("tenantA leaked capB: got %d (err: %v)", count, err)
		}
	})

	// 8. drift_records table
	t.Run("drift_records table isolation with string slugs", func(t *testing.T) {
		resA := fmt.Sprintf("res-a-%d", nonce)
		resB := fmt.Sprintf("res-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM drift_records WHERE resource IN ($1, $2)", resA, resB)

		insertSQL := `INSERT INTO drift_records (cluster, namespace, resource, resource_kind, tenant_id) VALUES ('cl-1', 'ns-1', $1, 'Deployment', $2)`
		if _, err := pool.Exec(ctxA, insertSQL, resA, tenantA); err != nil {
			t.Fatalf("inserting driftA failed: %v", err)
		}
		if _, err := pool.Exec(ctxB, insertSQL, resB, tenantB); err != nil {
			t.Fatalf("inserting driftB failed: %v", err)
		}

		qA, argsA := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM drift_records WHERE resource = $1", resA)
		var count int
		if err := pool.QueryRow(ctxA, qA, argsA...).Scan(&count); err != nil || count != 1 {
			t.Fatalf("tenantA expected 1 driftA, got %d (err: %v)", count, err)
		}

		qCross, argsCross := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM drift_records WHERE resource = $1", resB)
		if err := pool.QueryRow(ctxA, qCross, argsCross...).Scan(&count); err != nil || count != 0 {
			t.Fatalf("tenantA leaked driftB: got %d (err: %v)", count, err)
		}
	})

	// 9. tags table
	t.Run("tags table isolation with string slugs", func(t *testing.T) {
		keyA := fmt.Sprintf("key-a-%d", nonce)
		keyB := fmt.Sprintf("key-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM tags WHERE key IN ($1, $2)", keyA, keyB)

		insertSQL := `INSERT INTO tags (key, value, tenant_id) VALUES ($1, 'val-1', $2)`
		if _, err := pool.Exec(ctxA, insertSQL, keyA, tenantA); err != nil {
			t.Fatalf("inserting tagA failed: %v", err)
		}
		if _, err := pool.Exec(ctxB, insertSQL, keyB, tenantB); err != nil {
			t.Fatalf("inserting tagB failed: %v", err)
		}

		qA, argsA := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM tags WHERE key = $1", keyA)
		var count int
		if err := pool.QueryRow(ctxA, qA, argsA...).Scan(&count); err != nil || count != 1 {
			t.Fatalf("tenantA expected 1 tagA, got %d (err: %v)", count, err)
		}

		qCross, argsCross := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM tags WHERE key = $1", keyB)
		if err := pool.QueryRow(ctxA, qCross, argsCross...).Scan(&count); err != nil || count != 0 {
			t.Fatalf("tenantA leaked tagB: got %d (err: %v)", count, err)
		}
	})

	// 10. reports table
	t.Run("reports table isolation with string slugs", func(t *testing.T) {
		titleA := fmt.Sprintf("report-a-%d", nonce)
		titleB := fmt.Sprintf("report-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM reports WHERE title IN ($1, $2)", titleA, titleB)

		insertSQL := `INSERT INTO reports (title, type, format, created_by, expires_at, tenant_id)
			VALUES ($1, 'operational', 'pdf', 'user1', NOW() + interval '7 days', $2)`
		if _, err := pool.Exec(ctxA, insertSQL, titleA, tenantA); err != nil {
			t.Fatalf("inserting reportA failed: %v", err)
		}
		if _, err := pool.Exec(ctxB, insertSQL, titleB, tenantB); err != nil {
			t.Fatalf("inserting reportB failed: %v", err)
		}

		qA, argsA := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM reports WHERE title = $1", titleA)
		var count int
		if err := pool.QueryRow(ctxA, qA, argsA...).Scan(&count); err != nil || count != 1 {
			t.Fatalf("tenantA expected 1 reportA, got %d (err: %v)", count, err)
		}

		qCross, argsCross := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM reports WHERE title = $1", titleB)
		if err := pool.QueryRow(ctxA, qCross, argsCross...).Scan(&count); err != nil || count != 0 {
			t.Fatalf("tenantA leaked reportB: got %d (err: %v)", count, err)
		}
	})

	// 11. projects table
	t.Run("projects table isolation with string slugs", func(t *testing.T) {
		projA := fmt.Sprintf("proj-a-%d", nonce)
		projB := fmt.Sprintf("proj-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM projects WHERE id IN ($1, $2)", projA, projB)

		insertSQL := `INSERT INTO projects (id, org_id, name, envs, workloads, tenant_id)
			VALUES ($1, 'default-tenant', 'Proj Test', '["dev"]'::jsonb, 1, $2)`
		if _, err := pool.Exec(ctxA, insertSQL, projA, tenantA); err != nil {
			t.Fatalf("inserting projA failed: %v", err)
		}
		if _, err := pool.Exec(ctxB, insertSQL, projB, tenantB); err != nil {
			t.Fatalf("inserting projB failed: %v", err)
		}

		qA, argsA := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM projects WHERE id = $1", projA)
		var count int
		if err := pool.QueryRow(ctxA, qA, argsA...).Scan(&count); err != nil || count != 1 {
			t.Fatalf("tenantA expected 1 projA, got %d (err: %v)", count, err)
		}

		qCross, argsCross := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM projects WHERE id = $1", projB)
		if err := pool.QueryRow(ctxA, qCross, argsCross...).Scan(&count); err != nil || count != 0 {
			t.Fatalf("tenantA leaked projB: got %d (err: %v)", count, err)
		}
	})

	// 12. tenant_members table
	t.Run("tenant_members table isolation with string slugs", func(t *testing.T) {
		userA := fmt.Sprintf("user-a-%d", nonce)
		userB := fmt.Sprintf("user-b-%d", nonce)
		defer pool.Exec(context.Background(), "DELETE FROM tenant_members WHERE username IN ($1, $2)", userA, userB)

		insertSQL := `INSERT INTO tenant_members (org_id, username, role, scope, tenant_id)
			VALUES ('default-tenant', $1, 'admin', 'global', $2)`
		if _, err := pool.Exec(ctxA, insertSQL, userA, tenantA); err != nil {
			t.Fatalf("inserting memberA failed: %v", err)
		}
		if _, err := pool.Exec(ctxB, insertSQL, userB, tenantB); err != nil {
			t.Fatalf("inserting memberB failed: %v", err)
		}

		qA, argsA := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM tenant_members WHERE username = $1", userA)
		var count int
		if err := pool.QueryRow(ctxA, qA, argsA...).Scan(&count); err != nil || count != 1 {
			t.Fatalf("tenantA expected 1 memberA, got %d (err: %v)", count, err)
		}

		qCross, argsCross := postgres.BuildTenantQuery(ctxA, "SELECT count(*) FROM tenant_members WHERE username = $1", userB)
		if err := pool.QueryRow(ctxA, qCross, argsCross...).Scan(&count); err != nil || count != 0 {
			t.Fatalf("tenantA leaked memberB: got %d (err: %v)", count, err)
		}
	})
}
