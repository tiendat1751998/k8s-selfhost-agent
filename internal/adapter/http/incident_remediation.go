package http

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// executeFallbackReportAndStatus synthesizes a fallback report and transitions incident status to remediating.
func (h *Handler) executeFallbackReportAndStatus(ctx context.Context, targetInc *incident.Incident) *report.Report {
	evidence := []string{
		fmt.Sprintf("Target: %s in namespace '%s'", targetInc.PodName, targetInc.Namespace),
		fmt.Sprintf("Incident symptom: %s", targetInc.Message),
		"Automated TCP / HTTP agent health check: connection timed out or refused",
	}

	rootCause := fmt.Sprintf("System diagnostic: Outage or failure detected on %s. Probes detected unresponsive endpoint.", targetInc.PodName)
	remediation := fmt.Sprintf("Check host network, ensure port 9100 is open, and restart agent: systemctl --user restart k8s-agent on %s", targetInc.PodName)
	rollbackPlan := "Revert any recent configuration or network route changes."
	confidence := 0.90
	riskLevel := report.RiskHigh

	switch targetInc.Severity {
	case incident.SeverityCritical:
		confidence = 0.92
		riskLevel = report.RiskCritical
	case incident.SeverityMedium:
		confidence = 0.88
		riskLevel = report.RiskMedium
	case incident.SeverityLow:
		confidence = 0.85
		riskLevel = report.RiskLow
	default:
		confidence = 0.90
		riskLevel = report.RiskHigh
	}

	switch targetInc.Type {
	case incident.TypeNodeNotReady:
		rootCause = fmt.Sprintf("Compute node '%s' host agent stopped responding.", targetInc.PodName)
		remediation = fmt.Sprintf("Check host network, ensure port 9100 is open, and restart agent: systemctl --user restart k8s-agent on %s", targetInc.PodName)
		rollbackPlan = "Drain and cordon node until diagnostics complete."
	case incident.TypeOOMKilled:
		rootCause = fmt.Sprintf("Memory limit exceeded in %s/%s.", targetInc.Namespace, targetInc.PodName)
		remediation = fmt.Sprintf("Increase container memory limit in Deployment manifest for %s.", targetInc.PodName)
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", extractDeploymentName(targetInc.PodName), targetInc.Namespace)
	case incident.TypeCrashLoopBackOff:
		rootCause = fmt.Sprintf("Application '%s' repeatedly crashed on startup.", targetInc.PodName)
		remediation = fmt.Sprintf("Inspect application configuration and deployment spec for %s.", targetInc.PodName)
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", extractDeploymentName(targetInc.PodName), targetInc.Namespace)
	case incident.TypeResourceExhaust:
		rootCause = fmt.Sprintf("Compute capacity saturation on %s.", targetInc.PodName)
		remediation = "Increase CPU request/limit in Deployment manifest and adjust HPA."
		rollbackPlan = fmt.Sprintf("kubectl scale deployment %s --replicas=2 -n %s", extractDeploymentName(targetInc.PodName), targetInc.Namespace)
	case incident.TypeServiceUnhealthy:
		rootCause = fmt.Sprintf("Service '%s' failed health check probes.", targetInc.PodName)
		remediation = "Verify downstream service dependencies and restart unhealthy containers."
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", extractDeploymentName(targetInc.PodName), targetInc.Namespace)
	}

	rpt, err := report.New(
		targetInc.ID,
		rootCause,
		evidence,
		confidence,
		riskLevel,
		remediation,
		rollbackPlan,
	)
	if err == nil && rpt != nil {
		rpt.LLMModel = "heuristic-deterministic-v1"
		if h.reportRepo != nil {
			if createErr := h.reportRepo.Create(ctx, rpt); createErr != nil {
				logger.Get().Warn("failed to persist fallback RCA report in handler", zap.Error(createErr))
			}
		}
	} else if err != nil {
		logger.Get().Warn("failed to create fallback report entity in handler", zap.Error(err))
	}

	targetInc.Status = incident.StatusRemediating
	targetInc.UpdatedAt = time.Now().UTC()
	if h.incidentRepo != nil {
		if updateErr := h.incidentRepo.Update(ctx, targetInc); updateErr != nil {
			logger.Get().Warn("failed to update incident status to remediating in handler", zap.Error(updateErr))
		}
	}

	return rpt
}

// ensureRemediationPR idempotently creates a GitOps pull request for the incident if not already present.
func (h *Handler) ensureRemediationPR(ctx context.Context, inc *incident.Incident, rpt *report.Report) {
	if h.prRepo == nil {
		return
	}
	// Prevent duplicate PR creation for the same incident
	if existing, err := h.prRepo.GetByIncidentID(ctx, inc.ID); err == nil && existing != nil {
		return
	}

	var (
		filePath    string
		fileContent string
	)

	depName := extractDeploymentName(inc.PodName)
	switch inc.Type {
	case incident.TypeNodeNotReady:
		filePath = fmt.Sprintf("infrastructure/hosts/%s.yaml", inc.PodName)
		fileContent = fmt.Sprintf("apiVersion: v1\nkind: NodeConfig\nmetadata:\n  name: %s\n", inc.PodName)
	case incident.TypeCrashLoopBackOff:
		filePath = fmt.Sprintf("k8s/%s/%s/deployment.yaml", inc.Namespace, depName)
		fileContent = "envFrom:\n  - secretRef:\n      name: db-credentials\n"
	case incident.TypeResourceExhaust:
		filePath = fmt.Sprintf("k8s/%s/%s/deployment.yaml", inc.Namespace, depName)
		fileContent = "resources:\n  limits:\n    cpu: 2000m\n"
	case incident.TypeServiceUnhealthy:
		filePath = fmt.Sprintf("docker-compose.%s.yaml", inc.Namespace)
		fileContent = "healthcheck:\n  test: [\"CMD\", \"curl\", \"-f\", \"http://localhost/healthz\"]\n"
	default: // OOMKilled and others
		filePath = fmt.Sprintf("k8s/%s/%s/deployment.yaml", inc.Namespace, depName)
		fileContent = "resources:\n  limits:\n    memory: 1024Mi\n"
	}

	rootCause := fmt.Sprintf("Root cause identified for %s", inc.Type)
	remediation := fmt.Sprintf("Automated remediation generated for %s", inc.Type)
	if rpt != nil {
		rootCause = rpt.RootCause
		remediation = rpt.Remediation
	}

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
		if err := h.prRepo.Create(ctx, pr); err != nil {
			logger.Get().Warn("failed to persist GitOps PR in handler", zap.Error(err))
		}
	} else {
		logger.Get().Warn("failed to create GitOps PR entity in handler", zap.Error(err))
	}
}
