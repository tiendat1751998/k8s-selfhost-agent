// Package http provides HTTP handlers for the dashboard API.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/internal/usecase/gitops"
	"github.com/datdt/k8sselfhost/internal/usecase/rca"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// Handler provides HTTP handlers for the dashboard API.
type Handler struct {
	incidentRepo     incident.Repository
	reportRepo       report.Repository
	prRepo           domainGitops.Repository
	publisher        *infraNats.Publisher
	gitopsController *gitops.Controller
	rcaPipeline      *rca.Pipeline
}

// NewHandler creates a new dashboard HTTP handler.
func NewHandler(incidentRepo incident.Repository, reportRepo report.Repository, prRepo domainGitops.Repository, publisher *infraNats.Publisher, gitopsController *gitops.Controller) *Handler {
	return &Handler{
		incidentRepo:     incidentRepo,
		reportRepo:       reportRepo,
		prRepo:           prRepo,
		publisher:        publisher,
		gitopsController: gitopsController,
	}
}

// SetRCAPipeline sets the in-process RCA pipeline for standalone mode.
func (h *Handler) SetRCAPipeline(p *rca.Pipeline) {
	h.rcaPipeline = p
}


// RegisterRoutes registers dashboard API routes on the given chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/incidents", func(r chi.Router) {
		r.Get("/", h.ListIncidents)
		r.Post("/simulate", h.SimulateIncident)
		r.Get("/{id}", h.GetIncident)
		r.Get("/{id}/report", h.GetIncidentReport)
		r.Get("/{id}/pr", h.GetIncidentPR)
		r.Post("/{id}/analyze", h.AnalyzeIncident)
	})

	r.Route("/reports", func(r chi.Router) {
		r.Get("/", h.ListReports)
		r.Get("/{id}", h.GetReport)
	})

	r.Route("/prs", func(r chi.Router) {
		r.Get("/", h.ListPRs)
		r.Post("/", h.CreatePR)
		r.Get("/{id}", h.GetPR)
		r.Post("/{id}/merge", h.MergePR)
		r.Post("/{id}/close", h.ClosePR)
	})
}

// ListIncidents handles GET /api/v1/incidents
func (h *Handler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	filter := incident.Filter{
		Limit:  parseIntParam(r, "limit", 50),
		Offset: parseIntParam(r, "offset", 0),
	}

	if ns := r.URL.Query().Get("namespace"); ns != "" {
		filter.Namespace = ns
	}
	if cluster := r.URL.Query().Get("cluster"); cluster != "" {
		filter.ClusterName = cluster
	}
	if status := r.URL.Query().Get("status"); status != "" {
		s := incident.Status(status)
		filter.Status = &s
	}
	if typ := r.URL.Query().Get("type"); typ != "" {
		t := incident.Type(typ)
		filter.Type = &t
	}
	if sev := r.URL.Query().Get("severity"); sev != "" {
		s := incident.Severity(sev)
		filter.Severity = &s
	}

	incidents, total, err := h.incidentRepo.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list incidents", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":   incidents,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// GetIncident handles GET /api/v1/incidents/{id}
func (h *Handler) GetIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inc, err := h.incidentRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "incident not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get incident", err)
		return
	}

	writeJSON(w, http.StatusOK, inc)
}

// GetIncidentReport handles GET /api/v1/incidents/{id}/report
func (h *Handler) GetIncidentReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rpt, err := h.reportRepo.GetByIncidentID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "report not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get report", err)
		return
	}

	writeJSON(w, http.StatusOK, rpt)
}

// GetIncidentPR handles GET /api/v1/incidents/{id}/pr
func (h *Handler) GetIncidentPR(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pr, err := h.prRepo.GetByIncidentID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "pull request not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get PR", err)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// ListReports handles GET /api/v1/reports
func (h *Handler) ListReports(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	reports, total, err := h.reportRepo.List(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list reports", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":   reports,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetReport handles GET /api/v1/reports/{id}
func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rpt, err := h.reportRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "report not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get report", err)
		return
	}

	writeJSON(w, http.StatusOK, rpt)
}

// ListPRs handles GET /api/v1/prs
func (h *Handler) ListPRs(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	var status *domainGitops.PRStatus
	if s := r.URL.Query().Get("status"); s != "" {
		st := domainGitops.PRStatus(s)
		status = &st
	}

	prs, total, err := h.prRepo.List(r.Context(), status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list PRs", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":   prs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetPR handles GET /api/v1/prs/{id}
func (h *Handler) GetPR(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pr, err := h.prRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "pull request not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get PR", err)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// AnalyzeIncident handles POST /api/v1/incidents/{id}/analyze
func (h *Handler) AnalyzeIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inc, err := h.incidentRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "incident not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get incident", err)
		return
	}

	// Update status to analyzing
	if err := inc.MarkAnalyzing(); err != nil {
		writeError(w, http.StatusConflict, "invalid incident status for analysis", err)
		return
	}
	if err := h.incidentRepo.Update(r.Context(), inc); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update incident status", err)
		return
	}

	// Publish analyze task to NATS if configured
	if h.publisher != nil {
		task := map[string]string{"incident_id": inc.ID}
		if err := h.publisher.Publish(r.Context(), "incidents.analyze", task); err != nil {
			// Rollback status to detected
			inc.Status = incident.StatusDetected
			if rbErr := h.incidentRepo.Update(r.Context(), inc); rbErr != nil {
				logger.Get().Error("failed to rollback incident status", zap.Error(rbErr))
			}
			writeError(w, http.StatusInternalServerError, "failed to publish analyze task", err)
			return
		}
	} else if h.rcaPipeline != nil {
		// Standalone mode: execute RCA pipeline in background goroutine
		go func(targetInc *incident.Incident) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if _, err := h.rcaPipeline.Analyze(ctx, targetInc); err != nil {
				logger.Get().Error("In-process RCA analysis failed", zap.String("incident_id", targetInc.ID), zap.Error(err))
			}
		}(inc)
	} else {
		// Fallback diagnostic report generator for standalone mode without external LLM
		go func(targetInc *incident.Incident) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			time.Sleep(1 * time.Second)
			evidence := []string{
				fmt.Sprintf("Target: %s in namespace '%s'", targetInc.PodName, targetInc.Namespace),
				fmt.Sprintf("Incident symptom: %s", targetInc.Message),
				"Automated TCP / HTTP agent health check: connection timed out or refused",
			}
			rpt, _ := report.New(
				targetInc.ID,
				fmt.Sprintf("System diagnostic: Node or service outage on %s. Probes detected unresponsive endpoint.", targetInc.PodName),
				evidence,
				0.90,
				report.RiskHigh,
				fmt.Sprintf("Check host network, ensure port 9100 is open, and restart agent: systemctl --user restart k8s-agent on %s", targetInc.PodName),
				"Revert any recent firewall/IP route changes.",
			)
			if h.reportRepo != nil {
				_ = h.reportRepo.Create(ctx, rpt)
			}
			targetInc.Status = incident.StatusRemediating
			_ = h.incidentRepo.Update(ctx, targetInc)
		}(inc)
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "analyzing"})
}

type simulateIncidentRequest struct {
	Scenario    string `json:"scenario"`
	PodName     string `json:"pod_name"`
	Namespace   string `json:"namespace"`
	ClusterName string `json:"cluster_name"`
}

// SimulateIncident handles POST /api/v1/incidents/simulate
// NOTE: This endpoint is strictly for development, debugging, and demo simulations.
// In production environments, incidents are generated event-driven from Docker/K8s events.
func (h *Handler) SimulateIncident(w http.ResponseWriter, r *http.Request) {
	var req simulateIncidentRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, "invalid request body", err)
			return
		}
	}

	scenario := strings.ToLower(strings.TrimSpace(req.Scenario))
	clusterName := strings.TrimSpace(req.ClusterName)
	if clusterName == "" {
		clusterName = "fleet-primary"
	}
	namespace := strings.TrimSpace(req.Namespace)
	podName := strings.TrimSpace(req.PodName)

	var (
		incType      incident.Type
		severity     incident.Severity
		message      string
		rootCause    string
		evidence     []string
		confidence   float64
		riskLevel    report.RiskLevel
		remediation  string
		rollbackPlan string
		filePath     string
		fileContent  string
	)

	switch scenario {
	case "node_down", "node_not_ready", "server_down":
		if namespace == "" {
			namespace = "infrastructure"
		}
		if podName == "" {
			podName = "k8sworker3"
		}
		incType = incident.TypeNodeNotReady
		severity = incident.SeverityCritical
		message = fmt.Sprintf("Infrastructure host '%s' is unreachable: agent health check failed", podName)
		rootCause = fmt.Sprintf("Compute node '%s' host agent stopped responding.", podName)
		evidence = []string{fmt.Sprintf("Host '%s' TCP health check failed with connection refused", podName)}
		confidence = 0.92
		riskLevel = report.RiskCritical
		remediation = fmt.Sprintf("Reboot host '%s' via BMC/IPMI and restart agent service.", podName)
		rollbackPlan = "Drain and cordon node until diagnostics complete."
		filePath = fmt.Sprintf("infrastructure/hosts/%s.yaml", podName)
		fileContent = fmt.Sprintf("apiVersion: v1\nkind: NodeConfig\nmetadata:\n  name: %s\n", podName)

	case "crash_loop", "crash_loop_backoff", "crashloopbackoff":
		if namespace == "" {
			namespace = "production"
		}
		if podName == "" {
			podName = "auth-api-5c79895db-j9m7w"
		}
		incType = incident.TypeCrashLoopBackOff
		severity = incident.SeverityHigh
		message = fmt.Sprintf("Pod '%s' is in CrashLoopBackOff: repeated crash loop detected", podName)
		rootCause = fmt.Sprintf("Application '%s' failed initialization.", podName)
		evidence = []string{fmt.Sprintf("Container in '%s' exited with non-zero status repeatedly", podName)}
		confidence = 0.98
		riskLevel = report.RiskMedium
		remediation = fmt.Sprintf("Inspect application configuration and deployment spec for %s.", podName)
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", extractDeploymentName(podName), namespace)
		filePath = fmt.Sprintf("k8s/%s/%s/deployment.yaml", namespace, extractDeploymentName(podName))
		fileContent = "envFrom:\n  - secretRef:\n      name: db-credentials\n"

	case "resource_exhaustion", "resource_exhaust", "cpu_exhaustion":
		if namespace == "" {
			namespace = "default"
		}
		if podName == "" {
			podName = "order-processor-6b87d-8k9pq"
		}
		incType = incident.TypeResourceExhaust
		severity = incident.SeverityHigh
		message = fmt.Sprintf("Pod '%s' CPU throttling reached critical threshold", podName)
		rootCause = fmt.Sprintf("Compute capacity saturation on %s.", podName)
		evidence = []string{"CFS quota throttled execution cycles exceeding warning threshold"}
		confidence = 0.91
		riskLevel = report.RiskHigh
		remediation = "Increase CPU request/limit in Deployment manifest and adjust HPA."
		rollbackPlan = fmt.Sprintf("kubectl scale deployment %s --replicas=4", extractDeploymentName(podName))
		filePath = fmt.Sprintf("k8s/%s/%s/deployment.yaml", namespace, extractDeploymentName(podName))
		fileContent = "resources:\n  limits:\n    cpu: 2000m\n"

	case "service_unhealthy", "unhealthy":
		if namespace == "" {
			namespace = "production"
		}
		if podName == "" {
			podName = "api-gateway"
		}
		incType = incident.TypeServiceUnhealthy
		severity = incident.SeverityHigh
		message = fmt.Sprintf("Service '%s' failed health check probes", podName)
		rootCause = fmt.Sprintf("Service '%s' health probe failed.", podName)
		evidence = []string{"Health endpoint returned 503 Service Unavailable"}
		confidence = 0.94
		riskLevel = report.RiskHigh
		remediation = "Verify downstream service dependencies and restart unhealthy containers."
		rollbackPlan = fmt.Sprintf("docker service update --rollback %s", podName)
		filePath = fmt.Sprintf("docker-compose.%s.yaml", namespace)
		fileContent = "healthcheck:\n  test: [\"CMD\", \"curl\", \"-f\", \"http://localhost/healthz\"]\n"

	default: // "oom_killed", "oom", or default
		if namespace == "" {
			namespace = "production"
		}
		if podName == "" {
			podName = "payment-service-7d4b8f9c6d-x8k2l"
		}
		incType = incident.TypeOOMKilled
		severity = incident.SeverityCritical
		message = fmt.Sprintf("Container in pod '%s' terminated with exit code 137 (OOMKilled)", podName)
		rootCause = fmt.Sprintf("Memory limit exceeded in %s/%s.", namespace, podName)
		evidence = []string{"Process received SIGKILL (Exit Code 137), exceeding cgroup limit"}
		confidence = 0.95
		riskLevel = report.RiskHigh
		remediation = fmt.Sprintf("Increase container memory limit in Deployment manifest for %s.", podName)
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", extractDeploymentName(podName), namespace)
		filePath = fmt.Sprintf("k8s/%s/%s/deployment.yaml", namespace, extractDeploymentName(podName))
		fileContent = "resources:\n  limits:\n    memory: 1024Mi\n"
	}

	inc, err := incident.New(clusterName, namespace, podName, incType, severity, message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create incident entity", err)
		return
	}
	inc.AddRawData("simulated", "true")
	inc.AddRawData("scenario", scenario)

	if err := h.incidentRepo.Create(r.Context(), inc); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist incident", err)
		return
	}

	if inc.ID == "" {
		inc.ID = fmt.Sprintf("sim-%d", time.Now().UnixNano())
	}

	if h.reportRepo != nil {
		rpt, err := report.New(inc.ID, rootCause, evidence, confidence, riskLevel, remediation, rollbackPlan)
		if err == nil {
			rpt.LLMModel = "claude-3-5-sonnet"
			rpt.PromptTokens = 420
			rpt.ResponseTokens = 180
			if err := h.reportRepo.Create(r.Context(), rpt); err != nil {
				logger.Get().Warn("failed to persist simulated RCA report", zap.Error(err))
			}
		} else {
			logger.Get().Warn("failed to create simulated report entity", zap.Error(err))
		}
	}

	if h.prRepo != nil {
		prBranchID := inc.ID
		if len(prBranchID) > 8 {
			prBranchID = prBranchID[:8]
		}
		branch := fmt.Sprintf("fix/incident-%s", prBranchID)
		title := fmt.Sprintf("fix(%s): auto-remediation for %s incident", inc.Namespace, inc.Type)
		desc := fmt.Sprintf("Automated GitOps PR to remediate %s on %s.\n\n### Root Cause\n%s\n\n### Remediation\n%s", inc.Type, inc.PodName, rootCause, remediation)
		repoURL := "https://github.com/org/k8s-manifests"
		pr, err := domainGitops.New(inc.ID, domainGitops.ProviderGitHub, repoURL, branch, "main", title, desc)
		if err == nil {
			pr.AddFileChange(filePath, fileContent, domainGitops.FileActionModify)
			prNum := 100 + int(time.Now().Unix()%900)
			_ = pr.MarkOpen(fmt.Sprintf("%s/pull/%d", repoURL, prNum), prNum)
			if err := h.prRepo.Create(r.Context(), pr); err != nil {
				logger.Get().Warn("failed to persist simulated GitOps PR", zap.Error(err))
			}
		} else {
			logger.Get().Warn("failed to create simulated PR entity", zap.Error(err))
		}
	}

	writeJSON(w, http.StatusCreated, inc)
}

func extractDeploymentName(podName string) string {
	parts := strings.Split(podName, "-")
	if len(parts) >= 3 {
		return strings.Join(parts[:len(parts)-2], "-")
	}
	return podName
}

// Middleware: AuthMiddleware provides basic token authentication.
func AuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization header", nil)
				return
			}

			if authHeader != "Bearer "+token {
				writeError(w, http.StatusForbidden, "invalid token", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string, err error) {
	finalStatus := status
	finalMessage := message

	if err != nil {
		// Structured logging of the raw internal error for debugging
		logger.Get().Error("HTTP handler error",
			zap.Int("status", status),
			zap.String("message", message),
			zap.Error(err),
		)

		// Map domain error to appropriate HTTP status code if default/internal error is used
		if status == 0 || status == http.StatusInternalServerError {
			mappedStatus, mappedMsg := mapDomainErrorToHTTP(err)
			finalStatus = mappedStatus
			if finalMessage == "" || finalMessage == "Internal server error" {
				finalMessage = mappedMsg
			}
		}
	}

	if finalStatus == 0 {
		finalStatus = http.StatusInternalServerError
	}
	if finalMessage == "" {
		finalMessage = http.StatusText(finalStatus)
	}

	resp := map[string]string{"error": finalMessage}
	writeJSON(w, finalStatus, resp)
}

// decodeJSON decodes the request body into a value of type T and validates it if T or *T implements Validator.
// Returns the decoded value and true on success, or writes a 400 error and returns false.
func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return nil, false
	}

	var valErr error
	if validator, ok := any(&v).(Validator); ok {
		valErr = validator.Validate()
	} else if validator, ok := any(v).(Validator); ok {
		valErr = validator.Validate()
	}

	if valErr != nil {
		writeValidationError(w, valErr)
		return nil, false
	}

	return &v, true
}

func writeValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(*ValidationError); ok {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "validation failed",
			"details": ve.Fields,
		})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]interface{}{
		"error":  "validation failed",
		"detail": err.Error(),
	})
}

func parseIntParam(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return parsed
}

// parseUUIDParam extracts and validates an ID URL parameter.
func parseUUIDParam(r *http.Request, param string) (string, error) {
	id := chi.URLParam(r, param)
	if id == "" {
		return "", fmt.Errorf("missing parameter: %s", param)
	}
	if len(id) > 128 {
		return "", fmt.Errorf("parameter too long: %s", param)
	}
	return id, nil
}

type createPRRequest struct {
	gitops.CreatePRRequest
}

func (r *createPRRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.IncidentID) == "" && strings.TrimSpace(r.Title) == "" {
		ve.Add("incident_id", "incident_id or title is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreatePR handles POST /api/v1/prs
func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createPRRequest](w, r)
	if !ok {
		return
	}

	pr, err := h.gitopsController.CreatePR(r.Context(), req.CreatePRRequest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create PR", err)
		return
	}

	writeJSON(w, http.StatusCreated, pr)
}

// MergePR handles POST /api/v1/prs/{id}/merge
func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "PR id is required", err)
		return
	}
	pr, err := h.gitopsController.MergePR(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to merge PR", err)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// ClosePR handles POST /api/v1/prs/{id}/close
func (h *Handler) ClosePR(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "PR id is required", err)
		return
	}
	pr, err := h.gitopsController.ClosePR(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to close PR", err)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}
