package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/dbsafeguard"
)

// DBSafeguardHandler exposes REST API endpoints for database reliability, XID wraparound defense,
// Redis resync storm mitigations, and Kafka poison pill DLQ diversion.
type DBSafeguardHandler struct {
	engine dbsafeguard.DatabaseResilienceEngine
}

// NewDBSafeguardHandler instantiates a new DBSafeguardHandler with injected resilience engine.
func NewDBSafeguardHandler(engine dbsafeguard.DatabaseResilienceEngine) *DBSafeguardHandler {
	return &DBSafeguardHandler{engine: engine}
}

// RegisterRoutes registers the database safeguard endpoints on a chi.Router.
func (h *DBSafeguardHandler) RegisterRoutes(r chi.Router) {
	// Sub-router routes (when mounted under /api/v1/db)
	r.Get("/postgres/xid-health", h.GetPostgresXIDHealth)
	r.Post("/postgres/kill-idle-tx", h.KillIdleTransactions)
	r.Get("/redis/repl-health", h.GetRedisReplHealth)
	r.Post("/kafka/handle-poison-pill", h.HandleKafkaPoisonPill)

	// Direct full paths (when registered on root router)
	r.Get("/api/v1/db/postgres/xid-health", h.GetPostgresXIDHealth)
	r.Post("/api/v1/db/postgres/kill-idle-tx", h.KillIdleTransactions)
	r.Get("/api/v1/db/redis/repl-health", h.GetRedisReplHealth)
	r.Post("/api/v1/db/kafka/handle-poison-pill", h.HandleKafkaPoisonPill)
}

// GetPostgresXIDHealth handles GET /api/v1/db/postgres/xid-health
// Evaluates PostgreSQL transaction ID age and checks distance to 1.2 billion emergency wraparound horizon.
// Returns HTTP 500 with RFC 7807 problem details if XID age > 1.2 billion.
func (h *DBSafeguardHandler) GetPostgresXIDHealth(w http.ResponseWriter, r *http.Request) {
	health, err := h.engine.CheckPostgresXIDHealth(r.Context())
	if err != nil {
		if errors.Is(err, dbsafeguard.ErrDBXIDWraparoundRisk) {
			writeDBSafeguardRFC7807(w, dbsafeguard.ErrDBXIDWraparoundRisk, r.URL.Path)
			return
		}
		h.respondError(w, r, err, "failed to check postgres XID health")
		return
	}

	writeJSON(w, http.StatusOK, health)
}

type killIdleTxRequest struct {
	MaxAgeSeconds int `json:"max_age_seconds"`
}

// KillIdleTransactions handles POST /api/v1/db/postgres/kill-idle-tx
// Terminates PostgreSQL backends lingering in 'idle in transaction' state.
// Active queries are strictly protected by invariant guards.
func (h *DBSafeguardHandler) KillIdleTransactions(w http.ResponseWriter, r *http.Request) {
	maxAgeSeconds := 1800 // Default 30 minutes
	if r.Body != nil && r.ContentLength > 0 {
		var req killIdleTxRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.MaxAgeSeconds > 0 {
			maxAgeSeconds = req.MaxAgeSeconds
		}
	}

	maxAge := time.Duration(maxAgeSeconds) * time.Second
	if err := h.engine.TerminateBlockingTransactions(r.Context(), maxAge); err != nil {
		h.respondError(w, r, err, "failed to terminate idle transactions")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "ok",
		"message":         "blocking idle transactions terminated successfully",
		"max_age_seconds": maxAgeSeconds,
	})
}

// GetRedisReplHealth handles GET /api/v1/db/redis/repl-health
// Audits Redis replication backlog and client-output-buffer-limit saturation.
// Returns HTTP 500 with RFC 7807 problem details if resync storm loop is detected.
func (h *DBSafeguardHandler) GetRedisReplHealth(w http.ResponseWriter, r *http.Request) {
	health, err := h.engine.AuditRedisReplicationBuffer(r.Context())
	if err != nil {
		if errors.Is(err, dbsafeguard.ErrRedisResyncStormLoop) {
			writeDBSafeguardRFC7807(w, dbsafeguard.ErrRedisResyncStormLoop, r.URL.Path)
			return
		}
		h.respondError(w, r, err, "failed to audit redis replication health")
		return
	}

	writeJSON(w, http.StatusOK, health)
}

type handlePoisonPillRequest struct {
	Topic       string `json:"topic"`
	Partition   int32  `json:"partition"`
	Offset      int64  `json:"offset"`
	Payload     string `json:"payload"`
	ErrorReason string `json:"error_reason"`
}

func (req *handlePoisonPillRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(req.Topic) == "" {
		ve.Add("topic", "topic is required")
	}
	if req.Partition < 0 {
		ve.Add("partition", "partition must be non-negative")
	}
	if req.Offset < 0 {
		ve.Add("offset", "offset must be non-negative")
	}
	if strings.TrimSpace(req.ErrorReason) == "" {
		ve.Add("error_reason", "error_reason is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// HandleKafkaPoisonPill handles POST /api/v1/db/kafka/handle-poison-pill
// Intercepts malformed message payload and diverts to DLQ topic, committing offset.
// Returns HTTP 422 with RFC 7807 problem details confirming diversion to DLQ.
func (h *DBSafeguardHandler) HandleKafkaPoisonPill(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[handlePoisonPillRequest](w, r)
	if !ok {
		return
	}

	err := h.engine.HandleKafkaPoisonPill(
		r.Context(),
		req.Topic,
		req.Partition,
		req.Offset,
		[]byte(req.Payload),
		errors.New(req.ErrorReason),
	)
	if err != nil {
		if errors.Is(err, dbsafeguard.ErrKafkaPoisonPillDiverted) {
			writeDBSafeguardRFC7807(w, dbsafeguard.ErrKafkaPoisonPillDiverted, r.URL.Path)
			return
		}
		h.respondError(w, r, err, "failed to divert poison pill message")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "message processed successfully",
	})
}

// respondError sends RFC 7807 problem details if matching domain error, otherwise standard structured error.
func (h *DBSafeguardHandler) respondError(w http.ResponseWriter, r *http.Request, err error, defaultMsg string) {
	var rfcErr *dbsafeguard.RFC7807ProblemDetail
	if errors.As(err, &rfcErr) {
		writeDBSafeguardRFC7807(w, rfcErr, r.URL.Path)
		return
	}
	writeError(w, http.StatusInternalServerError, defaultMsg, err)
}

// writeDBSafeguardRFC7807 serializes RFC 7807 problem details to the HTTP response.
func writeDBSafeguardRFC7807(w http.ResponseWriter, prob *dbsafeguard.RFC7807ProblemDetail, instance string) {
	status := prob.Status
	if status == 0 {
		status = http.StatusInternalServerError
	}

	pCopy := *prob
	if instance != "" {
		pCopy.Instance = instance
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(pCopy)
}
