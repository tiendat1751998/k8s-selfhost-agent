package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// ApplyYAML handles POST /api/v1/k8s/{cluster}/apply?ns=default
func (h *K8sResourceHandler) ApplyYAML(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	ns := getNamespaceQuery(r)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body", err)
		return
	}

	// Support JSON wrapper with "yaml" field if provided
	var jsonBody struct {
		YAML string `json:"yaml"`
	}
	if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil && strings.TrimSpace(jsonBody.YAML) != "" {
		bodyBytes = []byte(jsonBody.YAML)
	}

	if len(strings.TrimSpace(string(bodyBytes))) == 0 {
		writeError(w, http.StatusBadRequest, "empty yaml content", nil)
		return
	}

	err = h.repo.ApplyYAML(r.Context(), cluster, ns, bodyBytes)

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "namespace": ns}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "apply", "k8s_manifest", cluster, "yaml_apply", result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for yaml apply", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusBadRequest, "failed to apply yaml", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "applied",
		"cluster": cluster,
	})
}

// ListEvents handles GET /api/v1/k8s/{cluster}/events
func (h *K8sResourceHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	ns := getNamespaceQuery(r)
	filterKind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
	filterName := strings.TrimSpace(r.URL.Query().Get("name"))
	filterType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	limitStr := strings.TrimSpace(r.URL.Query().Get("limit"))

	items, err := h.repo.ListResources(r.Context(), cluster, "events", ns)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list events", err)
		return
	}

	var filtered []map[string]interface{}
	for _, item := range items {
		// Filter by involvedObject.kind if specified
		if filterKind != "" {
			var objKind string
			if inv, ok := item["involvedObject"].(map[string]interface{}); ok {
				if k, ok := inv["kind"].(string); ok {
					objKind = strings.ToLower(k)
				}
			}
			if objKind == "" {
				if k, ok := item["kind"].(string); ok {
					objKind = strings.ToLower(k)
				}
			}
			if objKind != filterKind {
				continue
			}
		}

		// Filter by involvedObject.name or metadata.name if specified
		if filterName != "" {
			var objName string
			if inv, ok := item["involvedObject"].(map[string]interface{}); ok {
				if n, ok := inv["name"].(string); ok {
					objName = n
				}
			}
			metaName := ""
			if meta, ok := item["metadata"].(map[string]interface{}); ok {
				if n, ok := meta["name"].(string); ok {
					metaName = n
				}
			}
			if objName != filterName && metaName != filterName {
				continue
			}
		}

		// Filter by event type (e.g. Normal, Warning)
		if filterType != "" {
			itemType, _ := item["type"].(string)
			if !strings.EqualFold(itemType, filterType) {
				continue
			}
		}

		filtered = append(filtered, item)
	}

	if filtered == nil {
		filtered = []map[string]interface{}{}
	}

	if limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(filtered) {
			filtered = filtered[:limit]
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  filtered,
		"total": len(filtered),
	})
}
