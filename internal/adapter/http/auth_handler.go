package http

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/ports"
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
	if h.usecase != nil {
		if h.refreshTokenRepo != nil {
			h.usecase.SetRefreshTokenRepo(h.refreshTokenRepo)
			if provider, ok := h.refreshTokenRepo.(interface{ TxManager() ports.TransactionManager }); ok {
				if txMgr := provider.TxManager(); txMgr != nil {
					h.usecase.SetTxManager(txMgr)
				}
			}
		}
		h.usecase.SetTokenIssuer(&httpTokenIssuer{})
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

type httpTokenIssuer struct{}

func (httpTokenIssuer) GenerateAccessToken(userID, role, tenantID string) (string, error) {
	return middleware.GenerateAccessToken(userID, role, tenantID)
}

func (httpTokenIssuer) GenerateRefreshToken() (string, error) {
	return middleware.GenerateRefreshToken()
}

func (httpTokenIssuer) HashToken(token string) string {
	return middleware.HashToken(token)
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

	if h.usecase == nil {
		writeError(w, http.StatusInternalServerError, "auth usecase not configured", nil)
		return
	}

	result, err := h.usecase.RotateRefreshToken(r.Context(), authUsecase.RotateTokenInput{
		RefreshToken: cookie.Value,
		UserAgent:    r.UserAgent(),
		IPAddress:    r.RemoteAddr,
	})
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	h.setRefreshTokenCookie(w, result.RawRefreshToken)

	writeJSON(w, http.StatusOK, map[string]string{
		"token": result.AccessToken,
	})
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil && cookie.Value != "" && h.usecase != nil {
		_ = h.usecase.RevokeRefreshToken(r.Context(), cookie.Value)
	}

	h.clearRefreshTokenCookie(w)

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}