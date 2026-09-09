package ecosystem

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/ecosystem"
)

// DockerAPIClient defines the Docker API subset required for container inspection and version detection.
type DockerAPIClient interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ServerVersion(ctx context.Context) (types.Version, error)
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

