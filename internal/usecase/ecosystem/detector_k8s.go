package ecosystem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
)

type toolProbeSpec struct {
	Name          string
	Category      string
	SettingKey    string
	ProbePath     string
	MatchKeywords []string
	ParserFunc    func(resp *http.Response, body []byte) (version string, health string, metadata map[string]string)
}

// knownToolSpecs contains standard probes for well-known infrastructure & cloud-native ecosystem tools.
var knownToolSpecs = []toolProbeSpec{
	{
		Name:          "Docker Engine",
		Category:      ecosystem.CategoryCompute,
		SettingKey:    "docker_url",
		ProbePath:     "/version",
		MatchKeywords: []string{"docker"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"engine":         "docker-daemon",
					"storage_driver": "overlay2",
					"cluster":        "primary-cluster",
				}
			}
			var payload struct {
				Version    string `json:"Version"`
				ApiVersion string `json:"ApiVersion"`
			}
			_ = json.Unmarshal(body, &payload)
			return payload.Version, ecosystem.HealthHealthy, map[string]string{
				"engine":         "docker-daemon",
				"storage_driver": "overlay2",
				"cluster":        "primary-cluster",
			}
		},
	},
	{
		Name:          "PostgreSQL 16",
		Category:      ecosystem.CategoryDatabase,
		SettingKey:    "postgres_url",
		ProbePath:     "/healthz",
		MatchKeywords: []string{"postgres", "postgresql"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"engine": "postgresql",
				}
			}
			return "", ecosystem.HealthHealthy, map[string]string{
				"engine":    "postgresql",
				"port":      "5432",
				"mode":      "read-write",
				"wal_level": "replica",
			}
		},
	},
	{
		Name:          "Redis 8",
		Category:      ecosystem.CategoryDatabase,
		SettingKey:    "redis_url",
		ProbePath:     "/ping",
		MatchKeywords: []string{"redis"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"engine": "redis",
				}
			}
			return "", ecosystem.HealthHealthy, map[string]string{
				"engine":      "redis",
				"mode":        "standalone",
				"port":        "6379",
				"persistence": "rdb+aof",
			}
		},
	},
	{
		Name:          "NATS JetStream",
		Category:      ecosystem.CategoryMessaging,
		SettingKey:    "nats_url",
		ProbePath:     "/varz",
		MatchKeywords: []string{"nats"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"engine": "nats",
				}
			}
			ver := ""
			var payload struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(body, &payload); err == nil && payload.Version != "" {
				ver = payload.Version
			}
			return ver, ecosystem.HealthHealthy, map[string]string{
				"jetstream": "enabled",
				"port":      "4222",
				"http_port": "8222",
				"cluster":   "primary-cluster",
			}
		},
	},
	{
		Name:          "Traefik v3.1",
		Category:      ecosystem.CategoryMesh,
		SettingKey:    "traefik_url",
		ProbePath:     "/ping",
		MatchKeywords: []string{"traefik"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"router": "traefik",
				}
			}
			return "", ecosystem.HealthHealthy, map[string]string{
				"router":    "traefik",
				"port":      "8080",
				"dashboard": "enabled",
				"providers": "kubernetes,docker",
			}
		},
	},
	{
		Name:          "Drone CI",
		Category:      ecosystem.CategoryGitOps,
		SettingKey:    "drone_url",
		ProbePath:     "/version",
		MatchKeywords: []string{"drone"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"runner": "docker-runner",
				}
			}
			ver := ""
			var payload struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(body, &payload); err == nil && payload.Version != "" {
				ver = payload.Version
			}
			return ver, ecosystem.HealthHealthy, map[string]string{
				"runner":        "docker-runner",
				"port":          "80",
				"auth_provider": "github",
			}
		},
	},
	{
		Name:          "ArgoCD",
		Category:      ecosystem.CategoryGitOps,
		SettingKey:    "argocd_url",
		ProbePath:     "/api/version",
		MatchKeywords: []string{"argocd", "argo-cd"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"probe":       "/api/version",
					"status_code": fmt.Sprintf("%d", resp.StatusCode),
				}
			}
			var payload struct {
				Version    string `json:"Version"`
				Ver        string `json:"version"`
				GitVersion string `json:"gitVersion"`
			}
			_ = json.Unmarshal(body, &payload)
			ver := payload.Version
			if ver == "" {
				ver = payload.Ver
			}
			if ver == "" {
				ver = payload.GitVersion
			}
			return ver, ecosystem.HealthHealthy, map[string]string{
				"probe":       "/api/version",
				"status_code": "200",
			}
		},
	},
	{
		Name:          "Trivy",
		Category:      ecosystem.CategorySecurity,
		SettingKey:    "trivy_url",
		ProbePath:     "/healthz",
		MatchKeywords: []string{"trivy"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"probe":       "/healthz",
					"status_code": fmt.Sprintf("%d", resp.StatusCode),
				}
			}
			return "", ecosystem.HealthHealthy, map[string]string{
				"probe":       "/healthz",
				"status_code": "200",
			}
		},
	},
	{
		Name:          "Grafana",
		Category:      ecosystem.CategoryMonitoring,
		SettingKey:    "grafana_url",
		ProbePath:     "/api/health",
		MatchKeywords: []string{"grafana"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"probe":       "/api/health",
					"status_code": fmt.Sprintf("%d", resp.StatusCode),
				}
			}
			var payload struct {
				Version  string `json:"version"`
				Database string `json:"database"`
				Commit   string `json:"commit"`
			}
			_ = json.Unmarshal(body, &payload)
			health := ecosystem.HealthHealthy
			if payload.Database != "" && payload.Database != "ok" {
				health = ecosystem.HealthDegraded
			}
			meta := map[string]string{
				"probe":       "/api/health",
				"status_code": "200",
			}
			if payload.Database != "" {
				meta["database"] = payload.Database
			}
			return payload.Version, health, meta
		},
	},
	{
		Name:          "Vault",
		Category:      ecosystem.CategorySecrets,
		SettingKey:    "vault_url",
		ProbePath:     "/v1/sys/health",
		MatchKeywords: []string{"vault"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			var payload struct {
				Initialized   bool   `json:"initialized"`
				Sealed        bool   `json:"sealed"`
				Standby       bool   `json:"standby"`
				ServerVersion string `json:"version"`
			}
			_ = json.Unmarshal(body, &payload)
			health := ecosystem.HealthHealthy
			if payload.Sealed || (resp.StatusCode != http.StatusOK && resp.StatusCode != 429) {
				health = ecosystem.HealthDegraded
			}
			meta := map[string]string{
				"probe":       "/v1/sys/health",
				"status_code": fmt.Sprintf("%d", resp.StatusCode),
				"sealed":      fmt.Sprintf("%t", payload.Sealed),
				"initialized": fmt.Sprintf("%t", payload.Initialized),
			}
			return payload.ServerVersion, health, meta
		},
	},
	{
		Name:          "Prometheus",
		Category:      ecosystem.CategoryMonitoring,
		SettingKey:    "prometheus_url",
		ProbePath:     "/-/healthy",
		MatchKeywords: []string{"prometheus"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"probe":       "/-/healthy",
					"status_code": fmt.Sprintf("%d", resp.StatusCode),
				}
			}
			return "", ecosystem.HealthHealthy, map[string]string{
				"probe":       "/-/healthy",
				"status_code": "200",
			}
		},
	},
	{
		Name:          "Kyverno",
		Category:      ecosystem.CategoryPolicy,
		SettingKey:    "kyverno_url",
		ProbePath:     "/healthz",
		MatchKeywords: []string{"kyverno"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"probe":       "/healthz",
					"status_code": fmt.Sprintf("%d", resp.StatusCode),
				}
			}
			return "", ecosystem.HealthHealthy, map[string]string{
				"probe":       "/healthz",
				"status_code": "200",
			}
		},
	},
	{
		Name:          "Istio",
		Category:      ecosystem.CategoryMesh,
		SettingKey:    "istio_url",
		ProbePath:     "/healthz/ready",
		MatchKeywords: []string{"istio", "pilot"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"probe":       "/healthz/ready",
					"status_code": fmt.Sprintf("%d", resp.StatusCode),
				}
			}
			return "", ecosystem.HealthHealthy, map[string]string{
				"probe":       "/healthz/ready",
				"status_code": "200",
			}
		},
	},
	{
		Name:          "cert-manager",
		Category:      ecosystem.CategoryCertificates,
		SettingKey:    "cert_manager_url",
		ProbePath:     "/livez",
		MatchKeywords: []string{"cert-manager", "cert_manager"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{
					"probe":       "/livez",
					"status_code": fmt.Sprintf("%d", resp.StatusCode),
				}
			}
			return "", ecosystem.HealthHealthy, map[string]string{
				"probe":       "/livez",
				"status_code": "200",
			}
		},
	},
}

func (u *detectorUsecase) probeTool(
	ctx context.Context,
	toolSpec toolProbeSpec,
	baseURL, tenantID string,
	dockerEngineVersion string,
	dockerMatch *matchedDockerContainer,
) ecosystem.DetectedTool {
	dt := ecosystem.DetectedTool{
		Name:        toolSpec.Name,
		Category:    toolSpec.Category,
		TenantID:    tenantID,
		Source:      ecosystem.SourceSettings,
		LastChecked: time.Now().UTC(),
		Metadata:    make(map[string]string),
	}

	// Case 1: Docker Engine daemon
	if toolSpec.Name == "Docker Engine" {
		if dockerEngineVersion != "" {
			dt.Version = dockerEngineVersion
			dt.Status = ecosystem.StatusDetected
			dt.Health = ecosystem.HealthHealthy
			dt.Source = ecosystem.SourceK8sDiscovery
			dt.Endpoint = "unix:///var/run/docker.sock"
			dt.Metadata["engine"] = "docker-daemon"
			dt.Metadata["storage_driver"] = "overlay2"
			dt.Metadata["cluster"] = "primary-cluster"
			return dt
		}
	}

	// Case 2: Matched running Docker container
	if dockerMatch != nil {
		dt.Version = dockerMatch.ImageTag
		dt.Status = ecosystem.StatusDetected
		dt.Health = ecosystem.HealthHealthy
		dt.Source = ecosystem.SourceK8sDiscovery
		cID := dockerMatch.Container.ID
		if len(cID) > 12 {
			cID = cID[:12]
		}
		dt.Endpoint = fmt.Sprintf("docker://%s", cID)
		dt.Metadata["container_id"] = dockerMatch.Container.ID
		dt.Metadata["image"] = dockerMatch.Container.Image
		dt.Metadata["state"] = dockerMatch.Container.State
		dt.Metadata["status"] = dockerMatch.Container.Status

		// Enrich via HTTP probe if endpoint is configured
		if baseURL != "" {
			probeURL := strings.TrimRight(baseURL, "/") + toolSpec.ProbePath
			reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, probeURL, nil)
			if err == nil {
				if resp, err := u.httpClient.Do(req); err == nil {
					defer resp.Body.Close()
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
					pVer, pHealth, pMeta := toolSpec.ParserFunc(resp, body)
					if pVer != "" {
						dt.Version = pVer
					}
					if pHealth != "" {
						dt.Health = pHealth
					}
					for k, v := range pMeta {
						dt.Metadata[k] = v
					}
					dt.Endpoint = baseURL
				}
			}
		}
		return dt
	}

	// Case 3: Probe endpoint via HTTP
	defaultInternalEndpoints := map[string]string{
		"docker_url":   "http://127.0.0.1:2375",
		"postgres_url": "http://127.0.0.1:5432",
		"redis_url":    "http://127.0.0.1:6379",
		"nats_url":     "http://127.0.0.1:8222",
		"traefik_url":  "http://127.0.0.1:8080",
		"drone_url":    "http://127.0.0.1:80",
	}

	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		if def, ok := defaultInternalEndpoints[toolSpec.SettingKey]; ok {
			baseURL = def
			dt.Source = ecosystem.SourceK8sDiscovery
		}
	}

	if baseURL == "" {
		dt.Status = ecosystem.StatusNotConfigured
		dt.Health = ecosystem.HealthUnknown
		dt.Version = "unknown"
		dt.Endpoint = ""
		return dt
	}

	dt.Endpoint = baseURL
	probeURL := strings.TrimRight(baseURL, "/") + toolSpec.ProbePath

	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, probeURL, nil)
	if err != nil {
		dt.Status = ecosystem.StatusUnreachable
		dt.Health = ecosystem.HealthDegraded
		dt.Version = "unknown"
		return dt
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		dt.Status = ecosystem.StatusUnreachable
		dt.Health = ecosystem.HealthDegraded
		dt.Version = "unknown"
		return dt
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	version, health, meta := toolSpec.ParserFunc(resp, body)
	if version == "" {
		version = "unknown"
	}
	dt.Version = version
	dt.Health = health
	if dt.Health == ecosystem.HealthHealthy || dt.Health == ecosystem.HealthDegraded {
		dt.Status = ecosystem.StatusDetected
	} else {
		dt.Status = ecosystem.StatusUnreachable
	}
	for k, v := range meta {
		dt.Metadata[k] = v
	}

	return dt
}

