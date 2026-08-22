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

// SwarmTokens contains cluster join tokens and manager connection address.
type SwarmTokens struct {
	WorkerToken  string `json:"worker_token"`
	ManagerToken string `json:"manager_token"`
	ManagerAddr  string `json:"manager_addr"`
}

// SwarmInfo contains high-level Docker Swarm cluster status and node metrics.
type SwarmInfo struct {
	ID           string    `json:"id"`
	NodeCount    int       `json:"node_count"`
	ManagerCount int       `json:"manager_count"`
	WorkerCount  int       `json:"worker_count"`
	CreatedAt    time.Time `json:"created_at"`
	IsManager    bool      `json:"is_manager"`
}

// NodeDetails contains comprehensive system and hardware specifications for a swarm node.
type NodeDetails struct {
	ID            string            `json:"id"`
	Hostname      string            `json:"hostname"`
	Role          string            `json:"role"`         // manager, worker
	Availability  string            `json:"availability"` // active, pause, drain
	Status        string            `json:"status"`       // ready, down, disconnected
	EngineVersion string            `json:"engine_version"`
	OS            string            `json:"os"`
	Architecture  string            `json:"architecture"`
	CPUs          int64             `json:"cpus"`         // NanoCPUs
	Memory        int64             `json:"memory"`       // bytes
	IP            string            `json:"ip"`
	Labels        map[string]string `json:"labels"`
	JoinedAt      time.Time         `json:"joined_at"`
}

// ComputeHost represents an external compute host (k8s-agent, Docker Engine, or Kubernetes) registered in the multi-host registry.
type ComputeHost struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	HostType        string            `json:"host_type"` // agent, docker, k8s, prometheus, git, database, custom
	Endpoint        string            `json:"endpoint"`  // http://ip:9100, tcp://ip:port, or unix:///var/run/docker.sock
	TLSEnabled      bool              `json:"tls_enabled"`
	TLSCA           string            `json:"tls_ca,omitempty"`
	TLSCert         string            `json:"tls_cert,omitempty"`
	TLSKey          string            `json:"tls_key,omitempty"` // encrypted at rest with crypto.Encrypt
	APIVersion      string            `json:"api_version,omitempty"`
	Status          string            `json:"status"` // connected, disconnected, pending, error
	LastHealthCheck *time.Time        `json:"last_health_check,omitempty"`
	Labels          map[string]string `json:"labels"`
	TenantID        string            `json:"tenant_id"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// Repository defines data access for Docker and Swarm provider.
type Repository interface {
	ListContainers(ctx context.Context) ([]Container, error)
	ListNodes(ctx context.Context) ([]Node, error)
	ListServices(ctx context.Context) ([]Service, error)
	ScaleService(ctx context.Context, serviceID string, replicas int) error
	UpdateNodeAvailability(ctx context.Context, nodeID string, availability string) error
	ToggleContainer(ctx context.Context, containerID string, action string) error
	DeleteService(ctx context.Context, serviceID string) error
	RestartService(ctx context.Context, serviceID string) error
	CreateService(ctx context.Context, name string, image string, replicas int, port int) error
	GetLogs(ctx context.Context, targetID string, targetType string) (string, error)
	UpdateServiceImage(ctx context.Context, serviceID string, image string) error
	UpdateContainerImage(ctx context.Context, containerID string, image string) error
	UpdateServiceResources(ctx context.Context, serviceID string, memoryLimitBytes int64, memoryReservBytes int64, nanoCPUs int64) error

	GetSwarmJoinTokens(ctx context.Context) (*SwarmTokens, error)
	DrainNode(ctx context.Context, nodeID string) error
	ActivateNode(ctx context.Context, nodeID string) error
	RemoveNode(ctx context.Context, nodeID string, force bool) error
	GetNodeDetails(ctx context.Context, nodeID string) (*NodeDetails, error)
	GetSwarmInfo(ctx context.Context) (*SwarmInfo, error)
}

// ComputeHostRepository defines persistence operations for multi-host compute registries.
type ComputeHostRepository interface {
	Create(ctx context.Context, host *ComputeHost) error
	GetByID(ctx context.Context, id string) (*ComputeHost, error)
	List(ctx context.Context, tenantID string) ([]ComputeHost, error)
	ListAll(ctx context.Context) ([]ComputeHost, error)
	Update(ctx context.Context, host *ComputeHost) error
	Delete(ctx context.Context, id string) error
	UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error
}

