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
	"net/http"
	"os"
	"strings"
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

var ErrJWTSecretMissing = errors.New("JWT_SECRET environment variable must be set (minimum 32 characters)")

func init() {
	if secret := os.Getenv("JWT_SECRET"); len(secret) >= 32 {
		jwtSecret = []byte(secret)
	}
}

// ValidateJWTSecret validates that the JWT signing secret is configured and meets length requirements.
func ValidateJWTSecret() error {
	if len(jwtSecret) < 32 {
		return ErrJWTSecretMissing
	}
	return nil
}

// SetJWTSecret allows setting the JWT signing secret programmatically (e.g. for testing).
func SetJWTSecret(secret string) error {
	if len(secret) < 32 {
		return ErrJWTSecretMissing
	}
	jwtSecret = []byte(secret)
	return nil
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
	if len(jwtSecret) < 32 {
		return "", ErrJWTSecretMissing
	}

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
	if len(jwtSecret) < 32 {
		return "", ErrJWTSecretMissing
	}

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

// GenerateToken creates an access token (alias for GenerateAccessToken).
func GenerateToken(userID, role, tenantID string) (string, error) {
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

// parseAndValidateToken performs signature verification and standard claim checks,
// deserializing the claims JSON payload exactly once.
func parseAndValidateToken(token string) (*JWTClaims, []byte, error) {
	if len(jwtSecret) < 32 {
		return nil, nil, ErrJWTSecretMissing
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, errors.New("invalid token format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, errors.New("failed to decode header")
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, nil, errors.New("failed to decode header JSON")
	}
	if header.Alg != "HS256" {
		return nil, nil, errors.New("unsupported signing algorithm")
	}

	signingString := parts[0] + "." + parts[1]
	expectedSigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, nil, errors.New("failed to decode signature")
	}

	h := hmac.New(sha256.New, jwtSecret)
	if _, err := h.Write([]byte(signingString)); err != nil {
		return nil, nil, err
	}
	actualSigBytes := h.Sum(nil)

	if !hmac.Equal(expectedSigBytes, actualSigBytes) {
		return nil, nil, errors.New("invalid signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, errors.New("failed to decode payload")
	}

	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, nil, errors.New("failed to decode payload claims")
	}

	now := time.Now().Unix()
	if claims.Exp == 0 || now > claims.Exp {
		return nil, nil, errors.New("token has expired")
	}
	if claims.Iat == 0 || claims.Iat > now+60 {
		return nil, nil, errors.New("invalid or future iat claim")
	}
	if claims.Iss != JWTIssuer {
		return nil, nil, errors.New("invalid token issuer")
	}

	return &claims, payloadBytes, nil
}

// ValidateJWT verifies signature and standard claims (exp, iat, iss), returning the payload JSON string.
func ValidateJWT(token string) (string, error) {
	_, payloadBytes, err := parseAndValidateToken(token)
	if err != nil {
		return "", err
	}
	return string(payloadBytes), nil
}

// ValidateToken verifies signature and standard claims, returning the payload JSON string.
func ValidateToken(token string) (string, error) {
	return ValidateJWT(token)
}

// ValidateTokenClaims verifies the token and parses its claims into JWTClaims.
func ValidateTokenClaims(token string) (*JWTClaims, error) {
	claims, _, err := parseAndValidateToken(token)
	return claims, err
}

// ValidatePartialToken checks that a token is a valid unexpired partial token.
func ValidatePartialToken(token string) (*JWTClaims, error) {
	claims, _, err := parseAndValidateToken(token)
	if err != nil {
		return nil, err
	}
	if claims.Type != TokenTypePartial {
		return nil, errors.New("invalid token type: expected partial token")
	}
	return claims, nil
}

// Backward compatibility - delegate to pkg/tenancy
func TenantIDFromContext(ctx context.Context) string { return tenancy.TenantIDFromContext(ctx) }
func UserIDFromContext(ctx context.Context) string   { return tenancy.UserIDFromContext(ctx) }
func UserRoleFromContext(ctx context.Context) string { return tenancy.UserRoleFromContext(ctx) }

// IsStreamingRequest determines if an incoming HTTP request is a streaming connection
// (WebSocket, SSE / EventSource, log tail, or interactive exec).
func IsStreamingRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	// 1. WebSocket upgrade headers
	if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" ||
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		return true
	}
	// 2. Server-Sent Events (SSE) Accept header
	if strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
		return true
	}
	// 3. Known streaming routes
	path := r.URL.Path
	if path == "/ws" || strings.HasPrefix(path, "/ws/") {
		return true
	}
	if path == "/stream" || strings.HasSuffix(path, "/stream") || strings.Contains(path, "/stream/") || strings.Contains(path, "/stream") {
		return true
	}
	if strings.Contains(path, "/exec") {
		return true
	}
	if (strings.Contains(path, "/logs") || strings.Contains(path, "/pods/")) &&
		(r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1") {
		return true
	}
	return false
}

// extractTokenFromSecWebSocketProtocol extracts JWT token from Sec-WebSocket-Protocol header.
// Supported formats:
// 1. "bearer, <token>" or "<token>, bearer" (standard browser WebSocket subprotocol array)
// 2. "bearer <token>" or "Bearer <token>"
func extractTokenFromSecWebSocketProtocol(header string) string {
	if header == "" {
		return ""
	}
	if strings.Contains(header, ",") {
		parts := strings.Split(header, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(trimmed), "bearer ") {
				return strings.TrimSpace(trimmed[7:])
			}
			if !strings.EqualFold(trimmed, "bearer") && trimmed != "" {
				return trimmed
			}
		}
	}
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

// JWTAuthMiddleware validates the JWT token in the Authorization header or query parameters (for websockets and streaming).
func JWTAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(jwtSecret) < 32 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(`{"error":"JWT authentication not configured"}`)); err != nil {
				return
			}
			return
		}

		var token string
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				token = parts[1]
			}
		}

		// Support extracting token from Sec-WebSocket-Protocol header
		if token == "" {
			if secProto := r.Header.Get("Sec-WebSocket-Protocol"); secProto != "" {
				token = extractTokenFromSecWebSocketProtocol(secProto)
			}
		}

		// Fallback to query parameter "token" for WebSocket upgrade connections and streaming/SSE routes
		if token == "" {
			if IsStreamingRequest(r) {
				token = r.URL.Query().Get("token")
			}
		}

		if token == "" {
			http.Error(w, "missing or empty authorization token", http.StatusUnauthorized)
			return
		}

		claims, err := ValidateTokenClaims(token)
		if err != nil {
			http.Error(w, "invalid token: signature verification failed", http.StatusUnauthorized)
			return
		}

		// If client connected via Sec-WebSocket-Protocol with bearer, echo back Sec-WebSocket-Protocol: bearer
		if secProto := r.Header.Get("Sec-WebSocket-Protocol"); secProto != "" {
			if strings.Contains(strings.ToLower(secProto), "bearer") {
				w.Header().Set("Sec-WebSocket-Protocol", "bearer")
			}
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

// MetricsAuthMiddleware enforces authentication for the /metrics endpoint.
// It requires an Authorization: Bearer <token> header matching METRICS_TOKEN
// (if METRICS_TOKEN is set) or a valid JWT bearer token.
// If METRICS_TOKEN is unset and no valid JWT is provided, requests return 401 Unauthorized.
func MetricsAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		var token string
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}

		if token == "" {
			http.Error(w, "unauthorized: missing or invalid authorization header", http.StatusUnauthorized)
			return
		}

		expectedToken := os.Getenv("METRICS_TOKEN")
		if expectedToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) == 1 {
			next.ServeHTTP(w, r)
			return
		}

		// Also permit valid JWT tokens
		if _, err := ValidateJWT(token); err == nil {
			next.ServeHTTP(w, r)
			return
		}

		http.Error(w, "unauthorized: invalid metrics token", http.StatusUnauthorized)
	})
}
