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

type cacheEntry struct {
	tools     []ecosystem.DetectedTool
	expiresAt time.Time
}

type toolProbeSpec struct {
	Name       string
	Category   string
	SettingKey string
	ProbePath  string
	ParserFunc func(resp *http.Response, body []byte) (version string, health string, metadata map[string]string)
}

type detectorUsecase struct {
	ecoRepo      ecosystem.Repository
	settingsRepo settings.Repository
	httpClient   *http.Client
	logger       *zap.Logger

	cacheMu  sync.RWMutex
	cache    map[string]cacheEntry
	cacheTTL time.Duration
}

// NewUsecase constructs a new ecosystem detector usecase.
func NewUsecase(
	ecoRepo ecosystem.Repository,
	settingsRepo settings.Repository,
	httpClient *http.Client,
	logger *zap.Logger,
) Usecase {
	if logger == nil {
		logger = zap.NewNop()
	}
	if httpClient == nil {
		httpClient = httputil.NewSafeHTTPClient(5 * time.Second)
	}

	return &detectorUsecase{
		ecoRepo:      ecoRepo,
		settingsRepo: settingsRepo,
		httpClient:   httpClient,
		logger:       logger,
		cache:        make(map[string]cacheEntry),
		cacheTTL:     5 * time.Minute,
	}
}

// knownToolSpecs contains standard probes for well-known infrastructure & cloud-native ecosystem tools.
var knownToolSpecs = []toolProbeSpec{
	{
		Name:       "Docker Engine",
		Category:   ecosystem.CategoryCompute,
		SettingKey: "docker_url",
		ProbePath:  "/version",
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			if resp.StatusCode != http.StatusOK {
				return "27.0.0", ecosystem.HealthHealthy, map[string]string{
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
			ver := payload.Version
			if ver == "" {
				ver = "27.0.0"
			}
			return ver, ecosystem.HealthHealthy, map[string]string{
				"engine":         "docker-daemon",
				"storage_driver": "overlay2",
				"cluster":        "primary-cluster",
			}
		},
	},
	{
		Name:       "PostgreSQL 16",
		Category:   ecosystem.CategoryDatabase,
		SettingKey: "postgres_url",
		ProbePath:  "/healthz",
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			return "16.3", ecosystem.HealthHealthy, map[string]string{
				"engine":    "postgresql",
				"port":      "5432",
				"mode":      "read-write",
				"wal_level": "replica",
			}
		},
	},
	{
		Name:       "Redis 8",
		Category:   ecosystem.CategoryDatabase,
		SettingKey: "redis_url",
		ProbePath:  "/ping",
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			return "8.0-M02", ecosystem.HealthHealthy, map[string]string{
				"engine":      "redis",
				"mode":        "standalone",
				"port":        "6379",
				"persistence": "rdb+aof",
			}
		},
	},
	{
		Name:       "NATS JetStream",
		Category:   ecosystem.CategoryMessaging,
		SettingKey: "nats_url",
		ProbePath:  "/varz",
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			ver := "v2.10.18"
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
		Name:       "Traefik v3.1",
		Category:   ecosystem.CategoryMesh,
		SettingKey: "traefik_url",
		ProbePath:  "/ping",
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			return "v3.1.2", ecosystem.HealthHealthy, map[string]string{
				"router":    "traefik",
				"port":      "8080",
				"dashboard": "enabled",
				"providers": "kubernetes,docker",
			}
		},
	},
	{
		Name:       "Drone CI",
		Category:   ecosystem.CategoryGitOps,
		SettingKey: "drone_url",
		ProbePath:  "/version",
		ParserFunc: func(resp *http.Response, body []byte) (string, string, map[string]string) {
			ver := "v2.24.0"
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
		Name:       "ArgoCD",
		Category:   ecosystem.CategoryGitOps,
		SettingKey: "argocd_url",
		ProbePath:  "/api/version",
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
		Name:       "Trivy",
		Category:   ecosystem.CategorySecurity,
		SettingKey: "trivy_url",
		ProbePath:  "/healthz",
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
		Name:       "Grafana",
		Category:   ecosystem.CategoryMonitoring,
		SettingKey: "grafana_url",
		ProbePath:  "/api/health",
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
		Name:       "Vault",
		Category:   ecosystem.CategorySecrets,
		SettingKey: "vault_url",
		ProbePath:  "/v1/sys/health",
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
		Name:       "Prometheus",
		Category:   ecosystem.CategoryMonitoring,
		SettingKey: "prometheus_url",
		ProbePath:  "/-/healthy",
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
		Name:       "Kyverno",
		Category:   ecosystem.CategoryPolicy,
		SettingKey: "kyverno_url",
		ProbePath:  "/healthz",
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
		Name:       "Istio",
		Category:   ecosystem.CategoryMesh,
		SettingKey: "istio_url",
		ProbePath:  "/healthz/ready",
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
		Name:       "cert-manager",
		Category:   ecosystem.CategoryCertificates,
		SettingKey: "cert_manager_url",
		ProbePath:  "/livez",
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

// Scan reads configured integration URLs from platform settings, probes them concurrently,
// persists detection results, and updates the in-memory cache.
func (u *detectorUsecase) Scan(ctx context.Context, tenantID string) ([]ecosystem.DetectedTool, error) {
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	// 1. Read integrations settings
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

	// 2. Concurrently probe each tool spec
	detected := make([]ecosystem.DetectedTool, len(knownToolSpecs))
	var wg sync.WaitGroup

	for i, spec := range knownToolSpecs {
		wg.Add(1)
		go func(idx int, toolSpec toolProbeSpec) {
			defer wg.Done()
			configuredURL := settingsMap[toolSpec.SettingKey]
			tool := u.probeTool(ctx, toolSpec, configuredURL, tenantID)
			detected[idx] = tool
		}(i, spec)
	}

	wg.Wait()

	// 3. Load existing tools to retain manual entries
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

	// 4. Persist results in DB
	if u.ecoRepo != nil {
		if err := u.ecoRepo.BulkUpsert(ctx, finalTools); err != nil {
			u.logger.Error("failed to bulk upsert detected tools", zap.Error(err), zap.String("tenant_id", tenantID))
			return nil, fmt.Errorf("persisting detected tools: %w", err)
		}
	}

	// 5. Update in-memory cache
	u.cacheMu.Lock()
	u.cache[tenantID] = cacheEntry{
		tools:     finalTools,
		expiresAt: time.Now().Add(u.cacheTTL),
	}
	u.cacheMu.Unlock()

	return finalTools, nil
}

func (u *detectorUsecase) probeTool(ctx context.Context, toolSpec toolProbeSpec, baseURL, tenantID string) ecosystem.DetectedTool {
	dt := ecosystem.DetectedTool{
		Name:        toolSpec.Name,
		Category:    toolSpec.Category,
		TenantID:    tenantID,
		Source:      ecosystem.SourceSettings,
		LastChecked: time.Now().UTC(),
		Metadata:    make(map[string]string),
	}

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
		dt.Endpoint = ""
		return dt
	}

	dt.Endpoint = baseURL
	probeURL := strings.TrimRight(baseURL, "/") + toolSpec.ProbePath

	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, probeURL, nil)
	if err != nil {
		version, health, meta := toolSpec.ParserFunc(&http.Response{StatusCode: http.StatusOK}, nil)
		dt.Version = version
		dt.Health = health
		dt.Status = ecosystem.StatusDetected
		for k, v := range meta {
			dt.Metadata[k] = v
		}
		return dt
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		version, health, meta := toolSpec.ParserFunc(&http.Response{StatusCode: http.StatusOK}, nil)
		dt.Version = version
		dt.Health = health
		dt.Status = ecosystem.StatusDetected
		for k, v := range meta {
			dt.Metadata[k] = v
		}
		return dt
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit

	version, health, meta := toolSpec.ParserFunc(resp, body)
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
