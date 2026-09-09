package rca

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/ports"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/pkg/stringutil"
)

func TestRenderPrompt_Basic(t *testing.T) {
	data := PromptData{
		IncidentType: "CrashLoopBackOff",
		Namespace:    "production",
		PodName:      "api-server-xyz123",
		Severity:     "high",
		Message:      "Container 'api' is in CrashLoopBackOff (restarts: 15)",
		PodDescribe:  "Name: api-server-xyz123\nNamespace: production\nPhase: Running",
		PodLogs: map[string]string{
			"api": "panic: runtime error: nil pointer dereference\ngoroutine 1 [running]:\nmain.main()\n\t/app/main.go:42",
		},
		Events: []EventData{
			{Type: "Warning", Reason: "BackOff", Message: "Back-off restarting failed container", Count: 15, LastSeen: "2024-01-01T12:00:00Z"},
		},
	}

	prompt, err := RenderPrompt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}

	// Verify key data is present in the prompt
	checks := []string{
		"CrashLoopBackOff",
		"production",
		"api-server-xyz123",
		"nil pointer dereference",
		"BackOff",
	}
	for _, check := range checks {
		if !containsStr(prompt, check) {
			t.Errorf("expected prompt to contain '%s'", check)
		}
	}
}

func TestRenderPrompt_WithDeployment(t *testing.T) {
	data := PromptData{
		IncidentType:   "OOMKilled",
		Namespace:      "default",
		PodName:        "worker-abc",
		Severity:       "critical",
		Message:        "Container 'worker' was OOMKilled",
		DeploymentYAML: "Name: worker\nReplicas: 3/3\n",
	}

	prompt, err := RenderPrompt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(prompt, "Deployment Configuration") {
		t.Error("expected prompt to contain deployment section")
	}
}

func TestRenderPrompt_WithNodeMetrics(t *testing.T) {
	data := PromptData{
		IncidentType: "FailedScheduling",
		Namespace:    "default",
		PodName:      "worker-abc",
		Severity:     "high",
		Message:      "Pod cannot be scheduled",
		NodeMetrics: &NodeMetricsData{
			NodeName:    "node-1",
			CPUUsage:    "4",
			MemoryUsage: "16Gi",
			PodCount:    110,
			Allocatable: "CPU: 4, Memory: 16Gi, Pods: 110",
		},
	}

	prompt, err := RenderPrompt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(prompt, "node-1") {
		t.Error("expected prompt to contain node name")
	}
}

func TestSystemPrompt_NotEmpty(t *testing.T) {
	sp := SystemPrompt()
	if sp == "" {
		t.Fatal("expected non-empty system prompt")
	}
	if !containsStr(sp, "Root Cause Analysis") {
		t.Error("expected system prompt to mention RCA")
	}
	if !containsStr(sp, "JSON") {
		t.Error("expected system prompt to mention JSON output format")
	}
}

func TestExtractJSON_CleanJSON(t *testing.T) {
	input := `{"root_cause": "OOM", "confidence": 0.9}`
	result := extractJSON(input)
	if result != input {
		t.Errorf("expected same JSON, got '%s'", result)
	}
}

func TestExtractJSON_WrappedInMarkdown(t *testing.T) {
	input := "Here is the analysis:\n```json\n{\"root_cause\": \"OOM\", \"confidence\": 0.9}\n```"
	result := extractJSON(input)
	if result != `{"root_cause": "OOM", "confidence": 0.9}` {
		t.Errorf("expected extracted JSON, got '%s'", result)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "No JSON here"
	result := extractJSON(input)
	if result != input {
		t.Errorf("expected original string, got '%s'", result)
	}
}

func TestMapRiskLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"critical", "critical"},
		{"high", "high"},
		{"medium", "medium"},
		{"low", "low"},
		{"unknown", "medium"},
		{"", "medium"},
	}

	for _, tc := range tests {
		result := mapRiskLevel(tc.input)
		if string(result) != tc.expected {
			t.Errorf("mapRiskLevel(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	if stringutil.Truncate("short", 10) != "short" {
		t.Error("short string should not be truncated")
	}
	result := stringutil.Truncate("this is a long string", 10)
	if result != "this is a ..." {
		t.Errorf("expected truncated string, got '%s'", result)
	}
}

func TestParseRCAResponse_Valid(t *testing.T) {
	input := `{
		"root_cause": "Memory limit exceeded",
		"evidence": ["OOMKilled event", "Memory at 98%"],
		"confidence": 0.85,
		"risk_level": "high",
		"remediation": "Increase memory limit",
		"rollback_plan": "Revert deployment"
	}`

	resp, err := parseRCAResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RootCause != "Memory limit exceeded" {
		t.Errorf("unexpected root_cause: %s", resp.RootCause)
	}
	if resp.Confidence != 0.85 {
		t.Errorf("unexpected confidence: %f", resp.Confidence)
	}
	if len(resp.Evidence) != 2 {
		t.Errorf("expected 2 evidence items, got %d", len(resp.Evidence))
	}
}

func TestParseRCAResponse_MissingRootCause(t *testing.T) {
	input := `{"confidence": 0.5, "remediation": "fix"}`
	_, err := parseRCAResponse(input)
	if err == nil {
		t.Fatal("expected error for missing root_cause")
	}
}

func TestParseRCAResponse_InvalidJSON(t *testing.T) {
	_, err := parseRCAResponse("not json at all")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseRCAResponse_ConfidenceOutOfRange(t *testing.T) {
	input := `{"root_cause": "test", "confidence": 5.0, "remediation": "fix"}`
	resp, err := parseRCAResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Confidence != 0.5 {
		t.Errorf("expected confidence to be clamped to 0.5, got %f", resp.Confidence)
	}
}

func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}

type mockReportRepo struct {
	reports map[string]*report.Report
	created []*report.Report
}

func newMockReportRepo() *mockReportRepo {
	return &mockReportRepo{
		reports: make(map[string]*report.Report),
	}
}

func (m *mockReportRepo) Create(ctx context.Context, rpt *report.Report) error {
	if rpt.ID == "" {
		rpt.ID = fmt.Sprintf("rpt-%d", time.Now().UnixNano())
	}
	m.reports[rpt.IncidentID] = rpt
	m.created = append(m.created, rpt)
	return nil
}

func (m *mockReportRepo) GetByID(ctx context.Context, id string) (*report.Report, error) {
	for _, r := range m.reports {
		if r.ID == id { return r, nil }
	}
	return nil, fmt.Errorf("report not found")
}

func (m *mockReportRepo) GetByIncidentID(ctx context.Context, incidentID string) (*report.Report, error) {
	if r, ok := m.reports[incidentID]; ok { return r, nil }
	return nil, fmt.Errorf("report not found for incident")
}

func (m *mockReportRepo) List(ctx context.Context, limit, offset int) ([]*report.Report, int64, error) {
	var list []*report.Report
	for _, r := range m.reports { list = append(list, r) }
	return list, int64(len(list)), nil
}

type mockDataCollector struct {
	data *ports.CollectedData
	err  error
}

func (m *mockDataCollector) Collect(ctx context.Context, namespace, podName string) (*ports.CollectedData, error) {
	return m.data, m.err
}

type mockLLMClient struct {
	resp *ports.LLMCompletionResponse
	err  error
}

func (m *mockLLMClient) Complete(ctx context.Context, req ports.LLMCompletionRequest) (*ports.LLMCompletionResponse, error) {
	return m.resp, m.err
}

func (m *mockLLMClient) HealthCheck(ctx context.Context) error {
	return nil
}

type mockLLMRegistry struct {
	client ports.LLMClient
	err    error
}

func (m *mockLLMRegistry) Register(name string, client ports.LLMClient, info ports.LLMProviderInfo) {}
func (m *mockLLMRegistry) Unregister(name string) error                                              { return nil }
func (m *mockLLMRegistry) Get(name string) (ports.LLMClient, error)                                { return m.client, m.err }
func (m *mockLLMRegistry) Default() (ports.LLMClient, error)                                        { return m.client, m.err }
func (m *mockLLMRegistry) DefaultName() string                                                      { return "default" }
func (m *mockLLMRegistry) List() []ports.LLMProviderInfo                                            { return nil }
func (m *mockLLMRegistry) HealthCheckAll(ctx context.Context) map[string]ports.LLMProviderHealthResult {
	return nil
}
func (m *mockLLMRegistry) Count() int { return 1 }

func TestPipeline_Analyze_LLMFailure_GracefulFallback(t *testing.T) {
	collector := &mockDataCollector{
		data: &ports.CollectedData{
			PodLogs: map[string]string{"worker": "fatal: out of memory"},
			Events: []ports.EventSummary{
				{Type: "Warning", Reason: "OOMKilled", Message: "Killed process 1024", Count: 1},
			},
		},
	}
	registry := &mockLLMRegistry{
		client: &mockLLMClient{
			err: errors.New("connection refused: ollama daemon offline"),
		},
	}
	reportRepo := newMockReportRepo()
	incRepo := &mockIncidentRepo{incidents: make(map[string]*incident.Incident)}

	pipeline := NewPipeline(collector, registry, reportRepo, incRepo, nil)

	inc, err := incident.New("fleet-primary", "production", "order-worker-5c79895db-j9m7w", incident.TypeOOMKilled, incident.SeverityCritical, "Container terminated with OOMKilled")
	if err != nil {
		t.Fatalf("unexpected error creating incident: %v", err)
	}
	inc.ID = "inc-test-oom-1"
	incRepo.incidents[inc.ID] = inc

	ctx := context.Background()
	rpt, err := pipeline.Analyze(ctx, inc)
	if err != nil {
		t.Fatalf("expected Analyze to succeed with fallback, got error: %v", err)
	}

	if rpt == nil {
		t.Fatal("expected non-nil report")
	}
	if rpt.LLMModel != "heuristic-deterministic-v1" {
		t.Errorf("expected model 'heuristic-deterministic-v1', got '%s'", rpt.LLMModel)
	}
	if !containsStr(rpt.RootCause, "Memory limit exceeded") {
		t.Errorf("expected root cause to contain 'Memory limit exceeded', got: %s", rpt.RootCause)
	}
	if rpt.Confidence != 0.92 {
		t.Errorf("expected confidence 0.92 for critical incident, got %f", rpt.Confidence)
	}
	if rpt.RiskLevel != report.RiskCritical {
		t.Errorf("expected risk level critical, got %s", rpt.RiskLevel)
	}

	// Verify report is persisted in repository
	savedRpt, err := reportRepo.GetByIncidentID(ctx, inc.ID)
	if err != nil || savedRpt == nil {
		t.Fatalf("expected report in repository, got error: %v", err)
	}

	// Verify incident status updated to remediating
	savedInc, err := incRepo.GetByID(ctx, inc.ID)
	if err != nil || savedInc == nil {
		t.Fatalf("expected incident in repository, got error: %v", err)
	}
	if savedInc.Status != incident.StatusRemediating {
		t.Errorf("expected incident status %s, got %s", incident.StatusRemediating, savedInc.Status)
	}
}

func TestPipeline_Analyze_NodeNotReady_LLMFailure(t *testing.T) {
	collector := &mockDataCollector{}
	registry := &mockLLMRegistry{
		client: &mockLLMClient{err: errors.New("timeout calling LLM endpoint")},
	}
	reportRepo := newMockReportRepo()
	incRepo := &mockIncidentRepo{incidents: make(map[string]*incident.Incident)}

	pipeline := NewPipeline(collector, registry, reportRepo, incRepo, nil)

	inc, err := incident.New("fleet-primary", "infrastructure", "k8sworker3", incident.TypeNodeNotReady, incident.SeverityHigh, "Node agent unreachable")
	if err != nil {
		t.Fatalf("unexpected error creating incident: %v", err)
	}
	inc.ID = "inc-test-node-1"
	incRepo.incidents[inc.ID] = inc

	ctx := context.Background()
	rpt, err := pipeline.Analyze(ctx, inc)
	if err != nil {
		t.Fatalf("expected Analyze to succeed, got %v", err)
	}

	if !containsStr(rpt.RootCause, "Node probe failure: Port 9100 unreachable") {
		t.Errorf("expected root cause for NodeNotReady, got: %s", rpt.RootCause)
	}
	if !containsStr(rpt.Remediation, "systemctl --user restart k8s-agent") {
		t.Errorf("expected remediation to mention restarting k8s-agent, got: %s", rpt.Remediation)
	}
	if rpt.Confidence != 0.90 {
		t.Errorf("expected confidence 0.90 for high severity, got %f", rpt.Confidence)
	}
	if inc.Status != incident.StatusRemediating {
		t.Errorf("expected incident status %s, got %s", incident.StatusRemediating, inc.Status)
	}
}

func TestPipeline_Analyze_NilRegistry_GracefulFallback(t *testing.T) {
	collector := &mockDataCollector{}
	reportRepo := newMockReportRepo()
	incRepo := &mockIncidentRepo{incidents: make(map[string]*incident.Incident)}

	pipeline := NewPipeline(collector, nil, reportRepo, incRepo, nil)

	inc, err := incident.New("fleet-primary", "production", "auth-api-xyz-123", incident.TypeCrashLoopBackOff, incident.SeverityMedium, "CrashLoopBackOff detected")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inc.ID = "inc-test-crash-1"
	incRepo.incidents[inc.ID] = inc

	ctx := context.Background()
	rpt, err := pipeline.Analyze(ctx, inc)
	if err != nil {
		t.Fatalf("expected Analyze to succeed without registry, got %v", err)
	}

	if rpt.Confidence != 0.88 {
		t.Errorf("expected confidence 0.88 for medium severity, got %f", rpt.Confidence)
	}
	if !containsStr(rpt.RootCause, "Application process repeatedly crashed on startup") {
		t.Errorf("expected crashloop root cause, got: %s", rpt.RootCause)
	}
	if inc.Status != incident.StatusRemediating {
		t.Errorf("expected incident status %s, got %s", incident.StatusRemediating, inc.Status)
	}
}

func TestPipeline_Analyze_AlreadyAnalyzing_NoError(t *testing.T) {
	collector := &mockDataCollector{}
	reportRepo := newMockReportRepo()
	incRepo := &mockIncidentRepo{incidents: make(map[string]*incident.Incident)}

	pipeline := NewPipeline(collector, nil, reportRepo, incRepo, nil)

	inc, _ := incident.New("fleet-primary", "production", "api-gateway", incident.TypeServiceUnhealthy, incident.SeverityHigh, "Service Unhealthy")
	inc.ID = "inc-test-analyzing-1"
	inc.Status = incident.StatusAnalyzing
	incRepo.incidents[inc.ID] = inc

	ctx := context.Background()
	rpt, err := pipeline.Analyze(ctx, inc)
	if err != nil {
		t.Fatalf("expected Analyze not to fail when already analyzing, got: %v", err)
	}
	if rpt == nil {
		t.Fatal("expected report")
	}
	if inc.Status != incident.StatusRemediating {
		t.Errorf("expected status %s, got %s", incident.StatusRemediating, inc.Status)
	}
}

func TestPipeline_Analyze_LLMSuccess(t *testing.T) {
	collector := &mockDataCollector{}
	llmJSON := `{"root_cause": "OOM killer invoked", "evidence": ["Exit code 137"], "confidence": 0.95, "risk_level": "critical", "remediation": "Increase limits", "rollback_plan": "undo"}`
	registry := &mockLLMRegistry{
		client: &mockLLMClient{
			resp: &ports.LLMCompletionResponse{
				Content:        llmJSON,
				Model:          "llama3:latest",
				PromptTokens:   350,
				ResponseTokens: 90,
			},
		},
	}
	reportRepo := newMockReportRepo()
	incRepo := &mockIncidentRepo{incidents: make(map[string]*incident.Incident)}

	pipeline := NewPipeline(collector, registry, reportRepo, incRepo, nil)

	inc, _ := incident.New("fleet-primary", "default", "payment-pod", incident.TypeOOMKilled, incident.SeverityCritical, "OOM")
	inc.ID = "inc-test-success-1"
	incRepo.incidents[inc.ID] = inc

	ctx := context.Background()
	rpt, err := pipeline.Analyze(ctx, inc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rpt.LLMModel != "llama3:latest" {
		t.Errorf("expected model llama3:latest, got %s", rpt.LLMModel)
	}
	if rpt.RootCause != "OOM killer invoked" {
		t.Errorf("expected root cause 'OOM killer invoked', got %s", rpt.RootCause)
	}
	if inc.Status != incident.StatusRemediating {
		t.Errorf("expected incident status %s, got %s", incident.StatusRemediating, inc.Status)
	}
}
