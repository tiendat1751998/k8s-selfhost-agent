package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// resolveTenant derives the tenant ID strictly from authenticated claims in context,
// honoring the client X-Tenant-ID header only if the caller's verified role is platform_admin.
func resolveTenant(ctx context.Context, r *http.Request) string {
	if ctx == nil && r != nil {
		ctx = r.Context()
	}
	if ctx == nil {
		return ""
	}

	if tenancy.UserRoleFromContext(ctx) == "platform_admin" {
		if r != nil {
			if h := strings.TrimSpace(r.Header.Get("X-Tenant-ID")); h != "" {
				return h
			}
		}
	}

	return strings.TrimSpace(tenancy.TenantIDFromContext(ctx))
}
