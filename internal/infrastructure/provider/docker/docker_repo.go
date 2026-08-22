package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerImage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
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

// NewDockerClient creates a new Docker client targeting the given host.
func NewDockerClient(host string, version string) (*client.Client, error) {
	opts := []client.Opt{client.WithHost(host)}
	if version != "" {
		opts = append(opts, client.WithVersion(version))
	} else {
		opts = append(opts, client.WithAPIVersionNegotiation())
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return cli, nil
}

// NewRealDockerRepo initializes a new Docker client targeting the given host.
// Example host: "tcp://10.10.10.133:2375"
func NewRealDockerRepo(host string, version string) (domainDocker.Repository, error) {
	cli, err := NewDockerClient(host, version)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("listing nodes from docker api: %w", err)
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

func (r *realDockerRepo) UpdateContainerImage(ctx context.Context, containerID string, targetImage string) error {
	inspectData, err := r.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("inspecting container %s: %w", containerID, err)
	}

	// Pull new image if possible (ignore pull errors if local image exists)
	reader, pullErr := r.cli.ImagePull(ctx, targetImage, dockerImage.PullOptions{})
	if pullErr == nil && reader != nil {
		_, _ = io.Copy(io.Discard, reader)
		_ = reader.Close()
	}

	name := strings.TrimPrefix(inspectData.Name, "/")
	config := inspectData.Config
	config.Image = targetImage

	hostConfig := inspectData.HostConfig
	var netConfig *network.NetworkingConfig
	if inspectData.NetworkSettings != nil && len(inspectData.NetworkSettings.Networks) > 0 {
		netConfig = &network.NetworkingConfig{
			EndpointsConfig: inspectData.NetworkSettings.Networks,
		}
	}

	// Stop old container (10s timeout)
	timeout := 10
	_ = r.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})

	// Remove old container
	if err := r.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("removing old container %s: %w", containerID, err)
	}

	// Create new container with identical configs and updated image
	createResp, err := r.cli.ContainerCreate(ctx, config, hostConfig, netConfig, nil, name)
	if err != nil {
		return fmt.Errorf("creating updated container %s with image %s: %w", name, targetImage, err)
	}

	// Start new container
	if err := r.cli.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting updated container %s: %w", createResp.ID, err)
	}

	return nil
}

func (r *realDockerRepo) GetSwarmJoinTokens(ctx context.Context) (*domainDocker.SwarmTokens, error) {
	swarmObj, err := r.cli.SwarmInspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspecting swarm: %w", err)
	}

	managerAddr := ""
	info, err := r.cli.Info(ctx)
	if err == nil {
		if len(info.Swarm.RemoteManagers) > 0 {
			managerAddr = info.Swarm.RemoteManagers[0].Addr
		} else if info.Swarm.NodeAddr != "" {
			managerAddr = info.Swarm.NodeAddr + ":2377"
		}
	}

	return &domainDocker.SwarmTokens{
		WorkerToken:  swarmObj.JoinTokens.Worker,
		ManagerToken: swarmObj.JoinTokens.Manager,
		ManagerAddr:  managerAddr,
	}, nil
}

func (r *realDockerRepo) DrainNode(ctx context.Context, nodeID string) error {
	return r.UpdateNodeAvailability(ctx, nodeID, string(swarm.NodeAvailabilityDrain))
}

func (r *realDockerRepo) ActivateNode(ctx context.Context, nodeID string) error {
	return r.UpdateNodeAvailability(ctx, nodeID, string(swarm.NodeAvailabilityActive))
}

func (r *realDockerRepo) RemoveNode(ctx context.Context, nodeID string, force bool) error {
	err := r.cli.NodeRemove(ctx, nodeID, types.NodeRemoveOptions{Force: force})
	if err != nil {
		return fmt.Errorf("removing swarm node %s: %w", nodeID, err)
	}
	return nil
}

func (r *realDockerRepo) GetNodeDetails(ctx context.Context, nodeID string) (*domainDocker.NodeDetails, error) {
	node, _, err := r.cli.NodeInspectWithRaw(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("inspecting swarm node %s: %w", nodeID, err)
	}

	labels := make(map[string]string)
	for k, v := range node.Spec.Labels {
		labels[k] = v
	}

	return &domainDocker.NodeDetails{
		ID:            node.ID,
		Hostname:      node.Description.Hostname,
		Role:          string(node.Spec.Role),
		Availability:  string(node.Spec.Availability),
		Status:        string(node.Status.State),
		EngineVersion: node.Description.Engine.EngineVersion,
		OS:            node.Description.Platform.OS,
		Architecture:  node.Description.Platform.Architecture,
		CPUs:          node.Description.Resources.NanoCPUs,
		Memory:        node.Description.Resources.MemoryBytes,
		IP:            node.Status.Addr,
		Labels:        labels,
		JoinedAt:      node.CreatedAt,
	}, nil
}

func (r *realDockerRepo) GetSwarmInfo(ctx context.Context) (*domainDocker.SwarmInfo, error) {
	swarmObj, err := r.cli.SwarmInspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspecting swarm: %w", err)
	}

	nodes, err := r.cli.NodeList(ctx, swarm.NodeListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes for swarm info: %w", err)
	}

	managerCount := 0
	workerCount := 0
	for _, n := range nodes {
		if n.Spec.Role == swarm.NodeRoleManager {
			managerCount++
		} else {
			workerCount++
		}
	}

	info, err := r.cli.Info(ctx)
	isManager := false
	if err == nil {
		isManager = info.Swarm.ControlAvailable
	}

	return &domainDocker.SwarmInfo{
		ID:           swarmObj.ID,
		NodeCount:    len(nodes),
		ManagerCount: managerCount,
		WorkerCount:  workerCount,
		CreatedAt:    swarmObj.CreatedAt,
		IsManager:    isManager,
	}, nil
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



