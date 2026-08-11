package http

import (
	"net/http"
	"strings"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	authUsecase "github.com/datdt/k8sselfhost/internal/usecase/auth"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	usecase *authUsecase.Usecase
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(usecase *authUsecase.Usecase) *AuthHandler {
	return &AuthHandler{usecase: usecase}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *loginRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Email) == "" {
		ve.Add("email", "email is required")
	}
	if strings.TrimSpace(r.Password) == "" {
		ve.Add("password", "password is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
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
	req, ok := decodeJSON[loginRequest](w, r)
	if !ok {
		return
	}

	res, err := h.usecase.Authenticate(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials", nil)
		return
	}

	// Generate compliant JWT structure (header.payload.signature) signed with HMAC-SHA256
	token, err := middleware.GenerateJWT(res.UserID, res.Role, res.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token", err)
		return
	}

	resp := loginResponse{
		Token: token,
		User: loginUser{
			ID:       res.UserID,
			Role:     res.Role,
			TenantID: res.TenantID,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}
