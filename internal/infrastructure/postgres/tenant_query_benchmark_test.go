package postgres

import (
	"context"
	"testing"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

var benchmarkTenantCtx = context.WithValue(
	context.WithValue(context.Background(), tenancy.TenantIDKey, "tenant-benchmark-uuid-1234"),
	tenancy.UserRoleKey, "member",
)

var benchmarkPlatformAdminCtx = context.WithValue(
	context.WithValue(context.Background(), tenancy.TenantIDKey, "system"),
	tenancy.UserRoleKey, "platform_admin",
)

const (
	simpleQuery = "SELECT id, name, created_at FROM incidents ORDER BY created_at DESC LIMIT 20"
	whereQuery  = "SELECT id, title, severity FROM incidents WHERE status = $1 AND severity = $2 ORDER BY created_at DESC"
	joinQuery   = "SELECT i.id, i.title, c.name AS cluster_name FROM incidents i JOIN clusters c ON i.cluster_id = c.id WHERE i.status = $1 AND c.status = $2 ORDER BY i.created_at DESC LIMIT 50"
	insertQuery = "INSERT INTO incidents (title, severity, cluster_id) VALUES ($1, $2, $3) RETURNING id"
	updateQuery = "UPDATE incidents SET status = $1, resolved_at = $2 WHERE id = $3"
	deleteQuery = "DELETE FROM incidents WHERE id = $1"
	nonTenantQ  = "SELECT id, username, email, role FROM users WHERE is_active = $1"
)

func BenchmarkTokenizeSQL_Simple(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tokens := tokenizeSQL(simpleQuery)
		_ = tokens
	}
}

func BenchmarkTokenizeSQL_Complex(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tokens := tokenizeSQL(joinQuery)
		_ = tokens
	}
}

func BenchmarkBuildTenantQuery_PlatformAdmin(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q, args := BuildTenantQuery(benchmarkPlatformAdminCtx, simpleQuery)
		_ = q
		_ = args
	}
}

func BenchmarkBuildTenantQuery_Simple(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q, args := BuildTenantQuery(benchmarkTenantCtx, simpleQuery)
		_ = q
		_ = args
	}
}

func BenchmarkBuildTenantQuery_WithWhere(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q, args := BuildTenantQuery(benchmarkTenantCtx, whereQuery, "firing", "critical")
		_ = q
		_ = args
	}
}

func BenchmarkBuildTenantQuery_ComplexJoin(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q, args := BuildTenantQuery(benchmarkTenantCtx, joinQuery, "firing", "active")
		_ = q
		_ = args
	}
}

func BenchmarkBuildTenantQuery_Insert(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q, args := BuildTenantQuery(benchmarkTenantCtx, insertQuery, "High CPU", "critical", "clust-1")
		_ = q
		_ = args
	}
}

func BenchmarkBuildTenantQuery_Update(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q, args := BuildTenantQuery(benchmarkTenantCtx, updateQuery, "resolved", "now()", "inc-1")
		_ = q
		_ = args
	}
}

func BenchmarkBuildTenantQuery_Delete(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q, args := BuildTenantQuery(benchmarkTenantCtx, deleteQuery, "inc-1")
		_ = q
		_ = args
	}
}

func BenchmarkBuildTenantQuery_NonTenantTable(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q, args := BuildTenantQuery(benchmarkTenantCtx, nonTenantQ, true)
		_ = q
		_ = args
	}
}

func BenchmarkBuildTenantQuery_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			q, args := BuildTenantQuery(benchmarkTenantCtx, whereQuery, "firing", "critical")
			_ = q
			_ = args
		}
	})
}
