package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerImage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"

	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

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
	return r.GetLogsWithOptions(ctx, targetID, targetType, "100", "")
}

func (r *realDockerRepo) GetLogsWithOptions(ctx context.Context, targetID string, targetType string, tail string, since string) (string, error) {
	if tail == "all" || tail == "0" {
		tail = "all"
	} else if tail == "" {
		tail = "100"
	}
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
		Since:      since,
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


