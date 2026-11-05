package tenancy

import (
	"context"
	"sync"
	"sync/atomic"
)

type requestMetaKey struct{}

// RequestMeta carries thread-safe request identity and lifecycle metadata across middleware layers.
type RequestMeta struct {
	mu            sync.RWMutex
	userID        string
	userRole      string
	tenantID      string
	authMethod    string
	auditRecorded atomic.Bool
}

// UserID returns the authenticated user ID.
func (m *RequestMeta) UserID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.userID
}

// UserRole returns the authenticated user role.
func (m *RequestMeta) UserRole() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.userRole
}

// TenantID returns the tenant ID.
func (m *RequestMeta) TenantID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tenantID
}

// AuthMethod returns the authentication method (e.g. "jwt", "api_key", "agent_token").
func (m *RequestMeta) AuthMethod() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.authMethod
}

// SetIdentity atomically sets the identity fields on the RequestMeta.
func (m *RequestMeta) SetIdentity(userID, role, tenantID, authMethod string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userID = userID
	m.userRole = role
	m.tenantID = tenantID
	m.authMethod = authMethod
}

// MarkAuditRecorded flags that mutation audit logging has been completed for this request.
func (m *RequestMeta) MarkAuditRecorded() {
	m.auditRecorded.Store(true)
}

// IsAuditRecorded returns true if mutation audit logging has been completed.
func (m *RequestMeta) IsAuditRecorded() bool {
	return m.auditRecorded.Load()
}

// EnsureRequestMeta ensures that ctx contains a *RequestMeta carrier, allocating one if absent.
func EnsureRequestMeta(ctx context.Context) (context.Context, *RequestMeta) {
	if meta, ok := ctx.Value(requestMetaKey{}).(*RequestMeta); ok && meta != nil {
		return ctx, meta
	}
	meta := &RequestMeta{}
	return context.WithValue(ctx, requestMetaKey{}, meta), meta
}

// RequestMetaFromContext retrieves the *RequestMeta from ctx, or nil if not present.
func RequestMetaFromContext(ctx context.Context) *RequestMeta {
	if ctx == nil {
		return nil
	}
	if meta, ok := ctx.Value(requestMetaKey{}).(*RequestMeta); ok {
		return meta
	}
	return nil
}

// SetRequestIdentity updates the identity fields on the *RequestMeta present in ctx, if any.
func SetRequestIdentity(ctx context.Context, userID, role, tenantID, authMethod string) {
	if meta := RequestMetaFromContext(ctx); meta != nil {
		meta.SetIdentity(userID, role, tenantID, authMethod)
	}
}

// MarkAuditRecorded marks the audit recorded flag on the *RequestMeta in ctx, if present.
func MarkAuditRecorded(ctx context.Context) {
	if meta := RequestMetaFromContext(ctx); meta != nil {
		meta.MarkAuditRecorded()
	}
}

// IsAuditRecorded returns true if *RequestMeta in ctx has its audit recorded flag set.
func IsAuditRecorded(ctx context.Context) bool {
	if meta := RequestMetaFromContext(ctx); meta != nil {
		return meta.IsAuditRecorded()
	}
	return false
}
