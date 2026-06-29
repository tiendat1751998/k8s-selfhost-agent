package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	pool *pgxpool.Pool
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(pool *pgxpool.Pool) *AuthHandler {
	return &AuthHandler{pool: pool}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string    `json:"token"`
	User  loginUser `json:"user"`
}

type loginUser struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required", nil)
		return
	}

	var userID string
	var passwordHash string
	var platformRole string
	var tenantID string
	var tenantRole string

	query := `
		SELECT 
			u.id::text, 
			u.password_hash, 
			COALESCE(r_platform.name, '') as platform_role,
			COALESCE(tb.tenant_id, 'default-tenant') as tenant_id,
			COALESCE(r_tenant.name, 'viewer') as tenant_role
		FROM users u
		LEFT JOIN user_roles ur ON u.id = ur.user_id
		LEFT JOIN roles r_platform ON ur.role_id = r_platform.id AND r_platform.name = 'platform_admin'
		LEFT JOIN tenant_bindings tb ON u.id = tb.user_id
		LEFT JOIN roles r_tenant ON tb.role_id = r_tenant.id
		WHERE u.email = $1
		LIMIT 1
	`
	err := h.pool.QueryRow(r.Context(), query, req.Email).Scan(&userID, &passwordHash, &platformRole, &tenantID, &tenantRole)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials", nil)
		return
	}

	// Verify password hash
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials", nil)
		return
	}

	userRole := tenantRole
	if platformRole == "platform_admin" {
		userRole = "platform_admin"
	}

	// Generate compliant JWT structure (header.payload.signature)
	header := `{"alg":"HS256","typ":"JWT"}`
	payload := fmt.Sprintf(`{"sub":%q,"role":%q,"tenant":%q}`, userID, userRole, tenantID)

	encHeader := base64.RawURLEncoding.EncodeToString([]byte(header))
	encPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	token := fmt.Sprintf("%s.%s.signature", encHeader, encPayload)

	resp := loginResponse{
		Token: token,
		User: loginUser{
			ID:       userID,
			Role:     userRole,
			TenantID: tenantID,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}
