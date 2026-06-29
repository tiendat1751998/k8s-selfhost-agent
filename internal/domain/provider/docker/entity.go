package docker

import (
	"context"
	"time"
)

// Container represents a standalone Docker container.
type Container struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Image   string    `json:"image"`
	Status  string    `json:"status"`
	State   string    `json:"state"`
	Created time.Time `json:"created"`
}

// Node represents a node in a Docker Swarm cluster.
type Node struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`         // manager or worker
	Availability string    `json:"availability"` // active, pause, drain
	Status       string    `json:"status"`       // ready, down
	Version      string    `json:"version"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Service represents a Docker Swarm service.
type Service struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Replicas  int       `json:"replicas"`
	Ports     []string  `json:"ports"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Repository defines data access for Docker and Swarm provider.
type Repository interface {
	ListContainers(ctx context.Context) ([]Container, error)
	ListNodes(ctx context.Context) ([]Node, error)
	ListServices(ctx context.Context) ([]Service, error)
	ScaleService(ctx context.Context, serviceID string, replicas int) error
	UpdateNodeAvailability(ctx context.Context, nodeID string, availability string) error
	ToggleContainer(ctx context.Context, containerID string, action string) error
	GetLogs(ctx context.Context, targetID string, targetType string) (string, error)
}
