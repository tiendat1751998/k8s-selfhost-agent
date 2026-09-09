package ecosystem

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
	"github.com/datdt/k8sselfhost/internal/domain/settings"
	"github.com/datdt/k8sselfhost/internal/pkg/httputil"
)

// EcosystemDetector is an alias for detectorUsecase representing the core detection orchestrator.
type EcosystemDetector = detectorUsecase

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

