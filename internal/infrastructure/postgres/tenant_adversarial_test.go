package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

func TestAdversarial_BuildTenantQuery_StringLiterals(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	q1 := "SELECT * FROM incidents WHERE title = 'foo UNION bar'"
	gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q1)

	if strings.Contains(gotQ, "'foo AND tenant_id =") || strings.Contains(gotQ, "UNION bar' WHERE") {
		t.Fatalf("FAIL: String literal containing UNION corrupted query: %s", gotQ)
	}
	if !strings.Contains(gotQ, "WHERE title = 'foo UNION bar' AND") && !strings.Contains(gotQ, "WHERE title = 'foo UNION bar' AND incidents.tenant_id") {
		t.Errorf("Unexpected rewritten query for string literal with UNION:\n got: %s", gotQ)
	}
	if len(gotArgs) != 1 || gotArgs[0] != "org-acme" {
		t.Errorf("Expected args ['org-acme'], got %v", gotArgs)
	}
}

func TestAdversarial_BuildTenantQuery_Comments(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	q := "SELECT * FROM incidents WHERE status = $1 -- UNION SELECT everything"
	gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "OPEN")

	if strings.Contains(gotQ, "-- AND tenant_id =") {
		t.Fatalf("FAIL: Comment containing UNION corrupted query: %s", gotQ)
	}
	if len(gotArgs) != 2 || gotArgs[1] != "org-acme" {
		t.Errorf("Expected 2 args with 'org-acme', got %v", gotArgs)
	}
}

func TestAdversarial_BuildTenantQuery_SubqueriesAndCTEs(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	t.Run("Subquery in FROM clause", func(t *testing.T) {
		q := "SELECT * FROM (SELECT id FROM incidents UNION SELECT id FROM rca_reports) AS sub"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q)

		if strings.Contains(gotQ, ") AS sub WHERE tenant_id") {
			t.Fatalf("FAIL: Subquery alias appended invalid tenant_id: %s", gotQ)
		}
		if len(gotArgs) != 2 {
			t.Errorf("Expected 2 tenant args for 2 subquery UNION branches, got %d (%v)", len(gotArgs), gotArgs)
		}
	})

	t.Run("CTE query", func(t *testing.T) {
		q := "WITH active_incidents AS (SELECT id, title FROM incidents) SELECT id, title FROM active_incidents"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q)

		if strings.Contains(gotQ, "FROM active_incidents WHERE tenant_id") {
			t.Fatalf("FAIL: CTE query appended WHERE tenant_id to CTE alias that lacks tenant_id column: %s", gotQ)
		}
		if !strings.Contains(gotQ, "FROM incidents WHERE") {
			t.Errorf("Expected tenant filter inside CTE body, got: %s", gotQ)
		}
		if len(gotArgs) != 1 || gotArgs[0] != "org-acme" {
			t.Errorf("Expected 1 arg ['org-acme'], got %v", gotArgs)
		}
	})

	t.Run("JOIN query", func(t *testing.T) {
		q := "SELECT i.id, r.id FROM incidents i JOIN rca_reports r ON i.id = r.incident_id WHERE i.status = $1"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "OPEN")

		if strings.Contains(gotQ, "AND tenant_id =") && !strings.Contains(gotQ, "i.tenant_id =") && !strings.Contains(gotQ, "incidents.tenant_id =") {
			t.Fatalf("FAIL: JOIN query appended unqualified 'tenant_id' causing PostgreSQL column ambiguity: %s", gotQ)
		}
		if !strings.Contains(gotQ, "i.tenant_id = $2") || !strings.Contains(gotQ, "r.tenant_id = $3") {
			t.Errorf("Expected both JOINed tables i and r to be filtered, got: %s", gotQ)
		}
		if len(gotArgs) != 3 || gotArgs[1] != "org-acme" || gotArgs[2] != "org-acme" {
			t.Errorf("Expected 3 args (OPEN, org-acme, org-acme), got %v", gotArgs)
		}
	})
}

func TestAdversarial_BuildTenantQuery_FailClosedEmptyContext(t *testing.T) {
	emptyCtx := context.Background()
	q := "SELECT * FROM incidents"
	gotQ, gotArgs := postgres.BuildTenantQuery(emptyCtx, q)

	if gotQ == q && len(gotArgs) == 0 {
		t.Fatalf("FAIL: Empty tenant context failed open, returning raw query: %s", gotQ)
	}
	if !strings.Contains(gotQ, "tenant_id =") {
		t.Errorf("Expected tenant filtering or fail-closed condition for empty context, got %s", gotQ)
	}
}

func TestAdversarial_BuildTenantQuery_DollarQuotes(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	t.Run("Standard dollar quote $$", func(t *testing.T) {
		q := "SELECT * FROM incidents WHERE body = $$SELECT * FROM users WHERE '1'='1$$"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q)

		if strings.Contains(gotQ, "'1'='1 AND tenant_id") || strings.Contains(gotQ, "users AND tenant_id") {
			t.Fatalf("FAIL: Rewriter injected tenant filter inside dollar-quoted string: %s", gotQ)
		}
		if !strings.Contains(gotQ, "WHERE body = $$SELECT * FROM users WHERE '1'='1$$ AND") {
			t.Errorf("Expected rewritten query to preserve dollar quote intact, got: %s", gotQ)
		}
		if len(gotArgs) != 1 || gotArgs[0] != "org-acme" {
			t.Errorf("Expected 1 arg ['org-acme'], got %v", gotArgs)
		}
	})

	t.Run("Tagged dollar quote $sql$", func(t *testing.T) {
		q := "SELECT * FROM incidents WHERE body = $sql$UNION SELECT * FROM users$sql$"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q)

		if !strings.Contains(gotQ, "WHERE body = $sql$UNION SELECT * FROM users$sql$ AND") {
			t.Errorf("Expected tagged dollar quote intact, got: %s", gotQ)
		}
		if len(gotArgs) != 1 || gotArgs[0] != "org-acme" {
			t.Errorf("Expected 1 arg ['org-acme'], got %v", gotArgs)
		}
	})
}

func TestAdversarial_BuildTenantQuery_MultiLineComments(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	q := "SELECT * FROM incidents /* \n UNION SELECT * FROM users \n ORDER BY id \n */ WHERE status = $1"
	gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "OPEN")

	if !strings.Contains(gotQ, "WHERE status = $1 AND") {
		t.Errorf("Expected filter after status = $1, got: %s", gotQ)
	}
	if len(gotArgs) != 2 || gotArgs[1] != "org-acme" {
		t.Errorf("Expected 2 args, got %v", gotArgs)
	}
}

func TestAdversarial_BuildTenantQuery_NestedCTEs(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	q := "WITH cte1 AS (SELECT id FROM incidents WHERE severity = $1), cte2 AS (SELECT id, incident_id FROM rca_reports WHERE incident_id IN (SELECT id FROM cte1)) SELECT * FROM cte2"
	gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "HIGH")

	if !strings.Contains(gotQ, "incidents WHERE severity = $1 AND incidents.tenant_id = $2") && !strings.Contains(gotQ, "incidents WHERE severity = $1 AND tenant_id = $2") {
		t.Errorf("Expected tenant filter inside cte1, got: %s", gotQ)
	}
	if !strings.Contains(gotQ, "rca_reports WHERE incident_id IN (SELECT id FROM cte1) AND rca_reports.tenant_id = $3") && !strings.Contains(gotQ, "rca_reports WHERE incident_id IN (SELECT id FROM cte1) AND tenant_id = $3") {
		t.Errorf("Expected tenant filter inside cte2, got: %s", gotQ)
	}
	if strings.Contains(gotQ, "FROM cte2 WHERE") {
		t.Errorf("Should not append WHERE tenant_id to CTE alias cte2, got: %s", gotQ)
	}
	if len(gotArgs) != 3 || gotArgs[1] != "org-acme" || gotArgs[2] != "org-acme" {
		t.Errorf("Expected 3 args (HIGH, org-acme, org-acme), got %v", gotArgs)
	}
}

func TestAdversarial_BuildTenantQuery_ComplexJOINs(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	t.Run("LEFT JOIN with alias", func(t *testing.T) {
		q := "SELECT i.id, r.id FROM incidents AS i LEFT JOIN rca_reports AS r ON i.id = r.incident_id WHERE i.created_at > $1"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "2026-01-01")

		if !strings.Contains(gotQ, "i.tenant_id = $2") || !strings.Contains(gotQ, "r.tenant_id = $3") {
			t.Errorf("Expected both LEFT JOINed tables i and r to be filtered, got: %s", gotQ)
		}
		if len(gotArgs) != 3 || gotArgs[1] != "org-acme" || gotArgs[2] != "org-acme" {
			t.Errorf("Expected 3 args, got %v", gotArgs)
		}
	})

	t.Run("FULL OUTER JOIN without WHERE", func(t *testing.T) {
		q := "SELECT * FROM incidents inc FULL OUTER JOIN rca_reports rca ON inc.id = rca.incident_id"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q)

		if !strings.Contains(gotQ, "WHERE inc.tenant_id = $1 AND rca.tenant_id = $2") {
			t.Errorf("Expected WHERE inc.tenant_id = $1 AND rca.tenant_id = $2, got: %s", gotQ)
		}
		if len(gotArgs) != 2 || gotArgs[0] != "org-acme" || gotArgs[1] != "org-acme" {
			t.Errorf("Expected 2 args, got %v", gotArgs)
		}
	})
}

func TestAdversarial_BuildTenantQuery_PlaceholderReindexing(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	q := "SELECT * FROM incidents WHERE status = $1 AND severity = $2 ORDER BY created_at LIMIT $3 OFFSET $4"
	gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "OPEN", "CRITICAL", 10, 20)

	if !strings.Contains(gotQ, "AND incidents.tenant_id = $5 ORDER BY") && !strings.Contains(gotQ, "AND tenant_id = $5 ORDER BY") {
		t.Errorf("Expected tenant_id placeholder $5 inserted before ORDER BY, got: %s", gotQ)
	}
	if len(gotArgs) != 5 || gotArgs[4] != "org-acme" {
		t.Errorf("Expected 5 args with org-acme at end, got len=%d: %v", len(gotArgs), gotArgs)
	}
}

func TestAdversarial_BuildTenantQuery_SubqueryInSelectAndWhere(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	q := "SELECT id, (SELECT count(*) FROM rca_reports WHERE incident_id = incidents.id) FROM incidents WHERE status = $1"
	gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "OPEN")

	if !strings.Contains(gotQ, "rca_reports WHERE incident_id = incidents.id AND rca_reports.tenant_id = $2") && !strings.Contains(gotQ, "rca_reports WHERE incident_id = incidents.id AND tenant_id = $2") {
		t.Errorf("Expected subquery in SELECT to be rewritten with tenant filter, got: %s", gotQ)
	}
	if !strings.Contains(gotQ, "incidents WHERE status = $1 AND incidents.tenant_id = $3") && !strings.Contains(gotQ, "incidents WHERE status = $1 AND tenant_id = $3") {
		t.Errorf("Expected outer query to be rewritten with tenant filter $3, got: %s", gotQ)
	}
	if len(gotArgs) != 3 || gotArgs[1] != "org-acme" || gotArgs[2] != "org-acme" {
		t.Errorf("Expected 3 args (OPEN, org-acme, org-acme), got %v", gotArgs)
	}
}

func TestAdversarial_BuildTenantQuery_NonTenantTablesAndCaseSensitivity(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	t.Run("Non-tenant table ignored", func(t *testing.T) {
		q := "SELECT * FROM users WHERE email = $1"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "user@example.com")

		if strings.Contains(gotQ, "tenant_id") {
			t.Errorf("Expected non-tenant table users to NOT be filtered, got: %s", gotQ)
		}
		if len(gotArgs) != 1 || gotArgs[0] != "user@example.com" {
			t.Errorf("Expected unchanged args, got %v", gotArgs)
		}
	})

	t.Run("Case insensitive keyword and table name matching", func(t *testing.T) {
		q := "select ID, TITLE from INCIDENTS where STATUS = $1 order by CREATED_AT desc"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "OPEN")

		if !strings.Contains(gotQ, "AND INCIDENTS.tenant_id = $2 order by") && !strings.Contains(gotQ, "AND tenant_id = $2 order by") {
			t.Errorf("Expected tenant filter for uppercase INCIDENTS table, got: %s", gotQ)
		}
		if len(gotArgs) != 2 || gotArgs[1] != "org-acme" {
			t.Errorf("Expected 2 args, got %v", gotArgs)
		}
	})
}

func TestAdversarial_BuildTenantQuery_TripleUNION(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	q := "SELECT id FROM incidents UNION ALL SELECT id FROM rca_reports UNION ALL SELECT id FROM gitops_prs"
	gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q)

	if len(gotArgs) != 3 {
		t.Fatalf("Expected 3 args for 3 UNION branches, got %d (%v)", len(gotArgs), gotArgs)
	}
	if !strings.Contains(gotQ, "incidents WHERE tenant_id = $1") && !strings.Contains(gotQ, "incidents WHERE incidents.tenant_id = $1") {
		t.Errorf("Branch 1 missing tenant filter: %s", gotQ)
	}
	if !strings.Contains(gotQ, "rca_reports WHERE tenant_id = $2") && !strings.Contains(gotQ, "rca_reports WHERE rca_reports.tenant_id = $2") {
		t.Errorf("Branch 2 missing tenant filter: %s", gotQ)
	}
	if !strings.Contains(gotQ, "gitops_prs WHERE tenant_id = $3") && !strings.Contains(gotQ, "gitops_prs WHERE gitops_prs.tenant_id = $3") {
		t.Errorf("Branch 3 missing tenant filter: %s", gotQ)
	}
}

func TestAdversarial_BuildTenantQuery_WhereExistsSubquery(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	q := "SELECT * FROM incidents i WHERE EXISTS (SELECT 1 FROM rca_reports r WHERE r.incident_id = i.id AND r.status = $1)"
	gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "RESOLVED")

	if !strings.Contains(gotQ, "r.status = $1 AND r.tenant_id = $2") && !strings.Contains(gotQ, "r.status = $1 AND tenant_id = $2") {
		t.Errorf("Expected subquery inside EXISTS to be filtered with $2, got: %s", gotQ)
	}
	if !strings.Contains(gotQ, "i.tenant_id = $3") {
		t.Errorf("Expected outer query incidents alias i to be filtered with $3, got: %s", gotQ)
	}
	if len(gotArgs) != 3 || gotArgs[1] != "org-acme" || gotArgs[2] != "org-acme" {
		t.Errorf("Expected 3 args (RESOLVED, org-acme, org-acme), got %v", gotArgs)
	}
}

func TestAdversarial_BuildTenantQuery_UPDATE(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	t.Run("UPDATE without WHERE", func(t *testing.T) {
		q := "UPDATE incidents SET status = $1"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "RESOLVED")

		if !strings.Contains(gotQ, "WHERE tenant_id = $2") {
			t.Errorf("Expected UPDATE without WHERE to append WHERE tenant_id = $2, got: %s", gotQ)
		}
		if len(gotArgs) != 2 || gotArgs[1] != "org-acme" {
			t.Errorf("Expected 2 args with org-acme, got %v", gotArgs)
		}
	})

	t.Run("UPDATE with WHERE", func(t *testing.T) {
		q := "UPDATE incidents SET status = $1 WHERE id = $2"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "CLOSED", "inc-100")

		if !strings.Contains(gotQ, "WHERE id = $2 AND tenant_id = $3") {
			t.Errorf("Expected UPDATE with WHERE to append AND tenant_id = $3, got: %s", gotQ)
		}
		if len(gotArgs) != 3 || gotArgs[2] != "org-acme" {
			t.Errorf("Expected 3 args with org-acme at end, got %v", gotArgs)
		}
	})

	t.Run("UPDATE with alias and RETURNING", func(t *testing.T) {
		q := "UPDATE incidents AS i SET status = $1 WHERE i.id = $2 RETURNING i.id"
		gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "CLOSED", "inc-100")

		if !strings.Contains(gotQ, "i.tenant_id = $3 RETURNING") {
			t.Errorf("Expected filter before RETURNING, got: %s", gotQ)
		}
		if len(gotArgs) != 3 || gotArgs[2] != "org-acme" {
			t.Errorf("Expected 3 args, got %v", gotArgs)
		}
	})
}

func TestAdversarial_BuildTenantQuery_NonTenantPrimaryWithTenantJOIN(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-acme")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	q := "SELECT u.email, i.title FROM users u JOIN incidents i ON u.id = i.created_by WHERE u.status = $1"
	gotQ, gotArgs := postgres.BuildTenantQuery(ctx, q, "ACTIVE")

	if !strings.Contains(gotQ, "i.tenant_id = $2") {
		t.Errorf("Expected tenant filter for JOINed incidents table i.tenant_id = $2, got: %s", gotQ)
	}
	if strings.Contains(gotQ, "u.tenant_id") {
		t.Errorf("Should NOT append tenant_id for non-tenant users table u, got: %s", gotQ)
	}
	if len(gotArgs) != 2 || gotArgs[1] != "org-acme" {
		t.Errorf("Expected 2 args, got %v", gotArgs)
	}
}



