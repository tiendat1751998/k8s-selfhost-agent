// Package rca provides the Root Cause Analysis engine.
package rca

import (
	"fmt"
	"strings"
	"text/template"
)

// systemPrompt defines the LLM's role for RCA analysis.
const systemPrompt = `You are an expert Kubernetes Site Reliability Engineer (SRE) performing Root Cause Analysis on Kubernetes incidents.

Your task is to analyze the provided diagnostic data and produce a structured RCA report.

RULES:
- Be precise and specific about the root cause
- Base your analysis ONLY on the provided evidence
- DO NOT assume, guess, or extrapolate details not present in the logs, events, metrics, or YAML configurations.
- If certain telemetry is missing, displays "error:", or is empty, state that the data is unavailable instead of speculating.
- Assign a confidence score between 0.0 and 1.0 reflecting the certainty of evidence
- Provide actionable remediation steps
- Include a rollback plan in case remediation fails
- Use YAML format for any Kubernetes manifest patches

OUTPUT FORMAT (JSON):
{
  "root_cause": "Detailed description of the root cause",
  "evidence": ["evidence item 1", "evidence item 2"],
  "confidence": 0.85,
  "risk_level": "critical|high|medium|low",
  "remediation": "Step-by-step remediation plan with YAML patches if applicable",
  "rollback_plan": "Steps to rollback if remediation fails"
}`

// rcaPromptTemplate is the template for generating RCA prompts.
const rcaPromptTemplate = `## Kubernetes Incident RCA Request

### Incident Details
- **Type**: {{.IncidentType}}
- **Namespace**: {{.Namespace}}
- **Pod**: {{.PodName}}
- **Severity**: {{.Severity}}
- **Message**: {{.Message}}

### Pod Description
{{.PodDescribe}}

### Container Logs
{{range $container, $logs := .PodLogs}}
#### Container: {{$container}}
` + "```" + `
{{$logs}}
` + "```" + `
{{end}}

### Kubernetes Events
{{range .Events}}
- [{{.Type}}] {{.Reason}}: {{.Message}} (count: {{.Count}}, last: {{.LastSeen}})
{{end}}

{{if .DeploymentYAML}}
### Deployment Configuration
` + "```" + `
{{.DeploymentYAML}}
` + "```" + `
{{end}}

{{if .StatefulSetYAML}}
### StatefulSet Configuration
` + "```" + `
{{.StatefulSetYAML}}
` + "```" + `
{{end}}

{{if .ServiceYAML}}
### Service Configuration
` + "```" + `
{{.ServiceYAML}}
` + "```" + `
{{end}}

{{if .IngressYAML}}
### Ingress Configuration
` + "```" + `
{{.IngressYAML}}
` + "```" + `
{{end}}

{{if .NodeMetrics}}
### Node Metrics
- Node: {{.NodeMetrics.NodeName}}
- CPU: {{.NodeMetrics.CPUUsage}}
- Memory: {{.NodeMetrics.MemoryUsage}}
- Pod Count: {{.NodeMetrics.PodCount}}
- Allocatable: {{.NodeMetrics.Allocatable}}
{{end}}

{{if .SLO}}
### Service Level Objectives
- SLO Name: {{.SLO.Name}}
- Target: {{.SLO.Target}}%
- Current Value: {{.SLO.Value}}%
- Status: {{.SLO.Status}}
{{end}}


Please analyze this incident and provide your RCA report in the JSON format specified.`

// PromptData holds the data used to render RCA prompts.
type PromptData struct {
	IncidentType   string
	Namespace      string
	PodName        string
	Severity       string
	Message        string
	PodDescribe    string
	PodLogs        map[string]string
	Events         []EventData
	DeploymentYAML string
	StatefulSetYAML string
	ServiceYAML    string
	IngressYAML    string
	NodeMetrics    *NodeMetricsData
	SLO            *SLOData
}

// EventData holds event info for template rendering.
type EventData struct {
	Type      string
	Reason    string
	Message   string
	Count     int32
	LastSeen  string
}

// NodeMetricsData holds node metrics for template rendering.
type NodeMetricsData struct {
	NodeName    string
	CPUUsage    string
	MemoryUsage string
	PodCount    int
	Allocatable string
}

// SLOData holds SLO context for RCA.
type SLOData struct {
	Name   string
	Target float64
	Value  float64
	Status string
}

// SystemPrompt returns the system prompt for the LLM.
func SystemPrompt() string {
	return systemPrompt
}

// RenderPrompt renders the RCA prompt template with the given data.
func RenderPrompt(data PromptData) (string, error) {
	tmpl, err := template.New("rca").Parse(rcaPromptTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing RCA prompt template: %w", err)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("executing RCA prompt template: %w", err)
	}

	return sb.String(), nil
}
