package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

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
