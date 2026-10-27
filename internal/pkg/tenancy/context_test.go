package tenancy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestWithPlatformAdminContext_SetsRoleAndDefaultTenant(t *testing.T) {
	ctx := context.Background()
	adminCtx := tenancy.WithPlatformAdminContext(ctx)

	assert.Equal(t, "platform_admin", tenancy.UserRoleFromContext(adminCtx))
	assert.Equal(t, "default-tenant", tenancy.TenantIDFromContext(adminCtx))
}

func TestWithPlatformAdminContext_PreservesExistingTenant(t *testing.T) {
	ctx := tenancy.WithTenantID(context.Background(), "custom-tenant")
	adminCtx := tenancy.WithPlatformAdminContext(ctx)

	assert.Equal(t, "platform_admin", tenancy.UserRoleFromContext(adminCtx))
	assert.Equal(t, "custom-tenant", tenancy.TenantIDFromContext(adminCtx))
}
