package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// DefaultTenantID is the canonical tenant slug for single-tenant / default enterprise operations.
const DefaultTenantID = "default-tenant"

// DefaultTenantUUID is the canonical fallback UUID associated with DefaultTenantID.
const DefaultTenantUUID = "00000000-0000-0000-0000-000000000001"

// EnsureTenantContext returns a context guaranteed to contain a non-empty tenant ID.
// If the context contains no tenant ID or an empty string, it is enriched with DefaultTenantID.
func EnsureTenantContext(ctx context.Context) context.Context {
	if tid := tenancy.TenantIDFromContext(ctx); tid == "" {
		return tenancy.WithTenantID(ctx, DefaultTenantID)
	}
	return ctx
}

// EnsurePlatformAdminContext returns a context enriched with platform_admin role and DefaultTenantID if empty.
func EnsurePlatformAdminContext(ctx context.Context) context.Context {
	return tenancy.WithPlatformAdminContext(ctx)
}

// TenantIDOrDefault returns the tenant ID from context, or DefaultTenantID ("default-tenant") if empty.
func TenantIDOrDefault(ctx context.Context) string {
	if tid := tenancy.TenantIDFromContext(ctx); tid != "" {
		return tid
	}
	return DefaultTenantID
}

// DeterministicTenantUUID returns a stable UUID for any tenant slug or UUID string.
// If the input is empty or DefaultTenantID, it returns DefaultTenantUUID.
// If the input is already a valid UUID string, it returns it directly.
// Otherwise, it computes a deterministic SHA-1 UUID in the DNS namespace.
func DeterministicTenantUUID(slug string) string {
	if slug == "" || slug == DefaultTenantID {
		return DefaultTenantUUID
	}
	if _, err := uuid.Parse(slug); err == nil {
		return slug
	}
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(slug)).String()
}

// DeterministicSlugForUUID resolves a UUID back to a slug if it matches known defaults.
func DeterministicSlugForUUID(id string) string {
	if id == "" || id == DefaultTenantUUID || id == "00000000-0000-0000-0000-000000000000" {
		return DefaultTenantID
	}
	return id
}
