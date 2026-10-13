package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	domainconsensus "github.com/datdt/k8sselfhost/internal/domain/consensus"
)

// ConsensusHandler exposes REST API endpoints for safe rolling etcd defragmentation,
// monotonic distributed lock fencing tokens, and PgBouncer connection pool hygiene.
type ConsensusHandler struct {
	coordinator domainconsensus.ConsensusCoordinator
}

// NewConsensusHandler constructs a new ConsensusHandler with injected coordinator.
func NewConsensusHandler(coordinator domainconsensus.ConsensusCoordinator) *ConsensusHandler {
	return &ConsensusHandler{coordinator: coordinator}
}

// RegisterRoutes registers consensus endpoints on a chi.Router.
func (h *ConsensusHandler) RegisterRoutes(r chi.Router) {
	// Sub-router routes (when mounted under /api/v1/consensus)
	r.Post("/etcd/safe-defrag", h.SafeDefragCluster)
	r.Post("/locks/acquire-fencing", h.AcquireFencingLock)
	r.Post("/locks/validate-fencing", h.ValidateFencingToken)
	r.Post("/pgbouncer/flush-stale", h.FlushPgBouncerStale)

	// Direct full paths (when registered on root router)
	r.Post("/api/v1/consensus/etcd/safe-defrag", h.SafeDefragCluster)
	r.Post("/api/v1/consensus/locks/acquire-fencing", h.AcquireFencingLock)
	r.Post("/api/v1/consensus/locks/validate-fencing", h.ValidateFencingToken)
	r.Post("/api/v1/consensus/pgbouncer/flush-stale", h.FlushPgBouncerStale)
}

type safeDefragRequest struct {
	Endpoints []string `json:"endpoints"`
}

func (req *safeDefragRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if len(req.Endpoints) == 0 {
		ve.Add("endpoints", "at least one etcd endpoint is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// SafeDefragCluster handles POST /api/v1/consensus/etcd/safe-defrag.
// Executes rolling defragmentation on follower nodes first, enforces leader stepdown,
// and halts if consensus quorum would be violated.
func (h *ConsensusHandler) SafeDefragCluster(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[safeDefragRequest](w, r)
	if !ok {
		return
	}

	if err := h.coordinator.SafeDefragCluster(r.Context(), req.Endpoints); err != nil {
		h.respondConsensusError(w, r, err, "failed to safely defragment etcd cluster")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "etcd rolling defragmentation completed successfully",
	})
}

type acquireFencingRequest struct {
	Key     string `json:"key"`
	OwnerID string `json:"owner_id"`
	TTLMs   int64  `json:"ttl_ms"`
}

func (req *acquireFencingRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(req.Key) == "" {
		ve.Add("key", "lock key is required")
	}
	if strings.TrimSpace(req.OwnerID) == "" {
		ve.Add("owner_id", "owner_id is required")
	}
	if req.TTLMs <= 0 {
		ve.Add("ttl_ms", "ttl_ms must be positive")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// AcquireFencingLock handles POST /api/v1/consensus/locks/acquire-fencing.
// Acquires a distributed lock lease accompanied by a monotonically increasing generation token.
func (h *ConsensusHandler) AcquireFencingLock(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[acquireFencingRequest](w, r)
	if !ok {
		return
	}

	ttl := time.Duration(req.TTLMs) * time.Millisecond
	token, err := h.coordinator.AcquireLockWithFencing(r.Context(), req.Key, req.OwnerID, ttl)
	if err != nil {
		h.respondConsensusError(w, r, err, "failed to acquire fencing lock")
		return
	}

	writeJSON(w, http.StatusOK, token)
}

type validateFencingRequest struct {
	Key        string `json:"key"`
	Generation int64  `json:"generation"`
}

func (req *validateFencingRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(req.Key) == "" {
		ve.Add("key", "lock key is required")
	}
	if req.Generation <= 0 {
		ve.Add("generation", "generation must be positive")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// ValidateFencingToken handles POST /api/v1/consensus/locks/validate-fencing.
// Validates whether the token generation is strictly greater than the committed generation,
// returning HTTP 412 (RFC 7807) if stale.
func (h *ConsensusHandler) ValidateFencingToken(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[validateFencingRequest](w, r)
	if !ok {
		return
	}

	valid, err := h.coordinator.ValidateFencingToken(r.Context(), req.Key, req.Generation)
	if err != nil {
		h.respondConsensusError(w, r, err, "fencing token validation failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":      valid,
		"generation": req.Generation,
	})
}

type flushPgBouncerRequest struct {
	PoolerAddr    string `json:"pooler_addr"`
	NewMasterAddr string `json:"new_master_addr"`
}

func (req *flushPgBouncerRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(req.PoolerAddr) == "" {
		ve.Add("pooler_addr", "pooler_addr is required")
	}
	if strings.TrimSpace(req.NewMasterAddr) == "" {
		ve.Add("new_master_addr", "new_master_addr is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// FlushPgBouncerStale handles POST /api/v1/consensus/pgbouncer/flush-stale.
// Pauses pooler, severs lingering connections to demoted read-only primary, relocates target, and resumes.
func (h *ConsensusHandler) FlushPgBouncerStale(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[flushPgBouncerRequest](w, r)
	if !ok {
		return
	}

	if err := h.coordinator.FlushPgBouncerStaleConnections(r.Context(), req.PoolerAddr, req.NewMasterAddr); err != nil {
		h.respondConsensusError(w, r, err, "failed to flush pgbouncer stale connections")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "ok",
		"message":         "pgbouncer pool flushed and relocated successfully",
		"pooler_addr":     req.PoolerAddr,
		"new_master_addr": req.NewMasterAddr,
	})
}

// respondConsensusError converts RFC 7807 domain errors to application/problem+json.
func (h *ConsensusHandler) respondConsensusError(w http.ResponseWriter, r *http.Request, err error, defaultMsg string) {
	var rfcErr *domainconsensus.RFC7807ProblemDetail
	if errors.As(err, &rfcErr) {
		writeConsensusRFC7807(w, rfcErr, r.URL.Path)
		return
	}
	writeError(w, http.StatusInternalServerError, defaultMsg, err)
}

// writeConsensusRFC7807 writes RFC 7807 Problem Details to the HTTP response.
func writeConsensusRFC7807(w http.ResponseWriter, prob *domainconsensus.RFC7807ProblemDetail, instance string) {
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
