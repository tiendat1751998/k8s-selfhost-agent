package ecosystem

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
)

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
