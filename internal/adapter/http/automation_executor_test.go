package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/automation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutomationExecutor_ReapZombies(t *testing.T) {
	remediateCalled := false
	agentMux := http.NewServeMux()
	agentMux.HandleFunc("/k8s/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"node_name":    "k8smater",
			"zombie_count": 0,
		})
	})
	agentMux.HandleFunc("/remediate", func(w http.ResponseWriter, r *http.Request) {
		remediateCalled = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"action":  "reap_zombies",
			"status":  "success",
		})
	})
	agentServer := httptest.NewServer(agentMux)
	defer agentServer.Close()

	executor := adapthttp.NewAutomationRuleExecutor(
		adapthttp.WithExecutorClient(agentServer.Client()),
		adapthttp.WithStaticHosts([]adapthttp.HostTarget{
			{
				Name:     "k8smater",
				Endpoint: agentServer.URL,
			},
		}),
	)

	rule := &automation.Rule{
		ID:         "r-1",
		Name:       "Auto-Reap Zombie Processes",
		ActionType: "reap_zombies",
	}

	exec, err := executor.ExecuteRule(context.Background(), rule)
	require.NoError(t, err)
	assert.Equal(t, "success", exec.Result)
	assert.True(t, remediateCalled)
	assert.Contains(t, exec.TriggerEvent, "k8smater")
	assert.Contains(t, exec.ActionTaken, "reap_zombies")
	assert.Empty(t, exec.ErrorDetail)
}

func TestAutomationExecutor_DiskPressure_BelowThreshold(t *testing.T) {
	agentMux := http.NewServeMux()
	agentMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"hostname": "k8smater",
			"disks": []map[string]interface{}{
				{"mount_point": "/", "usage_percent": 46.3},
			},
		})
	})
	agentServer := httptest.NewServer(agentMux)
	defer agentServer.Close()

	executor := adapthttp.NewAutomationRuleExecutor(
		adapthttp.WithExecutorClient(agentServer.Client()),
		adapthttp.WithStaticHosts([]adapthttp.HostTarget{
			{Name: "k8smater", Endpoint: agentServer.URL},
		}),
	)

	rule := &automation.Rule{
		ID:         "r-2",
		Name:       "Disk Pressure Log Truncation",
		ActionType: "prune_logs_and_containers",
	}

	exec, err := executor.ExecuteRule(context.Background(), rule)
	require.NoError(t, err)
	assert.Equal(t, "success", exec.Result)
	assert.Contains(t, exec.ActionTaken, "Peak disk 46.3% on k8smater (< 85% threshold)")
	assert.Empty(t, exec.ErrorDetail)
}

func TestAutomationExecutor_TelemetryCheck_NotifyAndThrottle(t *testing.T) {
	agentMux := http.NewServeMux()
	agentMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"hostname": "k8smasterdeb",
			"memory":   map[string]interface{}{"usage_percent": 42.5},
			"cpu":      map[string]interface{}{"usage_percent": 11.0},
		})
	})
	agentServer := httptest.NewServer(agentMux)
	defer agentServer.Close()

	executor := adapthttp.NewAutomationRuleExecutor(
		adapthttp.WithExecutorClient(agentServer.Client()),
		adapthttp.WithStaticHosts([]adapthttp.HostTarget{
			{Name: "k8smasterdeb", Endpoint: agentServer.URL},
		}),
	)

	rule := &automation.Rule{
		ID:         "r-3",
		Name:       "High Memory Ingress Worker Scale",
		ActionType: "notify_and_throttle",
	}

	exec, err := executor.ExecuteRule(context.Background(), rule)
	require.NoError(t, err)
	assert.Equal(t, "success", exec.Result)
	assert.Contains(t, exec.ActionTaken, "k8smasterdeb")
	assert.Contains(t, exec.ActionTaken, "42.5%")
	assert.Empty(t, exec.ErrorDetail)
}

func TestAutomationExecutor_ZeroHosts_HonestFailure(t *testing.T) {
	executor := adapthttp.NewAutomationRuleExecutor(
		adapthttp.WithStaticHosts([]adapthttp.HostTarget{}),
	)

	rule := &automation.Rule{
		ID:         "r-4",
		Name:       "Any Rule",
		ActionType: "reap_zombies",
	}

	exec, err := executor.ExecuteRule(context.Background(), rule)
	require.NoError(t, err)
	assert.Equal(t, "failed", exec.Result)
	assert.NotEmpty(t, exec.ErrorDetail)
}
