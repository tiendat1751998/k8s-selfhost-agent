package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// Backward compatibility aliases - delegate to pkg/tenancy
var (
	UserIDKey   = tenancy.UserIDKey
	UserRoleKey = tenancy.UserRoleKey
	TenantIDKey = tenancy.TenantIDKey
)

const JWTIssuer = "k8s-selfhost"

const (
	AccessTokenDuration  = 15 * time.Minute
	PartialTokenDuration = 5 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypePartial TokenType = "partial"
)

var jwtSecret []byte

func isRunningInTest() bool {
	if testing.Testing() {
		return true
	}
	if len(os.Args) > 0 {
		base := strings.ToLower(os.Args[0])
		if strings.Contains(base, "test") {
			return true
		}
	}
	return false
}

func init() {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		// During go test execution, allow test runner to use test secret if JWT_SECRET is not explicitly exported in env
		if isRunningInTest() {
			secret = "k8sselfhost-jwt-test-secret-key-32bytes-secure!"
			jwtSecret = []byte(secret)
			return
		}
		log.Fatal("FATAL: JWT_SECRET environment variable must be set (minimum 32 characters)")
	}
	jwtSecret = []byte(secret)
}

// SetJWTSecret allows setting the JWT signing secret programmatically (e.g. for testing).
func SetJWTSecret(secret string) {
	if len(secret) < 32 {
		log.Fatal("FATAL: JWT_SECRET environment variable must be set (minimum 32 characters)")
	}
	jwtSecret = []byte(secret)
}

// JWTClaims holds standard and custom JWT claims.
type JWTClaims struct {
	Sub    string    `json:"sub"`
	Role   string    `json:"role,omitempty"`
	Tenant string    `json:"tenant,omitempty"`
	Type   TokenType `json:"type,omitempty"`
	Exp    int64     `json:"exp"`
	Iat    int64     `json:"iat"`
	Iss    string    `json:"iss"`
}

// GenerateAccessToken creates a secure HMAC-SHA256 JWT access token with 15-minute validity.
func GenerateAccessToken(userID, role, tenantID string) (string, error) {
	header := `{"alg":"HS256","typ":"JWT"}`
	now := time.Now()
	claims := JWTClaims{
		Sub:    userID,
		Role:   role,
		Tenant: tenantID,
		Type:   TokenTypeAccess,
		Exp:    now.Add(AccessTokenDuration).Unix(),
		Iat:    now.Unix(),
		Iss:    JWTIssuer,
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encHeader := base64.RawURLEncoding.EncodeToString([]byte(header))
	encPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingString := encHeader + "." + encPayload

	h := hmac.New(sha256.New, jwtSecret)
	if _, err := h.Write([]byte(signingString)); err != nil {
		return "", err
	}
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	return signingString + "." + signature, nil
}

// GeneratePartialToken creates a limited-scope temporary token (5 min) used between password verification and TOTP confirmation.
func GeneratePartialToken(userID string) (string, error) {
	header := `{"alg":"HS256","typ":"JWT"}`
	now := time.Now()
	claims := JWTClaims{
		Sub:  userID,
		Type: TokenTypePartial,
		Exp:  now.Add(PartialTokenDuration).Unix(),
		Iat:  now.Unix(),
		Iss:  JWTIssuer,
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encHeader := base64.RawURLEncoding.EncodeToString([]byte(header))
	encPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingString := encHeader + "." + encPayload

	h := hmac.New(sha256.New, jwtSecret)
	if _, err := h.Write([]byte(signingString)); err != nil {
		return "", err
	}
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	return signingString + "." + signature, nil
}

// GenerateJWT creates an access token (backward-compatible alias for GenerateAccessToken).
func GenerateJWT(userID, role, tenantID string) (string, error) {
	return GenerateAccessToken(userID, role, tenantID)
}

// GenerateRefreshToken generates a cryptographically secure random 32-byte hex-encoded refresh token.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashToken produces a SHA-256 hex string of the given raw token string.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// ValidateJWT verifies signature and standard claims (exp, iat, iss), returning the payload JSON string.
func ValidateJWT(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("invalid token format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errors.New("failed to decode header")
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return "", errors.New("failed to decode header JSON")
	}
	if header.Alg != "HS256" {
		return "", errors.New("unsupported signing algorithm")
	}

	signingString := parts[0] + "." + parts[1]
	expectedSigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", errors.New("failed to decode signature")
	}

	h := hmac.New(sha256.New, jwtSecret)
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

	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", errors.New("failed to decode payload claims")
	}

	now := time.Now().Unix()
	if claims.Exp == 0 || now > claims.Exp {
		return "", errors.New("token has expired")
	}
	if claims.Iat == 0 || claims.Iat > now+60 {
		return "", errors.New("invalid or future iat claim")
	}
	if claims.Iss != JWTIssuer {
		return "", errors.New("invalid token issuer")
	}

	return string(payloadBytes), nil
}

// ValidatePartialToken checks that a token is a valid unexpired partial token.
func ValidatePartialToken(token string) (*JWTClaims, error) {
	payloadStr, err := ValidateJWT(token)
	if err != nil {
		return nil, err
	}
	var claims JWTClaims
	if err := json.Unmarshal([]byte(payloadStr), &claims); err != nil {
		return nil, errors.New("invalid token claims")
	}
	if claims.Type != TokenTypePartial {
		return nil, errors.New("invalid token type: expected partial token")
	}
	return &claims, nil
}

// Backward compatibility - delegate to pkg/tenancy
func TenantIDFromContext(ctx context.Context) string { return tenancy.TenantIDFromContext(ctx) }
func UserIDFromContext(ctx context.Context) string   { return tenancy.UserIDFromContext(ctx) }
func UserRoleFromContext(ctx context.Context) string { return tenancy.UserRoleFromContext(ctx) }

// JWTAuthMiddleware validates the JWT token in the Authorization header or query parameters (for websockets only).
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

		// Fallback to query parameter "token" ONLY for WebSocket paths (/ws, /ws/..., /api/v1/logs/stream)
		if token == "" && (r.URL.Path == "/ws" || strings.HasPrefix(r.URL.Path, "/ws/") || r.URL.Path == "/api/v1/logs/stream" || strings.HasPrefix(r.URL.Path, "/api/v1/logs/stream")) {
			token = r.URL.Query().Get("token")
		}

		if token == "" {
			http.Error(w, "missing or empty authorization token", http.StatusUnauthorized)
			return
		}

		var claims JWTClaims

		payloadStr, err := ValidateJWT(token)
		if err != nil {
			http.Error(w, "invalid token: signature verification failed", http.StatusUnauthorized)
			return
		}

		if err := json.Unmarshal([]byte(payloadStr), &claims); err != nil {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}

		// REJECT partial tokens - they are only permitted for 2FA verification endpoints
		if claims.Type == TokenTypePartial {
			http.Error(w, "unauthorized: partial token cannot access API", http.StatusUnauthorized)
			return
		}

		userID := "guest"
		userRole := "guest"
		tenantID := "default-tenant"

		if claims.Sub != "" {
			userID = claims.Sub
		}
		if claims.Role != "" {
			userRole = claims.Role
		}
		if claims.Tenant != "" {
			tenantID = claims.Tenant
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
			if !ok || userRole == "" {
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

// RequireRolesForMutations enforces role-based access control on state-modifying HTTP methods
// (POST, PUT, PATCH, DELETE). Safe read-only methods (GET, HEAD, OPTIONS) are allowed
// for all authenticated callers.
func RequireRolesForMutations(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			userRole, ok := r.Context().Value(UserRoleKey).(string)
			if !ok || userRole == "" {
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

// MetricsAuthMiddleware checks for METRICS_TOKEN if configured in the environment.
// If METRICS_TOKEN is not set or empty, requests to /metrics are allowed without authentication.
// If METRICS_TOKEN is set, requests must provide Authorization: Bearer <METRICS_TOKEN>.
func MetricsAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedToken := os.Getenv("METRICS_TOKEN")
		if expectedToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		var token string
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}

		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
			http.Error(w, "unauthorized: invalid or missing metrics token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
