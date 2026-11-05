package tenancy_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestRequestMeta_LifecycleAndThreadSafety(t *testing.T) {
	ctx := context.Background()

	// Initially context has no RequestMeta
	assert.Nil(t, tenancy.RequestMetaFromContext(ctx))
	assert.False(t, tenancy.IsAuditRecorded(ctx))

	// EnsureRequestMeta returns an enriched context and a non-nil RequestMeta
	ctx, meta := tenancy.EnsureRequestMeta(ctx)
	require.NotNil(t, meta)
	assert.Same(t, meta, tenancy.RequestMetaFromContext(ctx))

	// Calling EnsureRequestMeta again on enriched context returns the same pointer
	ctx2, meta2 := tenancy.EnsureRequestMeta(ctx)
	assert.Same(t, meta, meta2)
	assert.Same(t, ctx, ctx2)

	// SetRequestIdentity updates the fields
	tenancy.SetRequestIdentity(ctx, "usr-123", "tenant_admin", "org-alpha", "jwt")
	assert.Equal(t, "usr-123", meta.UserID())
	assert.Equal(t, "tenant_admin", meta.UserRole())
	assert.Equal(t, "org-alpha", meta.TenantID())
	assert.Equal(t, "jwt", meta.AuthMethod())

	// Audit recorded flag starts false
	assert.False(t, tenancy.IsAuditRecorded(ctx))
	tenancy.MarkAuditRecorded(ctx)
	assert.True(t, tenancy.IsAuditRecorded(ctx))

	// Concurrent read and write to RequestMeta
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			tenancy.SetRequestIdentity(ctx, "u-concurrent", "operator", "t-concurrent", "api_key")
		}()
		go func() {
			defer wg.Done()
			_ = meta.UserID()
			_ = meta.UserRole()
			_ = meta.TenantID()
			_ = meta.AuthMethod()
			_ = tenancy.IsAuditRecorded(ctx)
		}()
	}
	wg.Wait()
}
