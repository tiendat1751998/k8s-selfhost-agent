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

const (
	JWTIssuer   = "k8s-selfhost"
	headerHS256 = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
)

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

	encPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingString := headerHS256 + "." + encPayload

	h := hmac.New(sha256.New, jwtSecret)
	_, _ = h.Write([]byte(signingString))
	var sigBuf [32]byte
	sig := base64.RawURLEncoding.EncodeToString(h.Sum(sigBuf[:0]))

	return signingString + "." + sig, nil
}

// GeneratePartialToken creates a limited-scope temporary token (5 min) used between password verification and TOTP confirmation.
func GeneratePartialToken(userID string) (string, error) {
	if len(jwtSecret) < 32 {
		return "", ErrJWTSecretMissing
	}

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

	encPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingString := headerHS256 + "." + encPayload

	h := hmac.New(sha256.New, jwtSecret)
	_, _ = h.Write([]byte(signingString))
	var sigBuf [32]byte
	sig := base64.RawURLEncoding.EncodeToString(h.Sum(sigBuf[:0]))

	return signingString + "." + sig, nil
}

// GenerateJWT creates an access token (backward-compatible alias for GenerateAccessToken).
func GenerateJWT(userID, role, tenantID string) (string, error) { return GenerateAccessToken(userID, role, tenantID) }

// GenerateToken creates an access token (alias for GenerateAccessToken).
func GenerateToken(userID, role, tenantID string) (string, error) { return GenerateAccessToken(userID, role, tenantID) }

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

	dot1 := strings.IndexByte(token, '.')
	dot2 := -1
	if dot1 >= 0 {
		if d := strings.IndexByte(token[dot1+1:], '.'); d >= 0 {
			dot2 = dot1 + 1 + d
		}
	}
	if dot1 < 0 || dot2 < 0 || strings.IndexByte(token[dot2+1:], '.') >= 0 {
		return nil, nil, errors.New("invalid token format")
	}
	p0, p1, p2 := token[:dot1], token[dot1+1:dot2], token[dot2+1:]

	if p0 != headerHS256 {
		headerBytes, err := base64.RawURLEncoding.DecodeString(p0)
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
	}

	signingString := token[:dot2]
	var expectedSig [48]byte
	n, err := base64.RawURLEncoding.Decode(expectedSig[:], []byte(p2))
	if err != nil {
		return nil, nil, errors.New("failed to decode signature")
	}

	h := hmac.New(sha256.New, jwtSecret)
	_, _ = h.Write([]byte(signingString))
	var actualSig [32]byte
	actualSigSlice := h.Sum(actualSig[:0])

	if n != len(actualSigSlice) || subtle.ConstantTimeCompare(expectedSig[:n], actualSigSlice) != 1 {
		return nil, nil, errors.New("invalid signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(p1)
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
func ValidateToken(token string) (string, error) { return ValidateJWT(token) }

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
	if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" ||
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") ||
		strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
		return true
	}
	path := r.URL.Path
	if path == "/ws" || strings.HasPrefix(path, "/ws/") ||
		path == "/stream" || strings.HasSuffix(path, "/stream") || strings.Contains(path, "/stream/") || strings.Contains(path, "/stream") ||
		strings.Contains(path, "/exec") {
		return true
	}
	if (strings.Contains(path, "/logs") || strings.Contains(path, "/pods/")) &&
		(r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1") {
		return true
	}
	return false
}

// authCtx avoids multiple heap-allocated context.WithValue wrappers on authenticated requests.
type authCtx struct {
	context.Context
	userID   string
	userRole string
	tenantID string
}

func (c authCtx) Value(key any) any {
	switch key {
	case tenancy.UserIDKey:
		return c.userID
	case tenancy.UserRoleKey:
		return c.userRole
	case tenancy.TenantIDKey:
		return c.tenantID
	default:
		return c.Context.Value(key)
	}
}

// JWTAuthMiddleware validates the JWT token in the Authorization header or query parameters (for websockets and streaming).
func JWTAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if matchAgentAuthToken(r) {
			ctx := authCtx{
				Context:  r.Context(),
				userID:   "edge-agent",
				userRole: "platform_admin",
				tenantID: "default-tenant",
			}
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

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
			prefix, rest, found := strings.Cut(authHeader, " ")
			if found && strings.EqualFold(prefix, "Bearer") {
				token = rest
			}
		}

		if token == "" {
			if secProto := r.Header.Get("Sec-WebSocket-Protocol"); secProto != "" {
				token = extractTokenFromSecWebSocketProtocol(secProto)
			}
		}

		if token == "" && IsStreamingRequest(r) {
			token = r.URL.Query().Get("token")
		}

		if token == "" {
			http.Error(w, "missing or empty authorization token", http.StatusUnauthorized)
			return
		}

		if IsTokenRevoked(token) {
			http.Error(w, "unauthorized: token has been revoked", http.StatusUnauthorized)
			return
		}

		if strings.HasPrefix(token, "k8s_live_") {
			if defaultAPIKeyValidator == nil {
				http.Error(w, "unauthorized: api key authentication not configured", http.StatusUnauthorized)
				return
			}
			tokenHash := HashToken(token)
			keyInfo, err := defaultAPIKeyValidator.ValidateAPIKey(r.Context(), tokenHash)
			if err != nil || keyInfo == nil {
				http.Error(w, "unauthorized: invalid or revoked api key", http.StatusUnauthorized)
				return
			}
			userID := "apikey:" + keyInfo.KeyID
			userRole := keyInfo.Role
			tenantID := keyInfo.TenantID
			if tenantID == "" {
				tenantID = "default-tenant"
			}
			ctx := authCtx{Context: r.Context(), userID: userID, userRole: userRole, tenantID: tenantID}
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		claims, err := ValidateTokenClaims(token)
		if err != nil {
			http.Error(w, "invalid token: signature verification failed", http.StatusUnauthorized)
			return
		}

		if secProto := r.Header.Get("Sec-WebSocket-Protocol"); secProto != "" {
			if strings.Contains(strings.ToLower(secProto), "bearer") {
				w.Header().Set("Sec-WebSocket-Protocol", "bearer")
			}
		}

		if claims.Type == TokenTypePartial {
			http.Error(w, "unauthorized: partial token cannot access API", http.StatusUnauthorized)
			return
		}

		userID, userRole, tenantID := "guest", "guest", "default-tenant"
		if claims.Sub != "" {
			userID = claims.Sub
		}
		if claims.Role != "" {
			userRole = claims.Role
		}
		if claims.Tenant != "" {
			tenantID = claims.Tenant
		}

		ctx := authCtx{Context: r.Context(), userID: userID, userRole: userRole, tenantID: tenantID}
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
	rbac := RBACMiddleware(requiredRoles...)
	return func(next http.Handler) http.Handler {
		rbacHandler := rbac(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			rbacHandler.ServeHTTP(w, r)
		})
	}
}
