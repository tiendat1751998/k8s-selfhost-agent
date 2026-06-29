package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

// realDockerRepo implements the domain.Repository interface by directly communicating
// with a Docker Engine or Docker Swarm Manager API.
type realDockerRepo struct {
	cli *client.Client
}

// NewRealDockerRepo initializes a new Docker client targeting the given host.
// Example host: "tcp://10.10.10.133:2375"
func NewRealDockerRepo(host string, version string) (domainDocker.Repository, error) {
	if version == "" {
		version = "1.41"
	}
	cli, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithVersion(version),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &realDockerRepo{cli: cli}, nil
}

// NewDockerRepoWithClient initializes a new Docker repository using an existing client.
func NewDockerRepoWithClient(cli *client.Client) domainDocker.Repository {
	return &realDockerRepo{cli: cli}
}

func (r *realDockerRepo) ListContainers(ctx context.Context) ([]domainDocker.Container, error) {
	containers, err := r.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	var result []domainDocker.Container
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
		}
		result = append(result, domainDocker.Container{
			ID:      c.ID,
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Created: time.Unix(c.Created, 0),
		})
	}
	return result, nil
}

func (r *realDockerRepo) ListNodes(ctx context.Context) ([]domainDocker.Node, error) {
	// Attempt to list swarm nodes. If not in swarm mode, this returns an error.
	nodes, err := r.cli.NodeList(ctx, swarm.NodeListOptions{})
	if err != nil {
		// Just return empty nodes if not a swarm manager
		return []domainDocker.Node{}, nil
	}

	var result []domainDocker.Node
	for _, n := range nodes {
		role := string(n.Spec.Role)
		avail := string(n.Spec.Availability)
		status := string(n.Status.State)

		result = append(result, domainDocker.Node{
			ID:           n.ID,
			Name:         n.Description.Hostname,
			Role:         role,
			Availability: avail,
			Status:       status,
			Version:      n.Description.Engine.EngineVersion,
			UpdatedAt:    n.UpdatedAt,
		})
	}
	return result, nil
}

func (r *realDockerRepo) ListServices(ctx context.Context) ([]domainDocker.Service, error) {
	services, err := r.cli.ServiceList(ctx, swarm.ServiceListOptions{})
	if err != nil {
		return []domainDocker.Service{}, nil
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

		result = append(result, domainDocker.Service{
			ID:        s.ID,
			Name:      s.Spec.Name,
			Image:     s.Spec.TaskTemplate.ContainerSpec.Image,
			Replicas:  replicas,
			Ports:     ports,
			UpdatedAt: s.UpdatedAt,
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

func (r *realDockerRepo) UpdateNodeAvailability(ctx context.Context, nodeID string, availability string) error {
	node, _, err := r.cli.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("inspecting node: %w", err)
	}

	spec := node.Spec
	spec.Availability = swarm.NodeAvailability(availability)

	err = r.cli.NodeUpdate(ctx, nodeID, node.Version, spec)
	if err != nil {
		return fmt.Errorf("updating node availability: %w", err)
	}
	return nil
}

func (r *realDockerRepo) ToggleContainer(ctx context.Context, containerID string, action string) error {
	if action == "start" {
		err := r.cli.ContainerStart(ctx, containerID, container.StartOptions{})
		if err != nil {
			return fmt.Errorf("starting container: %w", err)
		}
	} else if action == "stop" {
		err := r.cli.ContainerStop(ctx, containerID, container.StopOptions{})
		if err != nil {
			return fmt.Errorf("stopping container: %w", err)
		}
	} else {
		return fmt.Errorf("unknown action: %s", action)
	}
	return nil
}

func (r *realDockerRepo) GetLogs(ctx context.Context, targetID string, targetType string) (string, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
	}

	var reader io.ReadCloser
	var err error

	if targetType == "service" {
		reader, err = r.cli.ServiceLogs(ctx, targetID, options)
	} else {
		reader, err = r.cli.ContainerLogs(ctx, targetID, options)
	}

	if err != nil {
		return "", fmt.Errorf("fetching logs: %w", err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	_, err = stdcopy.StdCopy(&buf, &buf, reader)
	if err != nil && err != io.EOF {
		buf.WriteString(fmt.Sprintf("\n[Warning: Log stream interrupted: %v]\n", err))
	}

	return buf.String(), nil
}
