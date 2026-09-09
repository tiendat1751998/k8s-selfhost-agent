package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/datdt/k8sselfhost/internal/domain/deployment"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

func (r *deploymentRepo) scaleSwarmService(ctx context.Context, targetCluster, name string, replicas int) error {
	repo, err := r.getDockerClient(ctx, targetCluster)
	if err != nil {
		return err
	}
	return repo.ScaleService(ctx, name, replicas)
}

func (r *deploymentRepo) restartSwarmService(ctx context.Context, targetCluster, name string) error {
	repo, err := r.getDockerClient(ctx, targetCluster)
	if err != nil {
		return err
	}
	return repo.RestartService(ctx, name)
}

func (r *deploymentRepo) deleteSwarmService(ctx context.Context, targetCluster, name string) error {
	repo, err := r.getDockerClient(ctx, targetCluster)
	if err != nil {
		return err
	}
	return repo.DeleteService(ctx, name)
}

func (r *deploymentRepo) createSwarmService(ctx context.Context, app deployment.Application) error {
	repo, err := r.getDockerClient(ctx, app.Target)
	if err != nil {
		return err
	}
	return repo.CreateService(ctx, app.Name, app.Image, app.Replicas, app.Port)
}

func (r *deploymentRepo) updateSwarmResources(ctx context.Context, targetCluster, name string, memoryLimitBytes, memoryReservBytes, nanoCPUs int64, replicas int) error {
	repo, err := r.getDockerClient(ctx, targetCluster)
	if err != nil {
		return err
	}
	return repo.UpdateServiceResources(ctx, name, memoryLimitBytes, memoryReservBytes, nanoCPUs, replicas)
}

func (r *deploymentRepo) mapSwarmService(s domainDocker.Service, clusterName string) deployment.Application {
	status := "healthy"
	if s.Replicas == 0 {
		status = "down"
	}

	team := "SRE"
	env := "production"
	if strings.Contains(s.Name, "prod") {
		env = "production"
	} else if strings.Contains(s.Name, "stg") || strings.Contains(s.Name, "staging") {
		env = "staging"
		team = "Dev-Auth"
	} else if strings.Contains(s.Name, "worker") || strings.Contains(s.Name, "payment") {
		team = "Finance"
	}

	port := 80
	if len(s.Ports) > 0 {
		parts := strings.Split(s.Ports[0], ":")
		if len(parts) >= 2 {
			_, _ = fmt.Sscanf(parts[0], "%d", &port)
		}
	}

	memLimitStr := "1GiB"
	if s.MemoryLimitBytes > 0 {
		if s.MemoryLimitBytes%(1024*1024*1024) == 0 {
			memLimitStr = fmt.Sprintf("%dGiB", s.MemoryLimitBytes/(1024*1024*1024))
		} else if s.MemoryLimitBytes%(1024*1024) == 0 {
			memLimitStr = fmt.Sprintf("%dMiB", s.MemoryLimitBytes/(1024*1024))
		} else {
			memLimitStr = fmt.Sprintf("%dMiB", s.MemoryLimitBytes/(1024*1024))
		}
	}

	memReservStr := "256MiB"
	if s.MemoryReservBytes > 0 {
		if s.MemoryReservBytes%(1024*1024*1024) == 0 {
			memReservStr = fmt.Sprintf("%dGiB", s.MemoryReservBytes/(1024*1024*1024))
		} else if s.MemoryReservBytes%(1024*1024) == 0 {
			memReservStr = fmt.Sprintf("%dMiB", s.MemoryReservBytes/(1024*1024))
		} else {
			memReservStr = fmt.Sprintf("%dMiB", s.MemoryReservBytes/(1024*1024))
		}
	}

	cpuLimitStr := "1 Core"
	if s.NanoCPUs > 0 {
		if s.NanoCPUs%1000000000 == 0 {
			cores := s.NanoCPUs / 1000000000
			if cores == 1 {
				cpuLimitStr = "1 Core"
			} else {
				cpuLimitStr = fmt.Sprintf("%d Cores", cores)
			}
		} else {
			cpuLimitStr = fmt.Sprintf("%dm", s.NanoCPUs/1000000)
		}
	}

	return deployment.Application{
		Name:              s.Name,
		Team:              team,
		Env:               env,
		Image:             s.Image,
		Target:            clusterName,
		Namespace:         "",
		Type:              "swarm",
		Replicas:          s.Replicas,
		Status:            status,
		CPU:               cpuLimitStr,
		Memory:            memLimitStr,
		CPULimit:          cpuLimitStr,
		CPUReservation:    "250m",
		MemoryLimit:       memLimitStr,
		MemoryReservation: memReservStr,
		Port:              port,
		NetType:           "NodePort",
		Volume:            "none",
	}
}
