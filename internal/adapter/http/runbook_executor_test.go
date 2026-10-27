package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/runbook"
)

func TestRunbookExecutor_ParseSteps(t *testing.T) {
	content := `1. Verify target workload status:
` + "```bash" + `
kubectl get pods -A -l app=postgres
` + "```" + `
2. Inspect container events and crash logs:
` + "`kubectl describe pod postgres-0`" + `
3. Execute emergency failover if primary unreachable.
` + "`patronictl failover`"

	steps := ParseRunbookSteps(content, "PostgreSQL Recovery")
	require.Len(t, steps, 3)

	assert.Equal(t, 0, steps[0].Index)
	assert.Contains(t, steps[0].Title, "Verify target workload status")
	assert.Equal(t, "kubectl get pods -A -l app=postgres", steps[0].Command)

	assert.Equal(t, 1, steps[1].Index)
	assert.Contains(t, steps[1].Title, "Inspect container events and crash logs")
	assert.Equal(t, "kubectl describe pod postgres-0", steps[1].Command)

	assert.Equal(t, 2, steps[2].Index)
	assert.Contains(t, steps[2].Title, "Execute emergency failover")
	assert.Equal(t, "patronictl failover", steps[2].Command)
}

func TestRunbookExecutor_ExecuteStep_Diagnostics(t *testing.T) {
	// Mock agent server returning metrics & health
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/metrics":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"hostname": "k8smaster133",
				"cpu":      map[string]float64{"usage_percent": 15.2},
				"memory":   map[string]float64{"usage_percent": 42.5},
				"disks": []map[string]interface{}{
					{"mount_point": "/", "usage_percent": 48.0},
				},
			})
		case "/k8s/health":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"node_name":          "k8smaster133",
				"zombie_count":       0,
				"containerd_healthy": true,
				"docker_healthy":     true,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer agentServer.Close()

	executor := NewDefaultRunbookExecutor(
		WithRunbookStaticHosts([]HostTarget{
			{Name: "k8smaster133", Endpoint: agentServer.URL, Status: "ready"},
		}),
	)

	rb := &runbook.Runbook{
		ID:    "11111111-1111-1111-1111-111111111111",
		Title: "PostgreSQL Verification",
		Content: `1. Check postgres workload:
kubectl get pods -A -l app=postgres`,
	}

	res, err := executor.ExecuteStep(context.Background(), rb, 0)
	require.NoError(t, err)
	assert.Equal(t, rb.ID, res.RunbookID)
	assert.Equal(t, 0, res.StepIndex)
	assert.Contains(t, res.StepTitle, "Check postgres workload")
	assert.Equal(t, "kubectl get pods -A -l app=postgres", res.Command)
	assert.NotEmpty(t, res.Output)
	assert.Equal(t, 1, res.NodesInspected)
	assert.True(t, res.Success)
	assert.Contains(t, []string{"succeeded", "degraded"}, res.Status)
}

func TestRunbookExecutor_ExecuteStep_LogsAndEvents(t *testing.T) {
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/k8s/health":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"node_name":          "k8snode1",
				"zombie_count":       0,
				"containerd_healthy": true,
				"docker_healthy":     true,
			})
		case "/logs":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"timestamp": time.Now().Format(time.RFC3339), "level": "INFO", "message": "Containerd daemon active"},
				{"timestamp": time.Now().Format(time.RFC3339), "level": "WARN", "message": "High memory allocation detected"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer agentServer.Close()

	executor := NewDefaultRunbookExecutor(
		WithRunbookStaticHosts([]HostTarget{
			{Name: "k8snode1", Endpoint: agentServer.URL, Status: "ready"},
		}),
	)

	rb := &runbook.Runbook{
		ID:    "22222222-2222-2222-2222-222222222222",
		Title: "Log Inspection",
		Content: `1. Inspect crash events:
kubectl describe pod postgres-0`,
	}

	res, err := executor.ExecuteStep(context.Background(), rb, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Output)
	assert.Contains(t, res.Output, "k8snode1")
	assert.True(t, res.Success)
}

func TestRunbookExecutor_ExecuteStep_FailoverPreflight(t *testing.T) {
	executor := NewDefaultRunbookExecutor(
		WithRunbookStaticHosts([]HostTarget{
			{Name: "k8smaster133", Endpoint: "http://10.10.10.133:9100", Status: "ready"},
		}),
	)

	rb := &runbook.Runbook{
		ID:    "33333333-3333-3333-3333-333333333333",
		Title: "Emergency Failover",
		Content: `1. Trigger failover:
patronictl failover`,
	}

	res, err := executor.ExecuteStep(context.Background(), rb, 0)
	require.NoError(t, err)
	assert.Contains(t, res.Output, "FAILOVER PRE-FLIGHT")
	assert.Equal(t, "degraded", res.Status)
	assert.True(t, res.Success)
}

func TestRunbookExecutor_ExecuteStep_RemediateRPC(t *testing.T) {
	remediated := false
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/remediate" && r.Method == http.MethodPost {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["action"] == "reap_zombies" {
				remediated = true
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		http.NotFound(w, r)
	}))
	defer agentServer.Close()

	executor := NewDefaultRunbookExecutor(
		WithRunbookStaticHosts([]HostTarget{
			{Name: "k8snode1", Endpoint: agentServer.URL, Status: "ready"},
		}),
	)

	rb := &runbook.Runbook{
		ID:    "44444444-4444-4444-4444-444444444444",
		Title: "Reap Defunct Processes",
		Content: `1. Reap zombie processes:
reap_zombies`,
	}

	res, err := executor.ExecuteStep(context.Background(), rb, 0)
	require.NoError(t, err)
	assert.True(t, remediated)
	assert.Contains(t, res.Output, "reap_zombies")
}

func TestRunbookExecutor_ExecuteAll(t *testing.T) {
	executor := NewDefaultRunbookExecutor(
		WithRunbookStaticHosts([]HostTarget{
			{Name: "k8snode1", Endpoint: "http://127.0.0.1:9100", Status: "ready"},
		}),
	)

	rb := &runbook.Runbook{
		ID:    "55555555-5555-5555-5555-555555555555",
		Title: "Full Playbook",
		Content: `1. Check workload:
kubectl get pods
2. Check logs:
kubectl describe pod
3. Pre-flight failover:
patronictl failover`,
		StepsCount: 3,
	}

	summary, err := executor.ExecuteAll(context.Background(), rb)
	require.NoError(t, err)
	assert.Equal(t, 3, summary.TotalSteps)
	assert.Equal(t, 3, summary.CompletedSteps)
	assert.Len(t, summary.Steps, 3)
	assert.NotEmpty(t, summary.Status)
}
