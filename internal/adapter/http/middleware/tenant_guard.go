package middleware

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// RequireTenantParam enforces tenant isolation against URL parameters.
// Platform administrators bypass all checks. Non-administrators must have a tenant in context,
// and at least one specified URL param matching a tenant/org identifier must exist and match the caller's tenant.
func RequireTenantParam(paramNames ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := tenancy.UserRoleFromContext(r.Context())
			if role == tenancy.PlatformAdminRole {
				next.ServeHTTP(w, r)
				return
			}

			userTenant := tenancy.TenantIDFromContext(r.Context())
			if strings.TrimSpace(userTenant) == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				if err := json.NewEncoder(w).Encode(map[string]string{
					"error": "access denied: missing tenant context",
				}); err != nil {
					zap.L().Warn("failed to encode missing tenant context response", zap.Error(err))
				}
				return
			}

			matchedAny := false
			for _, param := range paramNames {
				raw := chi.URLParam(r, param)
				if raw != "" {
					matchedAny = true
					unescaped, err := url.PathUnescape(raw)
					if err != nil {
						unescaped = raw
					}
					target := strings.TrimSpace(unescaped)
					if target == "" {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusForbidden)
						if err := json.NewEncoder(w).Encode(map[string]string{
							"error": "access denied: invalid empty tenant parameter",
						}); err != nil {
							zap.L().Warn("failed to encode invalid empty tenant parameter response", zap.Error(err))
						}
						return
					}
					if !strings.EqualFold(target, userTenant) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusForbidden)
						if err := json.NewEncoder(w).Encode(map[string]string{
							"error": "access denied: cross-tenant access prohibited",
						}); err != nil {
							zap.L().Warn("failed to encode cross-tenant error response", zap.Error(err))
						}
						return
					}
				}
			}

			if !matchedAny || len(paramNames) == 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				if err := json.NewEncoder(w).Encode(map[string]string{
					"error": "access denied: missing required tenant parameter",
				}); err != nil {
					zap.L().Warn("failed to encode missing tenant param response", zap.Error(err))
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
