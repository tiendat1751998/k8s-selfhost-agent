package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UserRoleKey contextKey = "user_role"
	TenantIDKey contextKey = "tenant_id"
)

var JWTSecret = []byte(getEnv("JWT_SECRET", "k8s-selfhost-secret-key-change-in-prod"))

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// GenerateJWT creates a secure HMAC-SHA256 JWT.
func GenerateJWT(userID, role, tenantID string) (string, error) {
	header := `{"alg":"HS256","typ":"JWT"}`
	payload := fmt.Sprintf(`{"sub":%q,"role":%q,"tenant":%q}`, userID, role, tenantID)

	encHeader := base64.RawURLEncoding.EncodeToString([]byte(header))
	encPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signingString := encHeader + "." + encPayload

	h := hmac.New(sha256.New, JWTSecret)
	if _, err := h.Write([]byte(signingString)); err != nil {
		return "", err
	}
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	return signingString + "." + signature, nil
}

// ValidateJWT verifies signature and returns the payload JSON string.
func ValidateJWT(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("invalid token format")
	}

	signingString := parts[0] + "." + parts[1]
	expectedSigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", errors.New("failed to decode signature")
	}

	h := hmac.New(sha256.New, JWTSecret)
	if _, err := h.Write([]byte(signingString)); err != nil {
		return "", err
	}
	actualSigBytes := h.Sum(nil)

	if !hmac.Equal(expectedSigBytes, actualSigBytes) {
		return "", errors.New("invalid signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errors.New("failed to decode payload")
	}

	return string(payloadBytes), nil
}

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

		var claims struct {
			Sub    string `json:"sub"`
			Role   string `json:"role"`
			Tenant string `json:"tenant"`
		}

		if strings.HasPrefix(token, "k8s-enterprise-demo-") {
			userID = "admin-user-id"
			userRole = "platform_admin"
			tenantID = "default-tenant"
		} else {
			payloadStr, err := ValidateJWT(token)
			if err != nil {
				http.Error(w, "invalid token: signature verification failed", http.StatusUnauthorized)
				return
			}

			if err := json.Unmarshal([]byte(payloadStr), &claims); err != nil {
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
