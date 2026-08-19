package http

import (
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/user"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
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

type verifyMFARequest struct {
	PartialToken string `json:"partial_token"`
	Code         string `json:"code"`
	TOTPCode     string `json:"totp_code"`
}

func (r *verifyMFARequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.PartialToken) == "" {
		ve.Add("partial_token", "partial_token is required")
	}
	if strings.TrimSpace(r.Code) == "" && strings.TrimSpace(r.TOTPCode) == "" {
		ve.Add("code", "code or totp_code is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type verifyRecoveryRequest struct {
	PartialToken string `json:"partial_token"`
	RecoveryCode string `json:"recovery_code"`
	Code         string `json:"code"`
}

func (r *verifyRecoveryRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.PartialToken) == "" {
		ve.Add("partial_token", "partial_token is required")
	}
	if strings.TrimSpace(r.RecoveryCode) == "" && strings.TrimSpace(r.Code) == "" {
		ve.Add("code", "recovery_code or code is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type verifySetupRequest struct {
	Code string `json:"code"`
}

func (r *verifySetupRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Code) == "" {
		ve.Add("code", "code is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

type disableTOTPRequest struct {
	Password string `json:"password"`
	Code     string `json:"code"`
}

func (r *disableTOTPRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Password) == "" {
		ve.Add("password", "password is required")
	}
	if strings.TrimSpace(r.Code) == "" {
		ve.Add("code", "code is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
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

// VerifyMFA handles POST /api/v1/auth/verify-mfa — Step 2
func (h *AuthHandler) VerifyMFA(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[verifyMFARequest](w, r)
	if !ok {
		return
	}

	claims, err := middleware.ValidatePartialToken(req.PartialToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired partial token", nil)
		return
	}

	code := req.Code
	if code == "" {
		code = req.TOTPCode
	}

	if err := h.usecase.VerifyTOTP(r.Context(), claims.Sub, code); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	usr, err := h.userRepo.GetByID(r.Context(), claims.Sub)
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
			UserID:    usr.ID,
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

	logger.Get().Info("MFA verified for user", zap.String("user_id", usr.ID))

	writeJSON(w, http.StatusOK, loginResponse{
		Token: accessToken,
		User: &loginUser{
			ID:       usr.ID,
			Role:     role,
			TenantID: usr.TenantID,
		},
	})
}

// VerifyRecoveryCode handles POST /api/v1/auth/recovery/verify — Step 2 with recovery code
func (h *AuthHandler) VerifyRecoveryCode(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[verifyRecoveryRequest](w, r)
	if !ok {
		return
	}

	claims, err := middleware.ValidatePartialToken(req.PartialToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired partial token", nil)
		return
	}

	code := req.RecoveryCode
	if code == "" {
		code = req.Code
	}

	if err := h.usecase.VerifyRecoveryCode(r.Context(), claims.Sub, code); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error(), nil)
		return
	}

	usr, err := h.userRepo.GetByID(r.Context(), claims.Sub)
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
			UserID:    usr.ID,
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

	logger.Get().Info("Recovery code verified for user", zap.String("user_id", usr.ID))

	writeJSON(w, http.StatusOK, loginResponse{
		Token: accessToken,
		User: &loginUser{
			ID:       usr.ID,
			Role:     role,
			TenantID: usr.TenantID,
		},
	})
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
	_ = h.refreshTokenRepo.Revoke(r.Context(), rt.ID)

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
			_ = h.refreshTokenRepo.Revoke(r.Context(), rt.ID)
		}
	}

	clearRefreshTokenCookie(w)

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}

// SetupTOTP handles POST /api/v1/auth/totp/setup (authenticated)
func (h *AuthHandler) SetupTOTP(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" || userID == "guest" {
		writeError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	res, err := h.usecase.SetupTOTP(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// VerifyTOTPSetup handles POST /api/v1/auth/totp/verify-setup (authenticated)
func (h *AuthHandler) VerifyTOTPSetup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" || userID == "guest" {
		writeError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	req, ok := decodeJSON[verifySetupRequest](w, r)
	if !ok {
		return
	}

	codes, err := h.usecase.VerifyAndEnableTOTP(r.Context(), userID, req.Code)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"recovery_codes": codes,
		"message":        "MFA enabled successfully",
	})
}

// DisableTOTP handles POST /api/v1/auth/totp/disable (authenticated)
func (h *AuthHandler) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" || userID == "guest" {
		writeError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	req, ok := decodeJSON[disableTOTPRequest](w, r)
	if !ok {
		return
	}

	if err := h.usecase.DisableTOTP(r.Context(), userID, req.Password, req.Code); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "MFA disabled successfully",
	})
}

// TOTPStatus handles GET /api/v1/auth/totp/status (authenticated)
func (h *AuthHandler) TOTPStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" || userID == "guest" {
		writeError(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	status, err := h.usecase.GetMFAStatus(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	writeJSON(w, http.StatusOK, status)
}
