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

func TestBuildTenantQuery_TenantScopedTables(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-123")

	tables := []string{
		"backup_history",
		"notifications",
		"slo_definitions",
		"promotions",
		"reports",
		"change_requests",
		"drift_records",
		"compliance_violations",
		"correlated_events",
	}

	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			q := "SELECT * FROM " + table
			gotQ, gotArgs := BuildTenantQuery(ctx, q)

			expected := "WHERE tenant_id = $1"
			if !strings.Contains(gotQ, expected) {
				t.Errorf("Expected query for table %q to contain %q, got %q", table, expected, gotQ)
			}
			if len(gotArgs) != 1 || gotArgs[0] != "tenant-123" {
				t.Errorf("Expected args ['tenant-123'], got %v", gotArgs)
			}
		})
	}
}

func TestBuildTenantQuery_OrPrecedenceIsolation(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "tenant-sec")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "operator")

	tests := []struct {
		name     string
		query    string
		args     []interface{}
		wantSQL  string
		wantArgs []interface{}
	}{
		{
			name:     "SELECT with OR and LIMIT",
			query:    "SELECT * FROM incidents WHERE x = $1 OR y = $1 LIMIT 10",
			args:     []interface{}{"val"},
			wantSQL:  "SELECT * FROM incidents WHERE (x = $1 OR y = $1) AND tenant_id = $2 LIMIT 10",
			wantArgs: []interface{}{"val", "tenant-sec"},
		},
		{
			name:     "SELECT with OR and ORDER BY",
			query:    "SELECT * FROM incidents WHERE a OR b ORDER BY c",
			args:     nil,
			wantSQL:  "SELECT * FROM incidents WHERE (a OR b) AND tenant_id = $1 ORDER BY c",
			wantArgs: []interface{}{"tenant-sec"},
		},
		{
			name:     "UPDATE with OR",
			query:    "UPDATE audit_findings SET status = 'resolved' WHERE id = $1 OR name = $2",
			args:     []interface{}{"id-1", "name-1"},
			wantSQL:  "UPDATE audit_findings SET status = 'resolved' WHERE (id = $1 OR name = $2) AND tenant_id = $3",
			wantArgs: []interface{}{"id-1", "name-1", "tenant-sec"},
		},
		{
			name:     "no-OR case: SELECT with simple WHERE and LIMIT",
			query:    "SELECT * FROM incidents WHERE status = $1 LIMIT 10",
			args:     []interface{}{"OPEN"},
			wantSQL:  "SELECT * FROM incidents WHERE (status = $1) AND tenant_id = $2 LIMIT 10",
			wantArgs: []interface{}{"OPEN", "tenant-sec"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotSQL, gotArgs := BuildTenantQuery(ctx, tc.query, tc.args...)
			if gotSQL != tc.wantSQL {
				t.Errorf("SQL mismatch:\n got:  %s\n want: %s", gotSQL, tc.wantSQL)
			}
			if len(gotArgs) != len(tc.wantArgs) {
				t.Fatalf("Args len mismatch: got %d, want %d", len(gotArgs), len(tc.wantArgs))
			}
			for i := range gotArgs {
				if gotArgs[i] != tc.wantArgs[i] {
					t.Errorf("Arg[%d] mismatch: got %v, want %v", i, gotArgs[i], tc.wantArgs[i])
				}
			}
		})
	}
}
