package ecosystem

import (
	"encoding/json"
	"fmt"
	"net/http"

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

func simpleProbeParser(probePath string) func(resp *http.Response, body []byte) (string, string, map[string]string) {
	return func(resp *http.Response, body []byte) (string, string, map[string]string) {
		if resp.StatusCode != http.StatusOK {
			return "", ecosystem.HealthDegraded, map[string]string{
				"probe":       probePath,
				"status_code": fmt.Sprintf("%d", resp.StatusCode),
			}
		}
		return "", ecosystem.HealthHealthy, map[string]string{
			"probe":       probePath,
			"status_code": "200",
		}
	}
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
			meta := map[string]string{"engine": "docker-daemon", "storage_driver": "overlay2", "cluster": "primary-cluster"}
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, meta
			}
			var payload struct {
				Version    string `json:"Version"`
				ApiVersion string `json:"ApiVersion"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				meta["error"] = fmt.Sprintf("failed to parse version response: %v", err)
				return "", ecosystem.HealthDegraded, meta
			}
			return payload.Version, ecosystem.HealthHealthy, meta
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
				return "", ecosystem.HealthDegraded, map[string]string{"engine": "postgresql"}
			}
			return "", ecosystem.HealthHealthy, map[string]string{"engine": "postgresql", "port": "5432", "mode": "read-write", "wal_level": "replica"}
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
				return "", ecosystem.HealthDegraded, map[string]string{"engine": "redis"}
			}
			return "", ecosystem.HealthHealthy, map[string]string{"engine": "redis", "mode": "standalone", "port": "6379", "persistence": "rdb+aof"}
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
				return "", ecosystem.HealthDegraded, map[string]string{"engine": "nats"}
			}
			ver := ""
			var payload struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(body, &payload); err == nil && payload.Version != "" {
				ver = payload.Version
			}
			return ver, ecosystem.HealthHealthy, map[string]string{"jetstream": "enabled", "port": "4222", "http_port": "8222", "cluster": "primary-cluster"}
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
				return "", ecosystem.HealthDegraded, map[string]string{"router": "traefik"}
			}
			return "", ecosystem.HealthHealthy, map[string]string{"router": "traefik", "port": "8080", "dashboard": "enabled", "providers": "kubernetes,docker"}
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
				return "", ecosystem.HealthDegraded, map[string]string{"runner": "docker-runner"}
			}
			ver := ""
			var payload struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(body, &payload); err == nil && payload.Version != "" {
				ver = payload.Version
			}
			return ver, ecosystem.HealthHealthy, map[string]string{"runner": "docker-runner", "port": "80", "auth_provider": "github"}
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
				return "", ecosystem.HealthDegraded, map[string]string{"probe": "/api/version", "status_code": fmt.Sprintf("%d", resp.StatusCode)}
			}
			var payload struct {
				Version    string `json:"Version"`
				Ver        string `json:"version"`
				GitVersion string `json:"gitVersion"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				return "", ecosystem.HealthDegraded, map[string]string{"probe": "/api/version", "status_code": "200", "error": fmt.Sprintf("failed to parse version response: %v", err)}
			}
			ver := payload.Version
			if ver == "" {
				ver = payload.Ver
			}
			if ver == "" {
				ver = payload.GitVersion
			}
			return ver, ecosystem.HealthHealthy, map[string]string{"probe": "/api/version", "status_code": "200"}
		},
	},
	{
		Name:          "Trivy",
		Category:      ecosystem.CategorySecurity,
		SettingKey:    "trivy_url",
		ProbePath:     "/healthz",
		MatchKeywords: []string{"trivy"},
		ParserFunc:    simpleProbeParser("/healthz"),
	},
	{
		Name:          "Grafana",
		Category:      ecosystem.CategoryMonitoring,
		SettingKey:    "grafana_url",
		ProbePath:     "/api/health",
		MatchKeywords: []string{"grafana"},
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "", ecosystem.HealthDegraded, map[string]string{"probe": "/api/health", "status_code": fmt.Sprintf("%d", resp.StatusCode)}
			}
			var payload struct {
				Version  string `json:"version"`
				Database string `json:"database"`
				Commit   string `json:"commit"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				return "", ecosystem.HealthDegraded, map[string]string{"probe": "/api/health", "status_code": "200", "error": fmt.Sprintf("failed to parse health response: %v", err)}
			}
			health := ecosystem.HealthHealthy
			if payload.Database != "" && payload.Database != "ok" {
				health = ecosystem.HealthDegraded
			}
			meta := map[string]string{"probe": "/api/health", "status_code": "200"}
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
			if err := json.Unmarshal(body, &payload); err != nil {
				return "", ecosystem.HealthDegraded, map[string]string{"probe": "/v1/sys/health", "status_code": fmt.Sprintf("%d", resp.StatusCode), "error": fmt.Sprintf("failed to parse health response: %v", err)}
			}
			health := ecosystem.HealthHealthy
			if payload.Sealed || (resp.StatusCode != http.StatusOK && resp.StatusCode != 429) {
				health = ecosystem.HealthDegraded
			}
			return payload.ServerVersion, health, map[string]string{
				"probe":       "/v1/sys/health",
				"status_code": fmt.Sprintf("%d", resp.StatusCode),
				"sealed":      fmt.Sprintf("%t", payload.Sealed),
				"initialized": fmt.Sprintf("%t", payload.Initialized),
			}
		},
	},
	{
		Name:          "Prometheus",
		Category:      ecosystem.CategoryMonitoring,
		SettingKey:    "prometheus_url",
		ProbePath:     "/-/healthy",
		MatchKeywords: []string{"prometheus"},
		ParserFunc:    simpleProbeParser("/-/healthy"),
	},
	{
		Name:          "Kyverno",
		Category:      ecosystem.CategoryPolicy,
		SettingKey:    "kyverno_url",
		ProbePath:     "/healthz",
		MatchKeywords: []string{"kyverno"},
		ParserFunc:    simpleProbeParser("/healthz"),
	},
	{
		Name:          "Istio",
		Category:      ecosystem.CategoryMesh,
		SettingKey:    "istio_url",
		ProbePath:     "/healthz/ready",
		MatchKeywords: []string{"istio", "pilot"},
		ParserFunc:    simpleProbeParser("/healthz/ready"),
	},
	{
		Name:          "cert-manager",
		Category:      ecosystem.CategoryCertificates,
		SettingKey:    "cert_manager_url",
		ProbePath:     "/livez",
		MatchKeywords: []string{"cert-manager", "cert_manager"},
		ParserFunc:    simpleProbeParser("/livez"),
	},
}
