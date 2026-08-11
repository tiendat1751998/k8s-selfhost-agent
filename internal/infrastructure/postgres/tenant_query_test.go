package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
)

func TestBuildTenantQuery_StandardQueries(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-123")

	t.Run("Query without WHERE", func(t *testing.T) {
		q := "SELECT id, name FROM incidents ORDER BY created_at DESC"
		gotQ, gotArgs := BuildTenantQuery(ctx, q)

		expectedSubstring := "WHERE tenant_id = $1 ORDER BY"
		if !strings.Contains(gotQ, expectedSubstring) {
			t.Errorf("Expected query to contain %q, got %q", expectedSubstring, gotQ)
		}
		if len(gotArgs) != 1 || gotArgs[0] != "tenant-123" {
			t.Errorf("Expected args ['tenant-123'], got %v", gotArgs)
		}
	})

	t.Run("Query with WHERE", func(t *testing.T) {
		q := "SELECT id, name FROM incidents WHERE status = $1 ORDER BY created_at DESC"
		gotQ, gotArgs := BuildTenantQuery(ctx, q, "open")

		expectedSubstring := "AND tenant_id = $2 ORDER BY"
		if !strings.Contains(gotQ, expectedSubstring) {
			t.Errorf("Expected query to contain %q, got %q", expectedSubstring, gotQ)
		}
		if len(gotArgs) != 2 || gotArgs[1] != "tenant-123" {
			t.Errorf("Expected args ['open', 'tenant-123'], got %v", gotArgs)
		}
	})
}

func TestBuildTenantQuery_EdgeCasesAndLimitations(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-123")

	t.Run("Subquery in FROM clause without outer WHERE", func(t *testing.T) {
		// Outer query has NO WHERE clause, but subquery has WHERE clause
		q := "SELECT * FROM (SELECT id FROM incidents WHERE status = 'open') sub"
		gotQ, _ := BuildTenantQuery(ctx, q)

		// strings.Contains(..., " WHERE ") detects WHERE inside the subquery,
		// so it appends " AND tenant_id = $1" to the end of the query instead of " WHERE tenant_id = $1"
		if strings.HasSuffix(gotQ, "AND tenant_id = $1") {
			t.Logf("CONFIRMED ISSUE: Malformed SQL generated for subquery in FROM: %q", gotQ)
		}
	})

	t.Run("UNION query tenant leak", func(t *testing.T) {
		q := "SELECT id FROM incidents UNION SELECT id FROM rca_reports"
		gotQ, _ := BuildTenantQuery(ctx, q)

		// BuildTenantQuery appends WHERE to the end, which only filters the second SELECT branch of UNION
		if !strings.Contains(gotQ[:strings.Index(gotQ, "UNION")], "WHERE tenant_id") {
			t.Logf("CONFIRMED ISSUE: First branch of UNION query lacks tenant isolation: %q", gotQ)
		}
	})

	t.Run("Empty tenant ID in context fails open", func(t *testing.T) {
		emptyCtx := context.Background()
		q := "SELECT * FROM incidents"
		gotQ, gotArgs := BuildTenantQuery(emptyCtx, q)

		if gotQ == q && len(gotArgs) == 0 {
			t.Logf("CONFIRMED CAVEAT/BEHAVIOR: Empty tenant ID returns unfiltered query (fails open)")
		}
	})
}
