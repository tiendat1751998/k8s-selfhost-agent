package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"

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


