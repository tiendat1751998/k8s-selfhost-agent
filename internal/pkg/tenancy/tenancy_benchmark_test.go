package tenancy

import (
	"context"
	"testing"
)

func BenchmarkContextOperations(b *testing.B) {
	b.Run("InjectTenantID", func(b *testing.B) {
		ctx := context.Background()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = context.WithValue(ctx, TenantIDKey, "tenant-12345678-uuid")
		}
	})

	b.Run("ExtractTenantID", func(b *testing.B) {
		ctx := context.WithValue(context.Background(), TenantIDKey, "tenant-12345678-uuid")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = TenantIDFromContext(ctx)
		}
	})

	b.Run("ExtractUserRole", func(b *testing.B) {
		ctx := context.WithValue(context.Background(), UserRoleKey, "platform_admin")
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = UserRoleFromContext(ctx)
		}
	})
}
