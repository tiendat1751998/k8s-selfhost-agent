package http

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/user"
	authUsecase "github.com/datdt/k8sselfhost/internal/usecase/auth"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	usecase          *authUsecase.Usecase
	userRepo         user.Repository
	refreshTokenRepo user.RefreshTokenRepository
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(
	usecase *authUsecase.Usecase,
	userRepo user.Repository,
	refreshTokenRepo user.RefreshTokenRepository,
) *AuthHandler {
	return &AuthHandler{
		usecase:          usecase,
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
	}
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
	Token        string     `json:"token,omitempty"`
	MFARequired  bool       `json:"mfa_required,omitempty"`
	PartialToken string     `json:"partial_token,omitempty"`
	User         *loginUser `json:"user,omitempty"`
}

type loginUser struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
}

func setRefreshTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   false, // true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(middleware.RefreshTokenDuration.Seconds()),
	})
}

func clearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// Login handles POST /api/v1/auth/login — Step 1
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

	if res.MFARequired {
		partialToken, err := middleware.GeneratePartialToken(res.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate partial token", err)
			return
		}
		writeJSON(w, http.StatusOK, loginResponse{
			MFARequired:  true,
			PartialToken: partialToken,
		})
		return
	}

	accessToken, err := middleware.GenerateAccessToken(res.UserID, res.Role, res.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token", err)
		return
	}

	rawRefreshToken, err := middleware.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate refresh token", err)
		return
	}

	if h.refreshTokenRepo != nil {
		rt := &user.RefreshToken{
			UserID:    res.UserID,
			TokenHash: middleware.HashToken(rawRefreshToken),
			ExpiresAt: time.Now().Add(middleware.RefreshTokenDuration),
			UserAgent: r.UserAgent(),
			IPAddress: r.RemoteAddr,
		}
		if err := h.refreshTokenRepo.Create(r.Context(), rt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to persist refresh token", err)
			return
		}
	}

	setRefreshTokenCookie(w, rawRefreshToken)

	resp := loginResponse{
		Token: accessToken,
		User: &loginUser{
			ID:       res.UserID,
			Role:     res.Role,
			TenantID: res.TenantID,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// RefreshToken handles POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		writeError(w, http.StatusUnauthorized, "missing refresh token cookie", nil)
		return
	}

	if h.refreshTokenRepo == nil || h.userRepo == nil {
		writeError(w, http.StatusInternalServerError, "auth repositories not configured", nil)
		return
	}

	tokenHash := middleware.HashToken(cookie.Value)
	rt, err := h.refreshTokenRepo.GetByHash(r.Context(), tokenHash)
	if err != nil || rt == nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token", nil)
		return
	}

	if rt.RevokedAt != nil {
		writeError(w, http.StatusUnauthorized, "refresh token has been revoked", nil)
		return
	}

	if time.Now().After(rt.ExpiresAt) {
		writeError(w, http.StatusUnauthorized, "refresh token has expired", nil)
		return
	}

	// Revoke the old token (rotation)
	if err := h.refreshTokenRepo.Revoke(r.Context(), rt.ID); err != nil {
		log.Printf("failed to revoke old refresh token during rotation: %v", err)
	}

	usr, err := h.userRepo.GetByID(r.Context(), rt.UserID)
	if err != nil || usr == nil {
		writeError(w, http.StatusUnauthorized, "user not found", nil)
		return
	}

	role := usr.TenantRole
	if usr.PlatformRole == "platform_admin" {
		role = "platform_admin"
	}

	accessToken, err := middleware.GenerateAccessToken(usr.ID, role, usr.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate access token", err)
		return
	}

	newRawRefreshToken, err := middleware.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate refresh token", err)
		return
	}

	newRT := &user.RefreshToken{
		UserID:    usr.ID,
		TokenHash: middleware.HashToken(newRawRefreshToken),
		ExpiresAt: time.Now().Add(middleware.RefreshTokenDuration),
		UserAgent: r.UserAgent(),
		IPAddress: r.RemoteAddr,
	}
	if err := h.refreshTokenRepo.Create(r.Context(), newRT); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist new refresh token", err)
		return
	}

	setRefreshTokenCookie(w, newRawRefreshToken)

	writeJSON(w, http.StatusOK, map[string]string{
		"token": accessToken,
	})
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil && cookie.Value != "" && h.refreshTokenRepo != nil {
		tokenHash := middleware.HashToken(cookie.Value)
		if rt, err := h.refreshTokenRepo.GetByHash(r.Context(), tokenHash); err == nil && rt != nil {
			if err := h.refreshTokenRepo.Revoke(r.Context(), rt.ID); err != nil {
				log.Printf("failed to revoke refresh token during logout: %v", err)
			}
		}
	}

	clearRefreshTokenCookie(w)

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}