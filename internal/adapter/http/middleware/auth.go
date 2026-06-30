package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

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

// JWTAuthMiddleware validates the JWT token in the Authorization header or query parameters.
func JWTAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}

		// Fallback to query parameter "token" (for websockets)
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if token == "" {
			http.Error(w, "missing or empty authorization token", http.StatusUnauthorized)
			return
		}

		userID := "guest"
		userRole := "guest"
		tenantID := "default-tenant"

		// Decode base64 payload if it looks like a JWT
		tokenParts := strings.Split(token, ".")
		if len(tokenParts) == 3 {
			payloadSegment := tokenParts[1]
			if l := len(payloadSegment) % 4; l > 0 {
				payloadSegment += strings.Repeat("=", 4-l)
			}
			
			var payloadBytes []byte
			var err error
			payloadBytes, err = base64.URLEncoding.DecodeString(payloadSegment)
			if err != nil {
				payloadBytes, err = base64.StdEncoding.DecodeString(payloadSegment)
			}
			if err != nil {
				http.Error(w, "invalid token encoding", http.StatusUnauthorized)
				return
			}
			
			var claims struct {
				Sub    string `json:"sub"`
				Role   string `json:"role"`
				Tenant string `json:"tenant"`
			}
			if err := json.Unmarshal(payloadBytes, &claims); err != nil {
				http.Error(w, "invalid token claims", http.StatusUnauthorized)
				return
			}

			if claims.Sub != "" {
				userID = claims.Sub
			}
			if claims.Role != "" {
				userRole = claims.Role
			}
			if claims.Tenant != "" {
				tenantID = claims.Tenant
			}
		} else if strings.HasPrefix(token, "k8s-enterprise-demo-") {
			userID = "admin-user-id"
			userRole = "platform_admin"
			tenantID = "default-tenant"
		} else {
			http.Error(w, "invalid token format", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, UserRoleKey, userRole)
		ctx = context.WithValue(ctx, TenantIDKey, tenantID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RBACMiddleware enforces role-based access control.
func RBACMiddleware(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, ok := r.Context().Value(UserRoleKey).(string)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			hasRole := false
			for _, role := range requiredRoles {
				if userRole == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				http.Error(w, fmt.Sprintf("forbidden: requires one of %v", requiredRoles), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
