package ports

import (
	"context"
	"time"
)

// DockerContainer represents a standalone Docker container in ports.
type DockerContainer struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Image   string    `json:"image"`
	Status  string    `json:"status"`
	State   string    `json:"state"`
	Created time.Time `json:"created"`
}

// DockerNode represents a node in a Docker Swarm cluster in ports.
type DockerNode struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`         // manager or worker
	Availability string    `json:"availability"` // active, pause, drain
	Status       string    `json:"status"`       // ready, down
	Version      string    `json:"version"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DockerService represents a Docker Swarm service in ports.
type DockerService struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Replicas  int       `json:"replicas"`
	Ports     []string  `json:"ports"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DockerSandbox defines the port for Docker sandbox wrapping.
type DockerSandbox interface {
	ListContainers(ctx context.Context) ([]DockerContainer, error)
	ListNodes(ctx context.Context) ([]DockerNode, error)
	ListServices(ctx context.Context) ([]DockerService, error)
	ScaleService(ctx context.Context, serviceID string, replicas int) error
	UpdateNodeAvailability(ctx context.Context, nodeID string, availability string) error
	ToggleContainer(ctx context.Context, containerID string, action string) error
	DeleteService(ctx context.Context, serviceID string) error
	RestartService(ctx context.Context, serviceID string) error
	CreateService(ctx context.Context, name string, image string, replicas int, port int) error
	GetLogs(ctx context.Context, targetID string, targetType string) (string, error)
	UpdateServiceImage(ctx context.Context, serviceID string, image string) error
	UpdateContainerImage(ctx context.Context, containerID string, image string) error
}
