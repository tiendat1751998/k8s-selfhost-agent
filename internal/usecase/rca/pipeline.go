// Package rca provides the Root Cause Analysis pipeline.
package rca

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

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
	if err := inc.MarkAnalyzing(); err != nil {
		return nil, fmt.Errorf("marking incident as analyzing: %w", err)
	}
	if err := p.incRepo.Update(ctx, inc); err != nil {
		return nil, fmt.Errorf("updating incident status: %w", err)
	}

	// Step 2: Collect diagnostic data
	collectedData, err := p.collector.Collect(ctx, inc.Namespace, inc.PodName)
	if err != nil {
		log.Warn("partial data collection failure", zap.Error(err))
	}

	// Step 2.5: Collect Observability SLOs
	slos, _ := p.obsRepo.ListSLOSnapshots(ctx)
	var activeSLO *observability.SLOSnapshot
	if len(slos) > 0 {
		activeSLO = &slos[0] // Simple injection of primary SLO for context
	}

	// Step 3: Build prompt
	promptData := buildPromptData(inc, collectedData, activeSLO)
	prompt, err := RenderPrompt(promptData)
	if err != nil {
		return nil, fmt.Errorf("rendering RCA prompt: %w", err)
	}

	// Step 4: Call LLM
	llmClient, err := p.registry.Default()
	if err != nil {
		if markErr := inc.MarkFailed(); markErr == nil {
			if updateErr := p.incRepo.Update(ctx, inc); updateErr != nil {
				log.Error("failed to update incident fallback state", zap.Error(updateErr))
			}
		} else {
			log.Error("failed to mark incident failed", zap.Error(markErr))
		}
		return nil, fmt.Errorf("getting default LLM client: %w", err)
	}

	llmResp, err := llmClient.Complete(ctx, ports.LLMCompletionRequest{
		Prompt:      prompt,
		System:      SystemPrompt(),
		Temperature: 0.1,
		MaxTokens:   4096,
	})
	if err != nil {
		if markErr := inc.MarkFailed(); markErr == nil {
			if updateErr := p.incRepo.Update(ctx, inc); updateErr != nil {
				log.Error("failed to update incident fallback state", zap.Error(updateErr))
			}
		} else {
			log.Error("failed to mark incident failed", zap.Error(markErr))
		}
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Step 5: Parse response
	rcaResp, err := parseRCAResponse(llmResp.Content)
	if err != nil {
		if markErr := inc.MarkFailed(); markErr == nil {
			if updateErr := p.incRepo.Update(ctx, inc); updateErr != nil {
				log.Error("failed to update incident fallback state", zap.Error(updateErr))
			}
		} else {
			log.Error("failed to mark incident failed", zap.Error(markErr))
		}
		return nil, fmt.Errorf("parsing RCA response: %w", err)
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
		return nil, fmt.Errorf("creating RCA report: %w", err)
	}

	rpt.LLMModel = llmResp.Model
	rpt.PromptTokens = llmResp.PromptTokens
	rpt.ResponseTokens = llmResp.ResponseTokens

	// Step 7: Persist report
	if err := p.reportRepo.Create(ctx, rpt); err != nil {
		return nil, fmt.Errorf("storing RCA report: %w", err)
	}

		log.Info("RCA analysis complete",
		zap.String("incident_id", inc.ID),
		zap.String("root_cause", stringutil.Truncate(rcaResp.RootCause, 100)),
		zap.Float64("confidence", rcaResp.Confidence),
		zap.String("risk_level", rcaResp.RiskLevel),
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

// truncate removed, replaced by stringutil.Truncate

