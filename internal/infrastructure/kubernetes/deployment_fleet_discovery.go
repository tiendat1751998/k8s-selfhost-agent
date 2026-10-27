package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/deployment"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

type daemonMetadata struct {
	canonicalName string
	team          string
	port          int
	netType       string
}

var fleetDaemons = []daemonMetadata{
	{canonicalName: "k8s-agent", team: "observability", port: 9100, netType: "HostPort"},
	{canonicalName: "containerd", team: "infra", port: 0, netType: "UnixSocket"},
	{canonicalName: "dockerd", team: "infra", port: 2375, netType: "HostPort"},
	{canonicalName: "kubelet", team: "platform", port: 10250, netType: "HostPort"},
	{canonicalName: "kube-scheduler", team: "platform", port: 10259, netType: "HostPort"},
	{canonicalName: "postgres", team: "data", port: 5432, netType: "HostPort"},
	{canonicalName: "redis-server", team: "data", port: 6379, netType: "HostPort"},
	{canonicalName: "proxysql", team: "data", port: 6032, netType: "HostPort"},
}

type fleetProcessMetric struct {
	PID         int     `json:"pid"`
	Name        string  `json:"name"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes int64   `json:"memory_bytes"`
	State       string  `json:"state"`
}

type fleetMetricsPayload struct {
	Hostname         string               `json:"hostname"`
	DetectedServices []string             `json:"detected_services"`
	TopProcesses     []fleetProcessMetric `json:"top_processes"`
}

type fleetK8sHealthPayload struct {
	NodeName          string `json:"node_name"`
	KubeletHealthy    bool   `json:"kubelet_healthy"`
	ContainerdHealthy bool   `json:"containerd_healthy"`
	DockerHealthy     bool   `json:"docker_healthy"`
}

func resolveComputeHostToken(host domainDocker.ComputeHost) string {
	if host.Labels != nil {
		if token, ok := host.Labels["auth_token"]; ok && strings.TrimSpace(token) != "" {
			return strings.TrimSpace(token)
		}
		if token, ok := host.Labels["token"]; ok && strings.TrimSpace(token) != "" {
			return strings.TrimSpace(token)
		}
	}
	return os.Getenv("AGENT_AUTH_TOKEN")
}

func formatHostBaseURL(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if strings.HasPrefix(endpoint, "tcp://") {
			endpoint = "http://" + strings.TrimPrefix(endpoint, "tcp://")
		} else {
			endpoint = "http://" + endpoint
		}
	}
	if u, err := url.Parse(endpoint); err == nil && u.Port() == "" && u.Hostname() != "" {
		u.Host = net.JoinHostPort(u.Hostname(), "9100")
		endpoint = u.String()
	}
	return endpoint
}

func (r *deploymentRepo) discoverFleetWorkloads(ctx context.Context) ([]deployment.Application, error) {
	if r.computeHostRepo == nil {
		return []deployment.Application{}, nil
	}

	hosts, err := r.computeHostRepo.ListAll(ctx)
	if err != nil || len(hosts) == 0 {
		return []deployment.Application{}, nil
	}

	client := &http.Client{Timeout: 2 * time.Second}

	var mu sync.Mutex
	var applications []deployment.Application

	var wg sync.WaitGroup
	for _, h := range hosts {
		wg.Add(1)
		go func(host domainDocker.ComputeHost) {
			defer wg.Done()
			apps := r.discoverHostDaemons(ctx, client, host)
			if len(apps) > 0 {
				mu.Lock()
				applications = append(applications, apps...)
				mu.Unlock()
			}
		}(h)
	}
	wg.Wait()

	if applications == nil {
		applications = []deployment.Application{}
	}
	return applications, nil
}

func (r *deploymentRepo) discoverHostDaemons(ctx context.Context, client *http.Client, host domainDocker.ComputeHost) []deployment.Application {
	hostBaseURL := formatHostBaseURL(host.Endpoint)
	token := resolveComputeHostToken(host)

	hostCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var metricsPayload fleetMetricsPayload
	var healthPayload fleetK8sHealthPayload
	var metricsOK, healthOK bool

	var innerWG sync.WaitGroup
	innerWG.Add(2)

	// Query /metrics
	go func() {
		defer innerWG.Done()
		req, err := http.NewRequestWithContext(hostCtx, http.MethodGet, hostBaseURL+"/metrics", nil)
		if err != nil {
			return
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if dErr := json.NewDecoder(resp.Body).Decode(&metricsPayload); dErr == nil {
					metricsOK = true
				}
			}
		}
	}()

	// Query /k8s/health
	go func() {
		defer innerWG.Done()
		req, err := http.NewRequestWithContext(hostCtx, http.MethodGet, hostBaseURL+"/k8s/health", nil)
		if err != nil {
			return
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if dErr := json.NewDecoder(resp.Body).Decode(&healthPayload); dErr == nil {
					healthOK = true
				}
			}
		}
	}()

	innerWG.Wait()

	hostName := host.Name
	if hostName == "" {
		hostName = host.ID
	}

	// If host is completely unreachable
	if !metricsOK && !healthOK {
		return []deployment.Application{
			{
				Name:      "k8s-agent",
				Team:      "observability",
				Env:       "production",
				Image:     "k8s-agent:latest",
				Target:    hostName,
				Namespace: hostName,
				Type:      "daemon",
				Replicas:  0,
				Status:    "down",
				Port:      9100,
				NetType:   "HostPort",
			},
		}
	}

	// Index detected services and processes
	detectedMap := make(map[string]bool)
	for _, s := range metricsPayload.DetectedServices {
		detectedMap[strings.ToLower(strings.TrimSpace(s))] = true
	}

	procMap := make(map[string]fleetProcessMetric)
	for _, p := range metricsPayload.TopProcesses {
		nameLower := strings.ToLower(p.Name)
		procMap[nameLower] = p
	}

	var apps []deployment.Application

	for _, meta := range fleetDaemons {
		name := meta.canonicalName
		running := false
		var proc fleetProcessMetric
		var foundProc bool

		// Check runtime health
		switch name {
		case "k8s-agent":
			running = metricsOK || healthOK
		case "containerd":
			running = healthPayload.ContainerdHealthy || detectedMap["containerd"]
		case "dockerd":
			running = healthPayload.DockerHealthy || detectedMap["dockerd"] || detectedMap["docker"]
		case "kubelet":
			running = healthPayload.KubeletHealthy || detectedMap["kubelet"]
		case "kube-scheduler":
			running = detectedMap["kube-scheduler"] || detectedMap["scheduler"]
		case "postgres":
			running = detectedMap["postgres"] || detectedMap["postgresql"]
		case "redis-server":
			running = detectedMap["redis"] || detectedMap["redis-server"]
		case "proxysql":
			running = detectedMap["proxysql"]
		}

		// Check top processes
		for pName, pVal := range procMap {
			if strings.Contains(pName, name) || (name == "postgres" && strings.Contains(pName, "postgres")) || (name == "redis-server" && strings.Contains(pName, "redis")) {
				running = true
				proc = pVal
				foundProc = true
				break
			}
		}

		if !running {
			continue
		}

		cpuStr := "0.1%"
		memStr := "40Mi"
		if foundProc {
			cpuStr = fmt.Sprintf("%.1f%%", proc.CPUPercent)
			if proc.MemoryBytes > 0 {
				memStr = fmt.Sprintf("%dMi", proc.MemoryBytes/(1024*1024))
			}
		}

		apps = append(apps, deployment.Application{
			Name:      name,
			Team:      meta.team,
			Env:       "production",
			Image:     name + ":latest",
			Target:    hostName,
			Namespace: hostName,
			Type:      "daemon",
			Replicas:  1,
			Status:    "healthy",
			CPU:       cpuStr,
			Memory:    memStr,
			Port:      meta.port,
			NetType:   meta.netType,
		})
	}

	return apps
}
