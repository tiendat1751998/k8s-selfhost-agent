package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/runbook"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

// ParsedStep represents a discrete operational procedure step parsed from markdown content.
type ParsedStep struct {
	Index   int    `json:"index"`
	Title   string `json:"title"`
	Command string `json:"command"`
	Content string `json:"content"`
}

// RunbookStepResult represents the outcome of executing a single runbook step.
type RunbookStepResult struct {
	RunbookID      string    `json:"runbook_id"`
	StepIndex      int       `json:"step_index"`
	StepTitle      string    `json:"step_title"`
	Command        string    `json:"command"`
	Status         string    `json:"status"` // "succeeded" | "degraded" | "failed"
	DurationMs     int64     `json:"duration_ms"`
	Output         string    `json:"output"`
	NodesInspected int       `json:"nodes_inspected"`
	ExecutedAt     time.Time `json:"executed_at"`
	Success        bool      `json:"success"`
}

// RunbookExecutionSummary represents the outcome of executing all steps of a runbook playbook.
type RunbookExecutionSummary struct {
	Status         string              `json:"status"` // "succeeded" | "degraded" | "failed"
	RunbookID      string              `json:"runbook_id"`
	Title          string              `json:"title"`
	TotalSteps     int                 `json:"total_steps"`
	CompletedSteps int                 `json:"completed_steps"`
	DurationMs     int64               `json:"duration_ms"`
	Steps          []RunbookStepResult `json:"steps"`
	ExecutedAt     time.Time           `json:"executed_at"`
	Message        string              `json:"message"`
}

// RunbookExecutor executes runbook steps safely against live infrastructure.
type RunbookExecutor interface {
	ExecuteStep(ctx context.Context, rb *runbook.Runbook, stepIndex int) (*RunbookStepResult, error)
	ExecuteAll(ctx context.Context, rb *runbook.Runbook) (*RunbookExecutionSummary, error)
}

// RunbookExecutorOption configures DefaultRunbookExecutor.
type RunbookExecutorOption func(*DefaultRunbookExecutor)

// WithRunbookExecutorDB configures DBTX for compute_hosts queries.
func WithRunbookExecutorDB(db postgres.DBTX) RunbookExecutorOption {
	return func(e *DefaultRunbookExecutor) {
		e.db = db
	}
}

// WithRunbookExecutorClient sets custom http.Client.
func WithRunbookExecutorClient(client *http.Client) RunbookExecutorOption {
	return func(e *DefaultRunbookExecutor) {
		e.httpClient = client
	}
}

// WithRunbookStaticHosts sets static hosts for testing or fallback.
func WithRunbookStaticHosts(hosts []HostTarget) RunbookExecutorOption {
	return func(e *DefaultRunbookExecutor) {
		e.staticHosts = hosts
	}
}

// DefaultRunbookExecutor implements RunbookExecutor.
type DefaultRunbookExecutor struct {
	db          postgres.DBTX
	httpClient  *http.Client
	staticHosts []HostTarget
	dialer      *net.Dialer
	timeout     time.Duration
}

// NewDefaultRunbookExecutor creates a new safe runbook execution engine.
func NewDefaultRunbookExecutor(opts ...RunbookExecutorOption) *DefaultRunbookExecutor {
	e := &DefaultRunbookExecutor{
		timeout: 3 * time.Second,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		dialer: &net.Dialer{Timeout: 1 * time.Second},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// ParseRunbookSteps parses markdown content into discrete structured steps.
func ParseRunbookSteps(content, defaultTitle string) []ParsedStep {
	if strings.TrimSpace(content) == "" {
		return []ParsedStep{
			{Index: 0, Title: defaultTitle, Command: "", Content: ""},
		}
	}

	lines := strings.Split(content, "\n")
	var steps []ParsedStep
	var currentStep *ParsedStep
	stepCounter := 0
	inCodeFence := false
	var fenceBuffer []string

	stepHeaderRegex := regexp.MustCompile(`^(?:#{1,4}\s*(?:Step\s*\d+:?|\d+\.|\w+.*)|\d+\.\s+.*|Step\s+\d+:?.*)$`)

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if strings.HasPrefix(trimmed, "```") {
			if inCodeFence {
				inCodeFence = false
				if currentStep != nil && len(fenceBuffer) > 0 {
					cmdBlock := strings.TrimSpace(strings.Join(fenceBuffer, "\n"))
					if cmdBlock != "" {
						if currentStep.Command != "" {
							currentStep.Command += "\n" + cmdBlock
						} else {
							currentStep.Command = cmdBlock
						}
					}
				}
				fenceBuffer = nil
			} else {
				inCodeFence = true
				fenceBuffer = nil
			}
			continue
		}
		if inCodeFence {
			fenceBuffer = append(fenceBuffer, rawLine)
			continue
		}

		if stepHeaderRegex.MatchString(trimmed) {
			if currentStep != nil {
				steps = append(steps, *currentStep)
			}
			idx := stepCounter
			stepCounter++
			cleanTitle := cleanStepHeader(trimmed)
			if cleanTitle == "" {
				cleanTitle = fmt.Sprintf("Step %d", idx+1)
			}
			currentStep = &ParsedStep{
				Index:   idx,
				Title:   cleanTitle,
				Command: "",
				Content: "",
			}
		} else if strings.HasPrefix(trimmed, "`") && strings.HasSuffix(trimmed, "`") && len(trimmed) > 2 {
			cmd := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			if currentStep != nil {
				if currentStep.Command != "" {
					currentStep.Command += "\n" + cmd
				} else {
					currentStep.Command = cmd
				}
			}
		} else if isCommandLine(trimmed) {
			cleanCmd := strings.TrimPrefix(trimmed, "$ ")
			if currentStep != nil {
				if currentStep.Command != "" {
					currentStep.Command += "\n" + cleanCmd
				} else {
					currentStep.Command = cleanCmd
				}
			}
		} else if trimmed != "" {
			if currentStep != nil {
				if currentStep.Content != "" {
					currentStep.Content += "\n" + trimmed
				} else {
					currentStep.Content = trimmed
				}
			} else {
				currentStep = &ParsedStep{
					Index:   0,
					Title:   defaultTitle,
					Command: "",
					Content: trimmed,
				}
				stepCounter++
			}
		}
	}

	if inCodeFence && len(fenceBuffer) > 0 && currentStep != nil {
		cmdBlock := strings.TrimSpace(strings.Join(fenceBuffer, "\n"))
		if cmdBlock != "" {
			if currentStep.Command != "" {
				currentStep.Command += "\n" + cmdBlock
			} else {
				currentStep.Command = cmdBlock
			}
		}
	}
	if currentStep != nil {
		steps = append(steps, *currentStep)
	}

	if len(steps) == 0 {
		steps = append(steps, ParsedStep{Index: 0, Title: defaultTitle, Content: content})
	}
	return steps
}

func cleanStepHeader(s string) string {
	s = regexp.MustCompile(`^#{1,4}\s*`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`^(?i)Step\s*\d+:?\s*`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`^\d+\.\s*`).ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func isCommandLine(s string) bool {
	prefixes := []string{"kubectl ", "docker ", "crictl ", "patronictl ", "systemctl ", "reap_", "disk_pressure_", "curl ", "$ "}
	lower := strings.ToLower(s)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// ExecuteStep runs a single runbook step safely and returns real diagnostic telemetry.
func (e *DefaultRunbookExecutor) ExecuteStep(ctx context.Context, rb *runbook.Runbook, stepIndex int) (*RunbookStepResult, error) {
	start := time.Now()
	steps := ParseRunbookSteps(rb.Content, rb.Title)
	if stepIndex < 0 || stepIndex >= len(steps) {
		return nil, fmt.Errorf("step index %d out of bounds (total steps: %d)", stepIndex, len(steps))
	}
	step := steps[stepIndex]

	hosts, err := e.resolveHosts(ctx)
	if err != nil {
		hosts = e.fallbackHosts()
	}

	cmdLower := strings.ToLower(step.Command + " " + step.Title + " " + step.Content)

	var output string
	var status string
	nodesInspected := len(hosts)

	switch {
	case strings.Contains(cmdLower, "failover") || strings.Contains(cmdLower, "patronictl") || strings.Contains(cmdLower, "drain"):
		output, status = e.executeFailoverPreflight(ctx, hosts)
	case strings.Contains(cmdLower, "reap_zombies"):
		output, status = e.executeRemediateRPC(ctx, hosts, "reap_zombies")
	case strings.Contains(cmdLower, "disk_pressure_cleanup") || strings.Contains(cmdLower, "clean logs") || strings.Contains(cmdLower, "prune"):
		output, status = e.executeRemediateRPC(ctx, hosts, "disk_pressure_cleanup")
	case strings.Contains(cmdLower, "describe") || strings.Contains(cmdLower, "logs") || strings.Contains(cmdLower, "events"):
		output, status = e.executeLogsAndHealth(ctx, hosts)
	case strings.Contains(cmdLower, "kubectl get") || strings.Contains(cmdLower, "postgres") || strings.Contains(cmdLower, "workload") || strings.Contains(cmdLower, "pod"):
		output, status = e.executeWorkloadDiagnostics(ctx, hosts)
	default:
		output, status = e.executeTelemetryProbe(ctx, hosts)
	}

	elapsed := time.Since(start).Milliseconds()

	return &RunbookStepResult{
		RunbookID:      rb.ID,
		StepIndex:      stepIndex,
		StepTitle:      step.Title,
		Command:        step.Command,
		Status:         status,
		DurationMs:     elapsed,
		Output:         output,
		NodesInspected: nodesInspected,
		ExecutedAt:     time.Now().UTC(),
		Success:        status != "failed",
	}, nil
}

// ExecuteAll executes all steps in a runbook sequentially.
func (e *DefaultRunbookExecutor) ExecuteAll(ctx context.Context, rb *runbook.Runbook) (*RunbookExecutionSummary, error) {
	start := time.Now()
	steps := ParseRunbookSteps(rb.Content, rb.Title)
	results := make([]RunbookStepResult, 0, len(steps))

	overallStatus := "succeeded"

	for i := range steps {
		res, err := e.ExecuteStep(ctx, rb, i)
		if err != nil {
			return nil, err
		}
		if res.Status == "degraded" && overallStatus != "failed" {
			overallStatus = "degraded"
		} else if res.Status == "failed" {
			overallStatus = "failed"
		}
		results = append(results, *res)
	}

	elapsed := time.Since(start).Milliseconds()

	msg := fmt.Sprintf("Playbook executed %d/%d steps in %dms with overall status: %s", len(results), len(steps), elapsed, overallStatus)

	return &RunbookExecutionSummary{
		Status:         overallStatus,
		RunbookID:      rb.ID,
		Title:          rb.Title,
		TotalSteps:     len(steps),
		CompletedSteps: len(results),
		DurationMs:     elapsed,
		Steps:          results,
		ExecutedAt:     time.Now().UTC(),
		Message:        msg,
	}, nil
}
