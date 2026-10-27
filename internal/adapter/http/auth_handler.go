package http

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/ports"
	"github.com/datdt/k8sselfhost/internal/domain/user"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
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
	Token                  string     `json:"token,omitempty"`
	MFARequired            bool       `json:"mfa_required,omitempty"`
	PartialToken           string     `json:"partial_token,omitempty"`
	RequiresPasswordChange bool       `json:"requires_password_change"`
	User                   *loginUser `json:"user,omitempty"`
}

type loginUser struct {
	ID                     string `json:"id"`
	Role                   string `json:"role"`
	TenantID               string `json:"tenant_id"`
	RequiresPasswordChange bool   `json:"requires_password_change"`
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
	if env := os.Getenv("ENV"); env != "" && !strings.EqualFold(env, "production") {
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
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(middleware.RefreshTokenDuration.Seconds()),
	})
}

func clearRefreshTokenCookie(w http.ResponseWriter, secure ...bool) {
	isSecure := isCookieSecure()
	if len(secure) > 0 {
		isSecure = secure[0]
	}
	for _, name := range []string{"refresh_token", "k8s_refresh_token"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/api/v1/auth",
			HttpOnly: true,
			Secure:   isSecure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
		})
	}
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
		Token:                  accessToken,
		RequiresPasswordChange: res.RequiresPasswordChange,
		User: &loginUser{
			ID:                     res.UserID,
			Role:                   res.Role,
			TenantID:               res.TenantID,
			RequiresPasswordChange: res.RequiresPasswordChange,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// RefreshToken handles POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		cookie, err = r.Cookie("k8s_refresh_token")
	}
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":                    result.AccessToken,
		"requires_password_change": result.RequiresPasswordChange,
	})
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	var bearerToken string
	if authHeader != "" {
		prefix, rest, found := strings.Cut(authHeader, " ")
		if found && strings.EqualFold(prefix, "Bearer") {
			bearerToken = rest
		}
	}
	if bearerToken != "" {
		claims, err := middleware.ValidateTokenClaims(bearerToken)
		expTime := time.Now().Add(middleware.AccessTokenDuration)
		if err == nil && claims != nil && claims.Exp > 0 {
			expTime = time.Unix(claims.Exp, 0)
		}
		middleware.RevokeToken(bearerToken, expTime)
	}

	cookie, err := r.Cookie("refresh_token")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		cookie, err = r.Cookie("k8s_refresh_token")
	}
	if err == nil && cookie.Value != "" && h.usecase != nil {
		_ = h.usecase.RevokeRefreshToken(r.Context(), cookie.Value)
	}

	h.clearRefreshTokenCookie(w)

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}

// Me handles GET /api/v1/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := tenancy.UserIDFromContext(r.Context())
	if userID == "" || userID == "guest" {
		writeError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	if h.userRepo == nil {
		writeError(w, http.StatusInternalServerError, "user repository unavailable", nil)
		return
	}

	usr, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil || usr == nil {
		writeError(w, http.StatusNotFound, "user not found", nil)
		return
	}

	role := usr.TenantRole
	if usr.PlatformRole == "platform_admin" {
		role = "platform_admin"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":                       usr.ID,
		"email":                    usr.Email,
		"role":                     role,
		"tenant_id":                usr.TenantID,
		"mfa_enabled":              usr.MFAEnabled,
		"requires_password_change": usr.RequiresPasswordChange,
	})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (r *changePasswordRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.CurrentPassword) == "" {
		ve.Add("current_password", "current_password is required")
	}
	if len(r.NewPassword) < 10 {
		ve.Add("new_password", "new_password must be at least 10 characters")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// ChangePassword handles POST /api/v1/auth/change-password
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := tenancy.UserIDFromContext(r.Context())
	if userID == "" || userID == "guest" {
		writeError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	req, ok := decodeJSON[changePasswordRequest](w, r)
	if !ok {
		return
	}

	if h.usecase == nil {
		writeError(w, http.StatusInternalServerError, "auth usecase unavailable", nil)
		return
	}

	if err := h.usecase.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "characters") {
			writeError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to change password", err)
		return
	}

	// Revoke current access token if present in request header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		prefix, rest, found := strings.Cut(authHeader, " ")
		if found && strings.EqualFold(prefix, "Bearer") {
			middleware.RevokeToken(rest, time.Now().Add(middleware.AccessTokenDuration))
		}
	}
	h.clearRefreshTokenCookie(w)

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "password changed successfully",
	})
}