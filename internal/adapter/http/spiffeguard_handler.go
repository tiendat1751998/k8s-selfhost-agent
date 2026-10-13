package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	domain "github.com/datdt/k8sselfhost/internal/domain/spiffeguard"
)

// SPIFFEGuardHandler handles HTTP requests for SPIFFE/SPIRE workload identity
// rotation, projected service account token monitoring, and clock skew validation.
type SPIFFEGuardHandler struct {
	manager domain.WorkloadIdentityManager
}

// NewSPIFFEGuardHandler creates a new SPIFFEGuardHandler instance.
func NewSPIFFEGuardHandler(manager domain.WorkloadIdentityManager) *SPIFFEGuardHandler {
	return &SPIFFEGuardHandler{
		manager: manager,
	}
}

// Mount registers the routes onto a chi router under /api/v1/identity.
func (h *SPIFFEGuardHandler) Mount(r chi.Router) {
	r.Route("/api/v1/identity", func(sub chi.Router) {
		h.RegisterRoutes(sub)
	})
}

// RegisterRoutes registers endpoints directly on the provided router.
func (h *SPIFFEGuardHandler) RegisterRoutes(r chi.Router) {
	r.Get("/spiffe/svid-status", h.GetSVIDStatus)
	r.Post("/spiffe/renew-jittered", h.RenewSVIDJittered)
	r.Get("/token/projected-status", h.GetProjectedTokenStatus)
	r.Post("/token/validate-clock-skew", h.ValidateClockSkew)
}

// GetSVIDStatus handles GET /api/v1/identity/spiffe/svid-status.
func (h *SPIFFEGuardHandler) GetSVIDStatus(w http.ResponseWriter, r *http.Request) {
	socketPath := strings.TrimSpace(r.URL.Query().Get("socket_path"))

	status, err := h.manager.InspectSVID(r.Context(), socketPath)
	if err != nil {
		if errors.Is(err, domain.ErrSVIDBacklogSaturated) {
			h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
				"SPIRE agent Unix domain socket backlog saturated",
				"ERR_SVID_BACKLOG_SATURATED", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

type renewJitteredRequest struct {
	SocketPath  string `json:"socket_path"`
	MaxJitterMs int64  `json:"max_jitter_ms"`
}

// RenewSVIDJittered handles POST /api/v1/identity/spiffe/renew-jittered.
func (h *SPIFFEGuardHandler) RenewSVIDJittered(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Failed to read request body", "READ_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	var req renewJitteredRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Malformed JSON body", "VALIDATION_ERROR", r.URL.Path)
			return
		}
	}

	maxJitter := time.Duration(req.MaxJitterMs) * time.Millisecond
	status, err := h.manager.RenewSVIDWithJitter(r.Context(), req.SocketPath, maxJitter)
	if err != nil {
		if errors.Is(err, domain.ErrSVIDBacklogSaturated) {
			h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
				"SPIRE agent Unix domain socket backlog saturated",
				"ERR_SVID_BACKLOG_SATURATED", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

// GetProjectedTokenStatus handles GET /api/v1/identity/token/projected-status.
func (h *SPIFFEGuardHandler) GetProjectedTokenStatus(w http.ResponseWriter, r *http.Request) {
	tokenPath := strings.TrimSpace(r.URL.Query().Get("token_path"))
	leewaySecsStr := strings.TrimSpace(r.URL.Query().Get("leeway_seconds"))

	leeway := 60 * time.Second
	if leewaySecsStr != "" {
		if sec, err := strconv.Atoi(leewaySecsStr); err == nil && sec > 0 {
			leeway = time.Duration(sec) * time.Second
		}
	}

	status, err := h.manager.WatchProjectedToken(r.Context(), tokenPath, leeway)
	if err != nil {
		if errors.Is(err, domain.ErrTokenStaleExpired) {
			h.writeProblem(w, http.StatusUnauthorized, "Unauthorized",
				"Projected service account token is stale or expired",
				"ERR_TOKEN_STALE_EXPIRED", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

type validateClockSkewRequest struct {
	TokenIAT        int64  `json:"token_iat"`
	TokenIATISO     string `json:"token_iat_iso"`
	AllowedSkewSecs int64  `json:"allowed_skew_secs"`
}

// ValidateClockSkew handles POST /api/v1/identity/token/validate-clock-skew.
func (h *SPIFFEGuardHandler) ValidateClockSkew(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Missing request body", "READ_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	var req validateClockSkewRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Malformed JSON request body", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	var iat time.Time
	if req.TokenIAT > 0 {
		iat = time.Unix(req.TokenIAT, 0)
	} else if strings.TrimSpace(req.TokenIATISO) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, req.TokenIATISO)
		if parseErr != nil {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid token_iat_iso format", "VALIDATION_ERROR", r.URL.Path)
			return
		}
		iat = parsed
	} else {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Field 'token_iat' or 'token_iat_iso' is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	allowedSkew := time.Duration(req.AllowedSkewSecs) * time.Second
	if allowedSkew <= 0 {
		allowedSkew = 60 * time.Second
	}

	if err := h.manager.ValidateTokenClockSkew(r.Context(), iat, allowedSkew); err != nil {
		if errors.Is(err, domain.ErrTokenClockSkewExceeded) {
			h.writeProblem(w, http.StatusUnauthorized, "Clock Skew Exceeded",
				err.Error(), "ERR_TOKEN_CLOCK_SKEW_EXCEEDED", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "VALID",
		"clock_skew_valid":  true,
		"token_iat":         iat.Format(time.RFC3339),
		"allowed_skew_secs": allowedSkew.Seconds(),
	})
}

func (h *SPIFFEGuardHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
	problem := ProblemDetails{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Code:     code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
