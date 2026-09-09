package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// PasswordVerifier defines the interface for verifying a user's password.
type PasswordVerifier interface {
	VerifyPassword(ctx context.Context, userID, password string) error
}

// TokenViewRequest defines the request body for viewing swarm join tokens.
type TokenViewRequest struct {
	Password string `json:"password"`
}

// Validate validates the token view request.
func (r *TokenViewRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Password) == "" {
		ve.Add("password", "password is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// SwarmTokensResponse defines the protected response containing swarm join tokens.
type SwarmTokensResponse struct {
	WorkerToken        string `json:"worker_token"`
	ManagerToken       string `json:"manager_token"`
	ManagerAddr        string `json:"manager_addr"`
	WorkerTokenMasked  string `json:"worker_token_masked"`
	ManagerTokenMasked string `json:"manager_token_masked"`
	ExpiresInSeconds   int    `json:"expires_in_seconds"`
}

// TokenViewLimiter limits token views per user within a rolling time window.
type TokenViewLimiter struct {
	mu     sync.Mutex
	views  map[string][]time.Time
	max    int
	window time.Duration
}

const (
	defaultMaxTokenViews   = 3
	defaultTokenViewWindow = 15 * time.Minute
)

// NewTokenViewLimiter creates a new thread-safe token view rate limiter.
func NewTokenViewLimiter(max int, window time.Duration) *TokenViewLimiter {
	return &TokenViewLimiter{
		views:  make(map[string][]time.Time),
		max:    max,
		window: window,
	}
}

// Allow checks if a user is permitted to view tokens, recording the timestamp if permitted.
func (l *TokenViewLimiter) Allow(userID string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	var valid []time.Time
	for _, t := range l.views[userID] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.max {
		l.views[userID] = valid
		oldest := valid[0]
		retryAfter := oldest.Add(l.window).Sub(now)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return false, retryAfter
	}

	valid = append(valid, now)
	l.views[userID] = valid
	return true, 0
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "SWMTKN-1-***..." + token
	}
	last8 := token[len(token)-8:]
	return "SWMTKN-1-***..." + last8
}

// GetSwarmInfo handles GET /api/v1/docker/swarm
func (h *DockerHandler) GetSwarmInfo(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}
	info, err := h.repo.GetSwarmInfo(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get swarm info", err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// GetSwarmTokens handles POST /api/v1/docker/swarm/tokens (platform_admin only with re-authentication)
func (h *DockerHandler) GetSwarmTokens(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "docker service unavailable", nil)
		return
	}

	// 1. Role verification - platform_admin only
	role := tenancy.UserRoleFromContext(r.Context())
	if role != "platform_admin" {
		writeError(w, http.StatusForbidden, "platform_admin role required to view swarm join tokens", nil)
		return
	}

	userID := tenancy.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "unknown"
	}

	// 2. Decode & validate request body (password confirmation required)
	req, ok := decodeJSON[TokenViewRequest](w, r)
	if !ok {
		return
	}

	// 3. Rate limiting - Max 3 token views per 15 minutes per user
	if h.tokenLimiter != nil {
		allowed, retryAfter := h.tokenLimiter.Allow(userID)
		if !allowed {
			minutes := int(retryAfter.Minutes()) + 1
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, fmt.Sprintf("token viewing rate limited. Try again in %d minutes", minutes), nil)
			return
		}
	}

	// 4. Re-authentication verification
	if h.verifier != nil {
		if err := h.verifier.VerifyPassword(r.Context(), userID, req.Password); err != nil {
			writeError(w, http.StatusForbidden, "invalid password", nil)
			return
		}
	}

	// 5. Security audit logging
	logInstance := h.logger
	if logInstance == nil {
		logInstance = logger.Get()
	}
	logInstance.Warn("SECURITY: Swarm join tokens accessed",
		zap.String("user_id", userID),
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("user_agent", r.UserAgent()),
	)

	// 6. Fetch join tokens
	tokens, err := h.repo.GetSwarmJoinTokens(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get swarm join tokens", err)
		return
	}

	// 7. Return masked and full tokens response
	resp := SwarmTokensResponse{
		WorkerToken:        tokens.WorkerToken,
		ManagerToken:       tokens.ManagerToken,
		ManagerAddr:        tokens.ManagerAddr,
		WorkerTokenMasked:  maskToken(tokens.WorkerToken),
		ManagerTokenMasked: maskToken(tokens.ManagerToken),
		ExpiresInSeconds:   60,
	}

	writeJSON(w, http.StatusOK, resp)
}
