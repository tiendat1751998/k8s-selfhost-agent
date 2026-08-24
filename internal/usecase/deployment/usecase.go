package deployment

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/datdt/k8sselfhost/internal/domain/deployment"
)

// UpdateResourcesRequest contains resource limits and scaling configuration.
type UpdateResourcesRequest struct {
	Type              string `json:"type"`
	Cluster           string `json:"cluster"`
	Namespace         string `json:"namespace"`
	Name              string `json:"name"`
	Replicas          int    `json:"replicas"`
	MemoryLimit       string `json:"memory_limit"`       // e.g. "2GiB", "2G", "512MiB", "512M"
	MemoryReservation string `json:"memory_reservation"` // e.g. "512MiB"
	CPULimit          string `json:"cpu_limit"`          // e.g. "2", "2000m", "500m"
	CPUReservation    string `json:"cpu_reservation"`    // e.g. "500m", "100m"
}

// ParseMemoryBytes parses a human-readable memory string (e.g. "2GiB", "512M", "1024KiB", "1073741824") to bytes.
func ParseMemoryBytes(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "0" {
		return 0, nil
	}

	var multiplier float64 = 1
	numStr := s

	if strings.HasSuffix(numStr, "tib") {
		multiplier = 1024 * 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "tib")
	} else if strings.HasSuffix(numStr, "ti") {
		multiplier = 1024 * 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "ti")
	} else if strings.HasSuffix(numStr, "tb") {
		multiplier = 1024 * 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "tb")
	} else if strings.HasSuffix(numStr, "t") {
		multiplier = 1024 * 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "t")
	} else if strings.HasSuffix(numStr, "gib") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "gib")
	} else if strings.HasSuffix(numStr, "gi") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "gi")
	} else if strings.HasSuffix(numStr, "gb") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "gb")
	} else if strings.HasSuffix(numStr, "g") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "g")
	} else if strings.HasSuffix(numStr, "mib") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "mib")
	} else if strings.HasSuffix(numStr, "mi") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "mi")
	} else if strings.HasSuffix(numStr, "mb") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "mb")
	} else if strings.HasSuffix(numStr, "m") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(numStr, "m")
	} else if strings.HasSuffix(numStr, "kib") {
		multiplier = 1024
		numStr = strings.TrimSuffix(numStr, "kib")
	} else if strings.HasSuffix(numStr, "ki") {
		multiplier = 1024
		numStr = strings.TrimSuffix(numStr, "ki")
	} else if strings.HasSuffix(numStr, "kb") {
		multiplier = 1024
		numStr = strings.TrimSuffix(numStr, "kb")
	} else if strings.HasSuffix(numStr, "k") {
		multiplier = 1024
		numStr = strings.TrimSuffix(numStr, "k")
	} else if strings.HasSuffix(numStr, "b") {
		multiplier = 1
		numStr = strings.TrimSuffix(numStr, "b")
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory format: %s", s)
	}
	if val < 0 {
		return 0, fmt.Errorf("memory cannot be negative: %s", s)
	}
	return int64(val * multiplier), nil
}

// ParseCPUNano parses a human-readable CPU string (e.g. "2", "2000m", "500m", "0.5") to nanoCPUs.
func ParseCPUNano(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "0" {
		return 0, nil
	}

	var multiplier float64 = 1e9 // default is cores/CPUs
	numStr := s

	if strings.HasSuffix(numStr, "n") {
		multiplier = 1
		numStr = strings.TrimSuffix(numStr, "n")
	} else if strings.HasSuffix(numStr, "u") || strings.HasSuffix(numStr, "µ") {
		multiplier = 1e3
		numStr = strings.TrimSuffix(strings.TrimSuffix(numStr, "u"), "µ")
	} else if strings.HasSuffix(numStr, "m") {
		multiplier = 1e6 // 1 millicore = 1,000,000 nanocores
		numStr = strings.TrimSuffix(numStr, "m")
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU format: %s", s)
	}
	if val < 0 {
		return 0, fmt.Errorf("CPU cannot be negative: %s", s)
	}
	return int64(val * multiplier), nil
}

// Usecase handles application-level orchestration logic for deployments.
type Usecase struct {
	repo deployment.Repository
}

// NewUsecase creates a new Usecase instance.
func NewUsecase(repo deployment.Repository) *Usecase {
	return &Usecase{repo: repo}
}

// ListDeployments retrieves all applications across registered clusters.
func (u *Usecase) ListDeployments(ctx context.Context) ([]deployment.Application, error) {
	return u.repo.List(ctx)
}

// ScaleDeployment modifies replica count of a deployment or swarm service.
func (u *Usecase) ScaleDeployment(ctx context.Context, targetType, targetCluster, namespace, name string, replicas int) error {
	if name == "" {
		return fmt.Errorf("application name cannot be empty")
	}
	if replicas < 0 || replicas > 100 {
		return fmt.Errorf("invalid replica count: must be between 0 and 100")
	}
	return u.repo.Scale(ctx, targetType, targetCluster, namespace, name, replicas)
}

// RestartDeployment triggers a rolling restart for a deployment or swarm service.
func (u *Usecase) RestartDeployment(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	if name == "" {
		return fmt.Errorf("application name cannot be empty")
	}
	return u.repo.Restart(ctx, targetType, targetCluster, namespace, name)
}

// DeleteDeployment deletes a deployment or swarm service.
func (u *Usecase) DeleteDeployment(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	if name == "" {
		return fmt.Errorf("application name cannot be empty")
	}
	return u.repo.Delete(ctx, targetType, targetCluster, namespace, name)
}

// CreateDeployment creates a deployment or swarm service.
func (u *Usecase) CreateDeployment(ctx context.Context, app deployment.Application) error {
	if app.Name == "" {
		return fmt.Errorf("application name cannot be empty")
	}
	if app.Image == "" {
		return fmt.Errorf("container image cannot be empty")
	}
	if app.Replicas < 0 || app.Replicas > 100 {
		return fmt.Errorf("invalid replica count: must be between 0 and 100")
	}
	if app.CPU == "" {
		app.CPU = "100m"
	}
	if app.Memory == "" {
		app.Memory = "128Mi"
	}
	if app.Port <= 0 {
		app.Port = 80
	}
	if app.NetType == "" {
		app.NetType = "ClusterIP"
	}
	if app.Team == "" {
		app.Team = "SRE"
	}
	if app.Env == "" {
		app.Env = "production"
	}
	return u.repo.Create(ctx, app)
}

// UpdateResources updates resource limits (CPU/RAM) and scaling for a deployment or swarm service.
func (u *Usecase) UpdateResources(ctx context.Context, req UpdateResourcesRequest) error {
	if req.Name == "" {
		return fmt.Errorf("application name cannot be empty")
	}
	if req.Replicas < -1 || req.Replicas > 100 {
		return fmt.Errorf("invalid replica count: must be between 0 and 100")
	}

	memLimitBytes, err := ParseMemoryBytes(req.MemoryLimit)
	if err != nil {
		return fmt.Errorf("invalid memory limit: %w", err)
	}

	memReservBytes, err := ParseMemoryBytes(req.MemoryReservation)
	if err != nil {
		return fmt.Errorf("invalid memory reservation: %w", err)
	}

	nanoCPUs, err := ParseCPUNano(req.CPULimit)
	if err != nil {
		return fmt.Errorf("invalid cpu limit: %w", err)
	}

	return u.repo.UpdateResources(ctx, req.Type, req.Cluster, req.Namespace, req.Name, memLimitBytes, memReservBytes, nanoCPUs, req.Replicas)
}


