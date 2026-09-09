package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"

	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

func (r *realDockerRepo) ListServices(ctx context.Context) ([]domainDocker.Service, error) {
	services, err := r.cli.ServiceList(ctx, swarm.ServiceListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing services from docker api: %w", err)
	}

	var result []domainDocker.Service
	for _, s := range services {
		replicas := 0
		if s.Spec.Mode.Replicated != nil && s.Spec.Mode.Replicated.Replicas != nil {
			replicas = int(*s.Spec.Mode.Replicated.Replicas)
		}

		var ports []string
		if s.Endpoint.Ports != nil {
			for _, p := range s.Endpoint.Ports {
				ports = append(ports, fmt.Sprintf("%d:%d", p.PublishedPort, p.TargetPort))
			}
		}

		var memLimit int64
		var memReserv int64
		var nanoCPUs int64
		if s.Spec.TaskTemplate.Resources != nil {
			if s.Spec.TaskTemplate.Resources.Limits != nil {
				memLimit = s.Spec.TaskTemplate.Resources.Limits.MemoryBytes
				nanoCPUs = s.Spec.TaskTemplate.Resources.Limits.NanoCPUs
			}
			if s.Spec.TaskTemplate.Resources.Reservations != nil {
				memReserv = s.Spec.TaskTemplate.Resources.Reservations.MemoryBytes
			}
		}

		result = append(result, domainDocker.Service{
			ID:                s.ID,
			Name:              s.Spec.Name,
			Image:             s.Spec.TaskTemplate.ContainerSpec.Image,
			Replicas:          replicas,
			Ports:             ports,
			MemoryLimitBytes:  memLimit,
			MemoryReservBytes: memReserv,
			NanoCPUs:          nanoCPUs,
			UpdatedAt:         s.UpdatedAt,
		})
	}
	return result, nil
}


func (r *realDockerRepo) ScaleService(ctx context.Context, serviceID string, replicas int) error {
	service, _, err := r.cli.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return fmt.Errorf("inspecting service: %w", err)
	}

	spec := service.Spec
	if spec.Mode.Replicated == nil {
		return fmt.Errorf("service is not in replicated mode")
	}

	targetReplicas := uint64(replicas)
	spec.Mode.Replicated.Replicas = &targetReplicas

	_, err = r.cli.ServiceUpdate(ctx, serviceID, service.Version, spec, types.ServiceUpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating service replicas: %w", err)
	}
	return nil
}


func (r *realDockerRepo) DeleteService(ctx context.Context, serviceID string) error {
	err := r.cli.ServiceRemove(ctx, serviceID)
	if err != nil {
		return fmt.Errorf("removing service: %w", err)
	}
	return nil
}


func (r *realDockerRepo) RestartService(ctx context.Context, serviceID string) error {
	service, _, err := r.cli.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return fmt.Errorf("inspecting service for restart: %w", err)
	}

	spec := service.Spec
	spec.TaskTemplate.ForceUpdate++

	_, err = r.cli.ServiceUpdate(ctx, serviceID, service.Version, spec, types.ServiceUpdateOptions{})
	if err != nil {
		return fmt.Errorf("triggering service restart: %w", err)
	}
	return nil
}


func (r *realDockerRepo) CreateService(ctx context.Context, name string, image string, replicas int, port int) error {
	replicasVal := uint64(replicas)
	spec := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: name,
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image: image,
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &replicasVal,
			},
		},
	}
	if port > 0 {
		spec.EndpointSpec = &swarm.EndpointSpec{
			Ports: []swarm.PortConfig{
				{
					Protocol:   swarm.PortConfigProtocolTCP,
					TargetPort: uint32(port),
				},
			},
		}
	}
	_, err := r.cli.ServiceCreate(ctx, spec, types.ServiceCreateOptions{})
	if err != nil {
		return fmt.Errorf("creating swarm service: %w", err)
	}
	return nil
}


func (r *realDockerRepo) UpdateServiceImage(ctx context.Context, serviceID string, image string) error {
	service, _, err := r.cli.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	if err != nil {
		// If service inspect fails, check if serviceID is a standalone container
		if _, inspectErr := r.cli.ContainerInspect(ctx, serviceID); inspectErr == nil {
			return r.UpdateContainerImage(ctx, serviceID, image)
		}
		return fmt.Errorf("inspecting service for image update: %w", err)
	}

	spec := service.Spec
	if spec.TaskTemplate.ContainerSpec == nil {
		spec.TaskTemplate.ContainerSpec = &swarm.ContainerSpec{}
	}
	spec.TaskTemplate.ContainerSpec.Image = image

	_, err = r.cli.ServiceUpdate(ctx, serviceID, service.Version, spec, types.ServiceUpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating service image: %w", err)
	}
	return nil
}


func (r *realDockerRepo) UpdateServiceResources(ctx context.Context, serviceID string, memoryLimitBytes int64, memoryReservBytes int64, nanoCPUs int64, replicas int) error {
	service, _, err := r.cli.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	if err == nil {
		spec := service.Spec
		if spec.TaskTemplate.Resources == nil {
			spec.TaskTemplate.Resources = &swarm.ResourceRequirements{}
		}

		if memoryLimitBytes > 0 || nanoCPUs > 0 {
			if spec.TaskTemplate.Resources.Limits == nil {
				spec.TaskTemplate.Resources.Limits = &swarm.Limit{}
			}
			if memoryLimitBytes > 0 {
				spec.TaskTemplate.Resources.Limits.MemoryBytes = memoryLimitBytes
			}
			if nanoCPUs > 0 {
				spec.TaskTemplate.Resources.Limits.NanoCPUs = nanoCPUs
			}
		}

		if memoryReservBytes > 0 {
			if spec.TaskTemplate.Resources.Reservations == nil {
				spec.TaskTemplate.Resources.Reservations = &swarm.Resources{}
			}
			spec.TaskTemplate.Resources.Reservations.MemoryBytes = memoryReservBytes
		}

		if replicas >= 0 && spec.Mode.Replicated != nil {
			targetReplicas := uint64(replicas)
			spec.Mode.Replicated.Replicas = &targetReplicas
		}

		_, err = r.cli.ServiceUpdate(ctx, serviceID, service.Version, spec, types.ServiceUpdateOptions{})
		if err != nil {
			return fmt.Errorf("updating swarm service %s resources: %w", serviceID, err)
		}
		return nil
	}

	// Fallback to standalone Docker container update
	updateResources := container.Resources{}
	if memoryLimitBytes > 0 {
		updateResources.Memory = memoryLimitBytes
	}
	if memoryReservBytes > 0 {
		updateResources.MemoryReservation = memoryReservBytes
	}
	if nanoCPUs > 0 {
		updateResources.NanoCPUs = nanoCPUs
	}

	_, updateErr := r.cli.ContainerUpdate(ctx, serviceID, container.UpdateConfig{Resources: updateResources})
	if updateErr != nil {
		return fmt.Errorf("updating workload %s resources (swarm error: %v, container error: %w)", serviceID, err, updateErr)
	}
	return nil
}


