package ecosystem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
	"github.com/datdt/k8sselfhost/internal/domain/settings"
	"github.com/datdt/k8sselfhost/internal/pkg/httputil"
)

// Usecase defines the ecosystem detection business logic and tool discovery interface.
type Usecase interface {
	Scan(ctx context.Context, tenantID string) ([]ecosystem.DetectedTool, error)
	GetAll(ctx context.Context, tenantID string) ([]ecosystem.DetectedTool, error)
	GetSummary(ctx context.Context, tenantID string) (*ecosystem.EcosystemSummary, error)
	CreateTool(ctx context.Context, tool *ecosystem.DetectedTool) error
	DeleteTool(ctx context.Context, tenantID, id string) error
}

// DockerAPIClient defines the Docker API subset required for container inspection and version detection.
type DockerAPIClient interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ServerVersion(ctx context.Context) (types.Version, error)
}

type cacheEntry struct {
	tools     []ecosystem.DetectedTool
	expiresAt time.Time
}

type toolProbeSpec struct {
	Name          string
	Category      string
	SettingKey    string
	ProbePath     string
	MatchKeywords []string
	ParserFunc    func(resp *http.Response, body []byte) (version string, health string, metadata map[string]string)
}

type detectorUsecase struct {
	ecoRepo      ecosystem.Repository
	settingsRepo settings.Repository
	dockerClient DockerAPIClient
	httpClient   *http.Client
	logger       *zap.Logger

	cacheMu  sync.RWMutex
	cache    map[string]cacheEntry
	cacheTTL time.Duration
}

// Option configures detector usecase behavior.
type Option func(*detectorUsecase)

// WithDockerClient configures a Docker API client for live container inspection.
func WithDockerClient(dockerClient DockerAPIClient) Option {
	return func(u *detectorUsecase) {
		u.dockerClient = dockerClient
	}
}

// WithCacheTTL configures the in-memory cache TTL.
func WithCacheTTL(ttl time.Duration) Option {
	return func(u *detectorUsecase) {
		if ttl > 0 {
			u.cacheTTL = ttl
		}
	}
}

// NewUsecase constructs a new ecosystem detector usecase.
func NewUsecase(
	ecoRepo ecosystem.Repository,
	settingsRepo settings.Repository,
	httpClient *http.Client,
	logger *zap.Logger,
	opts ...Option,
) Usecase {
	if logger == nil {
		logger = zap.NewNop()
	}
	if httpClient == nil {
		httpClient = httputil.NewSafeHTTPClient(5 * time.Second)
	}

	u := &detectorUsecase{
		ecoRepo:      ecoRepo,
		settingsRepo: settingsRepo,
		httpClient:   httpClient,
		logger:       logger,
		cache:        make(map[string]cacheEntry),
		cacheTTL:     5 * time.Minute,
	}

	for _, opt := range opts {
		opt(u)
	}

	return u
}

// extractImageTag extracts the tag portion from a Docker container image string.
// Examples:
// - "redis:8-alpine" -> "8-alpine"
// - "docker.io/library/postgres:16.3-alpine" -> "16.3-alpine"
// - "registry.local:5000/repo/app:v1.2.0@sha256:abc..." -> "v1.2.0"
// - "postgres" -> "unknown"
func extractImageTag(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return "unknown"
	}
	// Strip digest (@sha256:...)
	if idx := strings.Index(image, "@"); idx != -1 {
		image = image[:idx]
	}
	// Extract the repository and tag part after the last slash to avoid confusing host:port with a tag
	lastSlash := strings.LastIndex(image, "/")
	repoAndTag := image
	if lastSlash != -1 {
		repoAndTag = image[lastSlash+1:]
	}
	// Find tag after the last colon
	if colonIdx := strings.LastIndex(repoAndTag, ":"); colonIdx != -1 {
		tag := strings.TrimSpace(repoAndTag[colonIdx+1:])
		if tag != "" {
			return tag
		}
	}
	return "unknown"
}

// matchCategoryForImageOrName infers the ecosystem category based on container image and name keywords.
func matchCategoryForImageOrName(image, name string) string {
	combined := strings.ToLower(image + " " + name)
	switch {
	case strings.Contains(combined, "postgres") || strings.Contains(combined, "mysql") ||
		strings.Contains(combined, "mariadb") || strings.Contains(combined, "mongo") ||
		strings.Contains(combined, "redis") || strings.Contains(combined, "cockroach") ||
		strings.Contains(combined, "cassandra") || strings.Contains(combined, "scylla") ||
		strings.Contains(combined, "clickhouse") || strings.Contains(combined, "elasticsearch") ||
		strings.Contains(combined, "opensearch") || strings.Contains(combined, "minio"):
		return ecosystem.CategoryDatabase
	case strings.Contains(combined, "nats") || strings.Contains(combined, "kafka") ||
		strings.Contains(combined, "rabbitmq") || strings.Contains(combined, "pulsar") ||
		strings.Contains(combined, "mosquitto") || strings.Contains(combined, "emqx"):
		return ecosystem.CategoryMessaging
	case strings.Contains(combined, "traefik") || strings.Contains(combined, "envoy") ||
		strings.Contains(combined, "istio") || strings.Contains(combined, "nginx") ||
		strings.Contains(combined, "kong") || strings.Contains(combined, "haproxy") ||
		strings.Contains(combined, "caddy") || strings.Contains(combined, "linkerd"):
		return ecosystem.CategoryMesh
	case strings.Contains(combined, "drone") || strings.Contains(combined, "argocd") ||
		strings.Contains(combined, "argo") || strings.Contains(combined, "flux") ||
		strings.Contains(combined, "tekton") || strings.Contains(combined, "jenkins") ||
		strings.Contains(combined, "gitea") || strings.Contains(combined, "gitlab"):
		return ecosystem.CategoryGitOps
	case strings.Contains(combined, "trivy") || strings.Contains(combined, "falco") ||
		strings.Contains(combined, "clair") || strings.Contains(combined, "grype") ||
		strings.Contains(combined, "cosign"):
		return ecosystem.CategorySecurity
	case strings.Contains(combined, "prometheus") || strings.Contains(combined, "grafana") ||
		strings.Contains(combined, "jaeger") || strings.Contains(combined, "loki") ||
		strings.Contains(combined, "tempo") || strings.Contains(combined, "alertmanager") ||
		strings.Contains(combined, "victoriametrics") || strings.Contains(combined, "thanos"):
		return ecosystem.CategoryMonitoring
	case strings.Contains(combined, "vault") || strings.Contains(combined, "sealed-secrets"):
		return ecosystem.CategorySecrets
	case strings.Contains(combined, "kyverno") || strings.Contains(combined, "gatekeeper") ||
		strings.Contains(combined, "opa"):
		return ecosystem.CategoryPolicy
	case strings.Contains(combined, "cert-manager") || strings.Contains(combined, "cert_manager"):
		return ecosystem.CategoryCertificates
	default:
		return ecosystem.CategoryCompute
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

type matchedDockerContainer struct {
	SpecIndex   int
	Container   container.Summary
	ImageTag    string
	MatchedTool string
	Category    string
}

// queryDocker inspects running containers and server version from the Docker API.
func (u *detectorUsecase) queryDocker(ctx context.Context) (string, map[string]matchedDockerContainer, []matchedDockerContainer) {
	matched := make(map[string]matchedDockerContainer)
	var unmatched []matchedDockerContainer
	var dockerEngineVersion string

	if u.dockerClient == nil {
		return "", matched, nil
	}

	// 1. Query Docker Engine daemon version
	ver, err := u.dockerClient.ServerVersion(ctx)
	if err == nil {
		dockerEngineVersion = ver.Version
	}

	// 2. Query running Docker containers
	containers, err := u.dockerClient.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		u.logger.Warn("failed to list docker containers for ecosystem detection", zap.Error(err))
		return dockerEngineVersion, matched, nil
	}

	for _, c := range containers {
		tag := extractImageTag(c.Image)
		imageLower := strings.ToLower(c.Image)
		nameLower := ""
		if len(c.Names) > 0 {
			nameLower = strings.ToLower(strings.TrimPrefix(c.Names[0], "/"))
		}

		foundSpec := false
		for i, spec := range knownToolSpecs {
			if spec.Name == "Docker Engine" {
				continue
			}
			for _, kw := range spec.MatchKeywords {
				if strings.Contains(imageLower, kw) || (nameLower != "" && strings.Contains(nameLower, kw)) {
					matched[spec.Name] = matchedDockerContainer{
						SpecIndex:   i,
						Container:   c,
						ImageTag:    tag,
						MatchedTool: spec.Name,
						Category:    spec.Category,
					}
					foundSpec = true
					break
				}
			}
			if foundSpec {
				break
			}
		}

		if !foundSpec {
			cat := matchCategoryForImageOrName(c.Image, nameLower)
			toolName := nameLower
			if toolName == "" {
				toolName = c.Image
			}
			unmatched = append(unmatched, matchedDockerContainer{
				SpecIndex:   -1,
				Container:   c,
				ImageTag:    tag,
				MatchedTool: toolName,
				Category:    cat,
			})
		}
	}

	return dockerEngineVersion, matched, unmatched
}

// Scan reads configured integration URLs from platform settings, probes them concurrently,
// queries live Docker container metadata, persists detection results, and updates the in-memory cache.
func (u *detectorUsecase) Scan(ctx context.Context, tenantID string) ([]ecosystem.DetectedTool, error) {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	// 1. Query Docker daemon and running containers
	dockerEngineVer, matchedContainers, unmatchedContainers := u.queryDocker(ctx)

	// 2. Read integrations settings
	settingsMap := make(map[string]string)
	if u.settingsRepo != nil {
		items, err := u.settingsRepo.GetByCategory(ctx, tenantID, settings.CategoryIntegrations)
		if err != nil {
			u.logger.Warn("failed to fetch integration settings for ecosystem scan", zap.Error(err), zap.String("tenant_id", tenantID))
		} else {
			for _, item := range items {
				settingsMap[item.Key] = item.Value
			}
		}
	}

	// 3. Concurrently probe each tool spec
	detected := make([]ecosystem.DetectedTool, len(knownToolSpecs))
	var wg sync.WaitGroup

	for i, spec := range knownToolSpecs {
		wg.Add(1)
		go func(idx int, toolSpec toolProbeSpec) {
			defer wg.Done()
			configuredURL := settingsMap[toolSpec.SettingKey]

			var dockerMatch *matchedDockerContainer
			if m, ok := matchedContainers[toolSpec.Name]; ok {
				dockerMatch = &m
			}

			tool := u.probeTool(ctx, toolSpec, configuredURL, tenantID, dockerEngineVer, dockerMatch)
			detected[idx] = tool
		}(i, spec)
	}

	wg.Wait()

	// 4. Add running containers that did not match known specs
	for _, uc := range unmatchedContainers {
		cName := uc.MatchedTool
		if len(uc.Container.Names) > 0 {
			cName = strings.TrimPrefix(uc.Container.Names[0], "/")
		}
		cID := uc.Container.ID
		if len(cID) > 12 {
			cID = cID[:12]
		}
		detected = append(detected, ecosystem.DetectedTool{
			Name:        cName,
			Category:    uc.Category,
			Status:      ecosystem.StatusDetected,
			Version:     uc.ImageTag,
			Endpoint:    fmt.Sprintf("docker://%s", cID),
			Source:      ecosystem.SourceK8sDiscovery,
			Health:      ecosystem.HealthHealthy,
			LastChecked: time.Now().UTC(),
			TenantID:    tenantID,
			Metadata: map[string]string{
				"container_id": uc.Container.ID,
				"image":        uc.Container.Image,
				"state":        uc.Container.State,
				"status":       uc.Container.Status,
			},
		})
	}

	// 5. Load existing tools to retain manual entries
	var finalTools []ecosystem.DetectedTool
	if u.ecoRepo != nil {
		existing, err := u.ecoRepo.GetAll(ctx, tenantID)
		if err == nil {
			for _, ex := range existing {
				if ex.Source == ecosystem.SourceManual {
					finalTools = append(finalTools, ex)
				}
			}
		}
	}

	finalTools = append(finalTools, detected...)

	// 6. Persist results in DB
	if u.ecoRepo != nil {
		if err := u.ecoRepo.BulkUpsert(ctx, finalTools); err != nil {
			u.logger.Error("failed to bulk upsert detected tools", zap.Error(err), zap.String("tenant_id", tenantID))
			return nil, fmt.Errorf("persisting detected tools: %w", err)
		}
	}

	// 7. Update in-memory cache
	u.cacheMu.Lock()
	u.cache[tenantID] = cacheEntry{
		tools:     finalTools,
		expiresAt: time.Now().Add(u.cacheTTL),
	}
	u.cacheMu.Unlock()

	return finalTools, nil
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

// GetAll returns all detected tools for a tenant, utilizing cache if valid.
func (u *detectorUsecase) GetAll(ctx context.Context, tenantID string) ([]ecosystem.DetectedTool, error) {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	// 1. Check in-memory cache
	u.cacheMu.RLock()
	entry, ok := u.cache[tenantID]
	u.cacheMu.RUnlock()

	if ok && time.Now().Before(entry.expiresAt) {
		return entry.tools, nil
	}

	// 2. Fetch from repository
	if u.ecoRepo != nil {
		tools, err := u.ecoRepo.GetAll(ctx, tenantID)
		if err != nil {
			return nil, fmt.Errorf("fetching ecosystem tools: %w", err)
		}
		if len(tools) > 0 {
			u.cacheMu.Lock()
			u.cache[tenantID] = cacheEntry{
				tools:     tools,
				expiresAt: time.Now().Add(u.cacheTTL),
			}
			u.cacheMu.Unlock()
			return tools, nil
		}
	}

	// 3. If repo is empty, perform initial scan
	return u.Scan(ctx, tenantID)
}

// GetSummary returns aggregated health and category counts for the tenant's ecosystem tools.
func (u *detectorUsecase) GetSummary(ctx context.Context, tenantID string) (*ecosystem.EcosystemSummary, error) {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	tools, err := u.GetAll(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	summary := &ecosystem.EcosystemSummary{
		Total:      len(tools),
		Healthy:    0,
		Degraded:   0,
		ByCategory: make(map[string]int),
	}

	for _, t := range tools {
		summary.ByCategory[t.Category]++
		if t.Health == ecosystem.HealthHealthy {
			summary.Healthy++
		} else if t.Health == ecosystem.HealthDegraded || t.Status == ecosystem.StatusUnreachable {
			summary.Degraded++
		}
	}

	return summary, nil
}

// CreateTool registers a manual tool entry and invalidates the tenant cache.
func (u *detectorUsecase) CreateTool(ctx context.Context, tool *ecosystem.DetectedTool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	if tool.Source == "" {
		tool.Source = ecosystem.SourceManual
	}

	if err := u.ecoRepo.Create(ctx, tool); err != nil {
		return fmt.Errorf("creating ecosystem tool: %w", err)
	}

	// Invalidate cache
	u.cacheMu.Lock()
	delete(u.cache, tool.TenantID)
	u.cacheMu.Unlock()

	return nil
}

// DeleteTool removes a tool and invalidates the tenant cache.
func (u *detectorUsecase) DeleteTool(ctx context.Context, tenantID, id string) error {
	if err := u.ecoRepo.Delete(ctx, tenantID, id); err != nil {
		return fmt.Errorf("deleting ecosystem tool: %w", err)
	}

	// Invalidate cache
	u.cacheMu.Lock()
	delete(u.cache, tenantID)
	u.cacheMu.Unlock()

	return nil
}
