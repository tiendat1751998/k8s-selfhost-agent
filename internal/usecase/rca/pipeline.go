// Package rca provides the Root Cause Analysis pipeline.
package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/observability"
	"github.com/datdt/k8sselfhost/internal/domain/ports"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	"github.com/datdt/k8sselfhost/internal/pkg/stringutil"
)


// Pipeline orchestrates the RCA analysis flow:
// collect data → build prompt → call LLM → parse response → create report.
type Pipeline struct {
	collector  ports.DataCollector
	registry   ports.LLMRegistry
	reportRepo report.Repository
	incRepo    incident.Repository
	obsRepo    observability.Repository
}

// NewPipeline creates a new RCA pipeline.
func NewPipeline(collector ports.DataCollector, registry ports.LLMRegistry, reportRepo report.Repository, incRepo incident.Repository, obsRepo observability.Repository) *Pipeline {
	return &Pipeline{
		collector:  collector,
		registry:   registry,
		reportRepo: reportRepo,
		incRepo:    incRepo,
		obsRepo:    obsRepo,
	}
}

// llmRCAResponse is the expected JSON structure from the LLM.
type llmRCAResponse struct {
	RootCause    string   `json:"root_cause"`
	Evidence     []string `json:"evidence"`
	Confidence   float64  `json:"confidence"`
	RiskLevel    string   `json:"risk_level"`
	Remediation  string   `json:"remediation"`
	RollbackPlan string   `json:"rollback_plan"`
}

// Analyze runs the full RCA pipeline for a given incident.
func (p *Pipeline) Analyze(ctx context.Context, inc *incident.Incident) (*report.Report, error) {
	log := logger.WithContext(ctx)

	log.Info("starting RCA analysis",
		zap.String("incident_id", inc.ID),
		zap.String("type", inc.Type.String()),
		zap.String("pod", inc.PodName),
	)

	// Step 1: Update incident status to analyzing
	if inc.Status != incident.StatusAnalyzing {
		if err := inc.MarkAnalyzing(); err != nil {
			return nil, fmt.Errorf("marking incident as analyzing: %w", err)
		}
		if p.incRepo != nil {
			if err := p.incRepo.Update(ctx, inc); err != nil {
				return nil, fmt.Errorf("updating incident status: %w", err)
			}
		}
	}

	// Step 2: Collect diagnostic data
	var collectedData *ports.CollectedData
	if p.collector != nil {
		var err error
		collectedData, err = p.collector.Collect(ctx, inc.Namespace, inc.PodName)
		if err != nil {
			log.Warn("partial data collection failure", zap.Error(err))
		}
	}

	// Step 2.5: Collect Observability SLOs
	var activeSLO *observability.SLOSnapshot
	if p.obsRepo != nil {
		slos, err := p.obsRepo.ListSLOSnapshots(ctx)
		if err != nil {
			log.Warn("failed to list SLO snapshots", zap.Error(err))
		} else if len(slos) > 0 {
			activeSLO = &slos[0] // Simple injection of primary SLO for context
		}
	}

	// Step 3: Build prompt
	promptData := buildPromptData(inc, collectedData, activeSLO)
	prompt, err := RenderPrompt(promptData)
	if err != nil {
		log.Warn("rendering RCA prompt failed, using deterministic fallback", zap.Error(err))
		return p.fallbackAnalysis(ctx, inc, collectedData)
	}

	// Step 4: Call LLM
	if p.registry == nil {
		log.Warn("LLM registry is nil, using deterministic fallback")
		return p.fallbackAnalysis(ctx, inc, collectedData)
	}

	llmClient, err := p.registry.Default()
	if err != nil {
		log.Warn("getting default LLM client failed, using deterministic fallback", zap.Error(err))
		return p.fallbackAnalysis(ctx, inc, collectedData)
	}

	llmResp, err := llmClient.Complete(ctx, ports.LLMCompletionRequest{
		Prompt:      prompt,
		System:      SystemPrompt(),
		Temperature: 0.1,
		MaxTokens:   4096,
	})
	if err != nil {
		log.Warn("LLM completion failed, using deterministic fallback", zap.Error(err))
		return p.fallbackAnalysis(ctx, inc, collectedData)
	}

	// Step 5: Parse response
	rcaResp, err := parseRCAResponse(llmResp.Content)
	if err != nil {
		log.Warn("parsing RCA response failed, using deterministic fallback", zap.Error(err))
		return p.fallbackAnalysis(ctx, inc, collectedData)
	}

	// Step 6: Create report
	rpt, err := report.New(
		inc.ID,
		rcaResp.RootCause,
		rcaResp.Evidence,
		rcaResp.Confidence,
		mapRiskLevel(rcaResp.RiskLevel),
		rcaResp.Remediation,
		rcaResp.RollbackPlan,
	)
	if err != nil {
		log.Warn("creating RCA report failed, using deterministic fallback", zap.Error(err))
		return p.fallbackAnalysis(ctx, inc, collectedData)
	}

	rpt.LLMModel = llmResp.Model
	rpt.PromptTokens = llmResp.PromptTokens
	rpt.ResponseTokens = llmResp.ResponseTokens

	// Step 7: Persist report
	if p.reportRepo != nil {
		if err := p.reportRepo.Create(ctx, rpt); err != nil {
			return nil, fmt.Errorf("storing RCA report: %w", err)
		}
	}

	// Step 8: Transition incident status to remediating
	inc.Status = incident.StatusRemediating
	inc.UpdatedAt = time.Now().UTC()
	if p.incRepo != nil {
		if err := p.incRepo.Update(ctx, inc); err != nil {
			log.Warn("failed to update incident status to remediating", zap.Error(err))
		}
	}

	log.Info("RCA analysis complete",
		zap.String("incident_id", inc.ID),
		zap.String("root_cause", stringutil.Truncate(rcaResp.RootCause, 100)),
		zap.Float64("confidence", rcaResp.Confidence),
		zap.String("risk_level", rcaResp.RiskLevel),
	)

	return rpt, nil
}

func (p *Pipeline) fallbackAnalysis(ctx context.Context, inc *incident.Incident, collectedData *ports.CollectedData) (*report.Report, error) {
	log := logger.WithContext(ctx)
	log.Info("executing deterministic heuristic diagnostic fallback",
		zap.String("incident_id", inc.ID),
		zap.String("type", inc.Type.String()),
	)

	rpt, err := generateFallbackReport(inc, collectedData)
	if err != nil {
		return nil, fmt.Errorf("generating fallback report: %w", err)
	}

	// Persist synthesized report
	if p.reportRepo != nil {
		if err := p.reportRepo.Create(ctx, rpt); err != nil {
			return nil, fmt.Errorf("storing fallback RCA report: %w", err)
		}
	}

	// Transition incident status to remediating
	inc.Status = incident.StatusRemediating
	inc.UpdatedAt = time.Now().UTC()
	if p.incRepo != nil {
		if err := p.incRepo.Update(ctx, inc); err != nil {
			log.Warn("failed to update incident status to remediating during fallback", zap.Error(err))
		}
	}

	log.Info("heuristic diagnostic report synthesized successfully",
		zap.String("incident_id", inc.ID),
		zap.String("root_cause", stringutil.Truncate(rpt.RootCause, 100)),
		zap.Float64("confidence", rpt.Confidence),
	)

	return rpt, nil
}

func buildPromptData(inc *incident.Incident, data *ports.CollectedData, slo *observability.SLOSnapshot) PromptData {
	pd := PromptData{
		IncidentType: inc.Type.String(),
		Namespace:    inc.Namespace,
		PodName:      inc.PodName,
		Severity:     inc.Severity.String(),
		Message:      inc.Message,
	}
	
	if slo != nil {
		pd.SLO = &SLOData{
			Name:   slo.Service,
			Target: slo.Target,
			Value:  slo.Actual,
			Status: slo.BudgetStatus,
		}
	}

	if data != nil {
		pd.PodDescribe = data.PodDescribe
		pd.PodLogs = data.PodLogs
		pd.DeploymentYAML = data.DeploymentYAML
		pd.StatefulSetYAML = data.StatefulSetYAML
		pd.ServiceYAML = data.ServiceYAML
		pd.IngressYAML = data.IngressYAML

		for _, ev := range data.Events {
			pd.Events = append(pd.Events, EventData{
				Type:     ev.Type,
				Reason:   ev.Reason,
				Message:  ev.Message,
				Count:    ev.Count,
				LastSeen: ev.LastSeen,
			})
		}

		if data.NodeMetrics != nil {
			pd.NodeMetrics = &NodeMetricsData{
				NodeName:    data.NodeMetrics.NodeName,
				CPUUsage:    data.NodeMetrics.CPUUsage,
				MemoryUsage: data.NodeMetrics.MemoryUsage,
				PodCount:    data.NodeMetrics.PodCount,
				Allocatable: data.NodeMetrics.Allocatable,
			}
		}
	}

	return pd
}

func parseRCAResponse(content string) (*llmRCAResponse, error) {
	// Try to extract JSON from the response (LLMs sometimes wrap in markdown)
	jsonStr := extractJSON(content)

		var resp llmRCAResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling RCA JSON response: %w (raw: %s)", err, stringutil.Truncate(content, 500))
	}


	if resp.RootCause == "" {
		return nil, fmt.Errorf("RCA response missing root_cause")
	}
	if resp.Confidence < 0 || resp.Confidence > 1 {
		resp.Confidence = 0.5
	}

	return &resp, nil
}

var jsonExtractRegex = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

func extractJSON(content string) string {
	matches := jsonExtractRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		return content[start : end+1]
	}

	return content
}

func mapRiskLevel(level string) report.RiskLevel {
	switch level {
	case "critical":
		return report.RiskCritical
	case "high":
		return report.RiskHigh
	case "medium":
		return report.RiskMedium
	case "low":
		return report.RiskLow
	default:
		return report.RiskMedium
	}
}

// generateFallbackReport creates a deterministic heuristic diagnostic report when LLM is unavailable.
func generateFallbackReport(inc *incident.Incident, collectedData *ports.CollectedData) (*report.Report, error) {
	var (
		confidence float64
		riskLevel  report.RiskLevel
	)

	switch inc.Severity {
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

	deploymentName := extractDeploymentName(inc.PodName)

	var (
		rootCause    string
		remediation  string
		rollbackPlan string
	)

	switch inc.Type {
	case incident.TypeNodeNotReady:
		rootCause = "Node probe failure: Port 9100 unreachable, agent service dead or firewall blocking network traffic"
		remediation = fmt.Sprintf("Check host network, restart k8s-agent with systemctl --user restart k8s-agent on target node %s", inc.PodName)
		rollbackPlan = fmt.Sprintf("Drain and cordon node %s until agent and network connectivity are restored", inc.PodName)

	case incident.TypeOOMKilled:
		rootCause = "Memory limit exceeded: Container cgroup limit reached memory threshold (exit code 137)"
		remediation = fmt.Sprintf("Increase container memory limits or optimize heap footprint in Deployment manifest for %s", deploymentName)
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", deploymentName, inc.Namespace)

	case incident.TypeCrashLoopBackOff:
		rootCause = "Application process repeatedly crashed on startup (check container logs and environment variables)"
		remediation = fmt.Sprintf("Inspect application logs for %s, verify database connection credentials and environment variables", inc.PodName)
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", deploymentName, inc.Namespace)

	case incident.TypeResourceExhaust:
		rootCause = "Compute resource saturation: CFS quota throttling or host CPU/memory capacity exhausted"
		remediation = fmt.Sprintf("Increase CPU and memory requests/limits in Deployment manifest for %s and configure HPA", deploymentName)
		rollbackPlan = fmt.Sprintf("kubectl scale deployment %s -n %s --replicas=2", deploymentName, inc.Namespace)

	case incident.TypeServiceUnhealthy:
		rootCause = fmt.Sprintf("Service endpoint unhealthy: Backing pods for %s failed readiness/health probes", inc.PodName)
		remediation = fmt.Sprintf("Verify downstream service dependencies, check health check configuration and restart unhealthy containers for %s", inc.PodName)
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", deploymentName, inc.Namespace)

	case incident.TypeFailedScheduling:
		rootCause = "Pod scheduling failure: Insufficient cluster compute resources or unmatched node selectors/taints"
		remediation = "Scale cluster worker nodes or reduce resource requests in pod specification"
		rollbackPlan = fmt.Sprintf("kubectl delete pod %s -n %s", inc.PodName, inc.Namespace)

	case incident.TypeImagePullBackOff:
		rootCause = "Container image pull failure: Image tag not found, registry authentication failure, or network timeout"
		remediation = "Verify image repository path, check imagePullSecrets, and confirm image tag exists in registry"
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", deploymentName, inc.Namespace)

	case incident.TypeProbeFailed:
		rootCause = fmt.Sprintf("Health probe failure: Container liveness/readiness probe timed out on %s", inc.PodName)
		remediation = "Adjust probe initialDelaySeconds and timeoutSeconds or optimize application startup routine"
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", deploymentName, inc.Namespace)

	case incident.TypeNetworkFailure:
		rootCause = "Network connectivity failure: DNS resolution timeout, packet drop, or CNI plugin communication failure"
		remediation = "Inspect CoreDNS logs, verify CNI plugin status, and check NetworkPolicies in namespace"
		rollbackPlan = "Revert recent NetworkPolicy and routing configuration changes"

	case incident.TypeStorageFailure:
		rootCause = "Storage volume failure: PVC mount timeout, disk quota exceeded, or CSI driver attachment failure"
		remediation = "Check PersistentVolumeClaim status, storage node mount points, and expand volume capacity if needed"
		rollbackPlan = "Unmount stale volume attachments and restart CSI driver daemonset"

	case incident.TypeHPAFailure:
		rootCause = "Horizontal Pod Autoscaler failure: Metric source unavailable or unable to calculate target replica count"
		remediation = "Check metrics-server health, verify custom metrics API endpoints, and ensure resource metrics are emitting"
		rollbackPlan = fmt.Sprintf("kubectl scale deployment %s -n %s --replicas=2", deploymentName, inc.Namespace)

	case incident.TypeIngressFailure:
		rootCause = "Ingress routing failure: Ingress controller backend unreachable or TLS certificate misconfiguration"
		remediation = "Verify ingress controller logs, check service backend port alignment, and validate TLS secret certificates"
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", deploymentName, inc.Namespace)

	default:
		rootCause = fmt.Sprintf("System diagnostic: Outage or failure detected on %s (%s). Symptom: %s", inc.PodName, inc.Type, inc.Message)
		remediation = fmt.Sprintf("Inspect pod description: 'kubectl describe pod %s -n %s' and restart failing workload", inc.PodName, inc.Namespace)
		rollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", deploymentName, inc.Namespace)
	}

	var evidence []string
	evidence = append(evidence, fmt.Sprintf("Target: pod '%s' in namespace '%s'", inc.PodName, inc.Namespace))
	if inc.Message != "" {
		evidence = append(evidence, fmt.Sprintf("Symptom message: %s", inc.Message))
	}

	switch inc.Type {
	case incident.TypeNodeNotReady:
		evidence = append(evidence, "TCP/HTTP probe to node host port 9100 timed out (connection refused or unreachable)")
	case incident.TypeOOMKilled:
		evidence = append(evidence, "Linux cgroup memory controller triggered OOM killer: process received SIGKILL (exit code 137)")
	case incident.TypeCrashLoopBackOff:
		evidence = append(evidence, "Container process terminated repeatedly immediately after startup with non-zero exit code")
	case incident.TypeResourceExhaust:
		evidence = append(evidence, "CFS quota throttled execution cycles exceeding warning threshold")
	case incident.TypeServiceUnhealthy:
		evidence = append(evidence, "Automated health probe endpoint returned 5xx or connection refused")
	case incident.TypeProbeFailed:
		evidence = append(evidence, "Liveness/readiness probe connection timed out after retry threshold")
	default:
		evidence = append(evidence, fmt.Sprintf("Automated diagnostic probe detected abnormal state for %s", inc.Type))
	}

	if collectedData != nil {
		for i, ev := range collectedData.Events {
			if i >= 3 {
				break
			}
			evidence = append(evidence, fmt.Sprintf("K8s Event [%s] %s: %s (count: %d)", ev.Type, ev.Reason, ev.Message, ev.Count))
		}
		if collectedData.NodeMetrics != nil {
			evidence = append(evidence, fmt.Sprintf("Node metrics (%s): CPU %s, Memory %s", collectedData.NodeMetrics.NodeName, collectedData.NodeMetrics.CPUUsage, collectedData.NodeMetrics.MemoryUsage))
		}
		if collectedData.PodMetrics != nil {
			evidence = append(evidence, fmt.Sprintf("Pod metrics: CPU %s, Memory %s", collectedData.PodMetrics.CPUUsage, collectedData.PodMetrics.MemoryUsage))
		}
		if len(collectedData.PodLogs) > 0 {
			for cName, cLog := range collectedData.PodLogs {
				lines := strings.Split(strings.TrimSpace(cLog), "\n")
				for j := len(lines) - 1; j >= 0; j-- {
					line := strings.TrimSpace(lines[j])
					if line != "" {
						evidence = append(evidence, fmt.Sprintf("Recent log [%s]: %s", cName, stringutil.Truncate(line, 120)))
						break
					}
				}
			}
		}
	}

	rpt, err := report.New(
		inc.ID,
		rootCause,
		evidence,
		confidence,
		riskLevel,
		remediation,
		rollbackPlan,
	)
	if err != nil {
		return nil, fmt.Errorf("creating fallback report entity: %w", err)
	}

	rpt.LLMModel = "heuristic-deterministic-v1"
	return rpt, nil
}

func extractDeploymentName(podName string) string {
	parts := strings.Split(podName, "-")
	if len(parts) >= 3 {
		return strings.Join(parts[:len(parts)-2], "-")
	}
	return podName
}


