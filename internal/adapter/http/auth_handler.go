package http

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/user"
	authUsecase "github.com/datdt/k8sselfhost/internal/usecase/auth"
)

// AuthHandlerOption configures AuthHandler.
type AuthHandlerOption func(*AuthHandler)

// WithCookieSecure sets whether cookies should have the Secure flag enabled.
func WithCookieSecure(secure bool) AuthHandlerOption {
	return func(h *AuthHandler) {
		h.cookieSecure = secure
	}
}

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	usecase          *authUsecase.Usecase
	userRepo         user.Repository
	refreshTokenRepo user.RefreshTokenRepository
	cookieSecure     bool
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(
	usecase *authUsecase.Usecase,
	userRepo user.Repository,
	refreshTokenRepo user.RefreshTokenRepository,
	opts ...AuthHandlerOption,
) *AuthHandler {
	cookieSecure := true
	if val := os.Getenv("COOKIE_SECURE"); strings.EqualFold(val, "false") {
		cookieSecure = false
	} else if val := os.Getenv("K8S_COOKIE_SECURE"); strings.EqualFold(val, "false") {
		cookieSecure = false
	}
	h := &AuthHandler{
		usecase:          usecase,
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		cookieSecure:     cookieSecure,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
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

func getFirstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func isCookieSecure() bool {
	if val := getFirstEnv("COOKIE_SECURE", "K8S_COOKIE_SECURE"); strings.EqualFold(val, "false") {
		return false
	}
	return true
}

func (h *AuthHandler) isCookieSecure() bool {
	return h.cookieSecure
}

func setRefreshTokenCookie(w http.ResponseWriter, token string, secure ...bool) {
	isSecure := isCookieSecure()
	if len(secure) > 0 {
		isSecure = secure[0]
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(middleware.RefreshTokenDuration.Seconds()),
	})
}

func clearRefreshTokenCookie(w http.ResponseWriter, secure ...bool) {
	isSecure := isCookieSecure()
	if len(secure) > 0 {
		isSecure = secure[0]
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func (h *AuthHandler) setRefreshTokenCookie(w http.ResponseWriter, token string) {
	setRefreshTokenCookie(w, token, h.isCookieSecure())
}

func (h *AuthHandler) clearRefreshTokenCookie(w http.ResponseWriter) {
	clearRefreshTokenCookie(w, h.isCookieSecure())
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

	h.setRefreshTokenCookie(w, rawRefreshToken)

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

	h.setRefreshTokenCookie(w, newRawRefreshToken)

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

	h.clearRefreshTokenCookie(w)

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}