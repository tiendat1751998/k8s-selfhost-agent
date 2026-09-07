package tenancy

import "context"

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UserRoleKey contextKey = "user_role"
	TenantIDKey contextKey = "tenant_id"
)

// UserIDFromContext retrieves the user ID from context.
func UserIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(UserIDKey).(string); ok {
		return val
	}
	return ""
}

// UserRoleFromContext retrieves the user role from context.
func UserRoleFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(UserRoleKey).(string); ok {
		return val
	}
	return ""
}

// TenantIDFromContext retrieves the tenant ID from context.
func TenantIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(TenantIDKey).(string); ok {
		return val
	}
	return ""
}

// WithTenantID returns a copy of parent context with the given tenant ID attached.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDKey, tenantID)
}

// WithUserID returns a copy of parent context with the given user ID attached.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// WithUserRole returns a copy of parent context with the given user role attached.
func WithUserRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, UserRoleKey, role)
}
