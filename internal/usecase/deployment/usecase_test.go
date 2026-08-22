package deployment

import (
	"context"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/deployment"
)

type mockDeploymentRepo struct {
	apps []deployment.Application

	lastUpdateTargetType        string
	lastUpdateTargetCluster     string
	lastUpdateNamespace         string
	lastUpdateName              string
	lastUpdateMemoryLimitBytes  int64
	lastUpdateMemoryReservBytes int64
	lastUpdateNanoCPUs          int64
	lastUpdateReplicas          int

	scaleErr   error
	restartErr error
	deleteErr  error
	createErr  error
	updateErr  error
}

func (m *mockDeploymentRepo) List(ctx context.Context) ([]deployment.Application, error) {
	return m.apps, nil
}

func (m *mockDeploymentRepo) Scale(ctx context.Context, targetType, targetCluster, namespace, name string, replicas int) error {
	if m.scaleErr != nil {
		return m.scaleErr
	}
	return nil
}

func (m *mockDeploymentRepo) Restart(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	if m.restartErr != nil {
		return m.restartErr
	}
	return nil
}

func (m *mockDeploymentRepo) Delete(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return nil
}

func (m *mockDeploymentRepo) Create(ctx context.Context, app deployment.Application) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.apps = append(m.apps, app)
	return nil
}

func (m *mockDeploymentRepo) UpdateResources(ctx context.Context, targetType, targetCluster, namespace, name string, memoryLimitBytes, memoryReservBytes, nanoCPUs int64, replicas int) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.lastUpdateTargetType = targetType
	m.lastUpdateTargetCluster = targetCluster
	m.lastUpdateNamespace = namespace
	m.lastUpdateName = name
	m.lastUpdateMemoryLimitBytes = memoryLimitBytes
	m.lastUpdateMemoryReservBytes = memoryReservBytes
	m.lastUpdateNanoCPUs = nanoCPUs
	m.lastUpdateReplicas = replicas
	return nil
}

func TestParseMemoryBytes(t *testing.T) {
	tests := []struct {
		input       string
		expected    int64
		expectError bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"512B", 512, false},
		{"1024", 1024, false},
		{"1K", 1024, false},
		{"1KB", 1024, false},
		{"1Ki", 1024, false},
		{"1KiB", 1024, false},
		{"512M", 512 * 1024 * 1024, false},
		{"512MB", 512 * 1024 * 1024, false},
		{"512Mi", 512 * 1024 * 1024, false},
		{"512MiB", 512 * 1024 * 1024, false},
		{"2G", 2 * 1024 * 1024 * 1024, false},
		{"2GB", 2 * 1024 * 1024 * 1024, false},
		{"2Gi", 2 * 1024 * 1024 * 1024, false},
		{"2GiB", 2 * 1024 * 1024 * 1024, false},
		{"1.5G", int64(1.5 * 1024 * 1024 * 1024), false},
		{"1T", 1024 * 1024 * 1024 * 1024, false},
		{"1TiB", 1024 * 1024 * 1024 * 1024, false},
		{"invalid", 0, true},
		{"-512M", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			res, err := ParseMemoryBytes(tt.input)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for %q: %v", tt.input, err)
				}
				if res != tt.expected {
					t.Errorf("ParseMemoryBytes(%q) = %d; want %d", tt.input, res, tt.expected)
				}
			}
		})
	}
}

func TestParseCPUNano(t *testing.T) {
	tests := []struct {
		input       string
		expected    int64
		expectError bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"1", 1000000000, false},
		{"2", 2000000000, false},
		{"0.5", 500000000, false},
		{"1.5", 1500000000, false},
		{"100m", 100000000, false},
		{"500m", 500000000, false},
		{"2000m", 2000000000, false},
		{"500000u", 500000000, false},
		{"500000000n", 500000000, false},
		{"invalid", 0, true},
		{"-1", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			res, err := ParseCPUNano(tt.input)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for %q: %v", tt.input, err)
				}
				if res != tt.expected {
					t.Errorf("ParseCPUNano(%q) = %d; want %d", tt.input, res, tt.expected)
				}
			}
		})
	}
}

func TestUsecase_UpdateResources(t *testing.T) {
	ctx := context.Background()

	t.Run("success kubernetes resource update", func(t *testing.T) {
		repo := &mockDeploymentRepo{}
		uc := NewUsecase(repo)

		req := UpdateResourcesRequest{
			Type:              "kubernetes",
			Cluster:           "prod-k8s",
			Namespace:         "default",
			Name:              "web-app",
			Replicas:          3,
			MemoryLimit:       "2GiB",
			MemoryReservation: "512MiB",
			CPULimit:          "2000m",
			CPUReservation:    "500m",
		}

		err := uc.UpdateResources(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if repo.lastUpdateName != "web-app" {
			t.Errorf("expected name web-app, got %s", repo.lastUpdateName)
		}
		if repo.lastUpdateReplicas != 3 {
			t.Errorf("expected replicas 3, got %d", repo.lastUpdateReplicas)
		}
		if repo.lastUpdateMemoryLimitBytes != 2*1024*1024*1024 {
			t.Errorf("expected memory limit %d, got %d", 2*1024*1024*1024, repo.lastUpdateMemoryLimitBytes)
		}
		if repo.lastUpdateMemoryReservBytes != 512*1024*1024 {
			t.Errorf("expected memory reserv %d, got %d", 512*1024*1024, repo.lastUpdateMemoryReservBytes)
		}
		if repo.lastUpdateNanoCPUs != 2000000000 {
			t.Errorf("expected nanoCPUs %d, got %d", 2000000000, repo.lastUpdateNanoCPUs)
		}
	})

	t.Run("empty name returns error", func(t *testing.T) {
		repo := &mockDeploymentRepo{}
		uc := NewUsecase(repo)

		err := uc.UpdateResources(ctx, UpdateResourcesRequest{
			Name: "",
		})
		if err == nil {
			t.Fatal("expected error for empty name, got nil")
		}
	})

	t.Run("invalid replicas returns error", func(t *testing.T) {
		repo := &mockDeploymentRepo{}
		uc := NewUsecase(repo)

		err := uc.UpdateResources(ctx, UpdateResourcesRequest{
			Name:     "web-app",
			Replicas: 101,
		})
		if err == nil {
			t.Fatal("expected error for replicas > 100, got nil")
		}

		err = uc.UpdateResources(ctx, UpdateResourcesRequest{
			Name:     "web-app",
			Replicas: -2,
		})
		if err == nil {
			t.Fatal("expected error for replicas < -1, got nil")
		}
	})

	t.Run("invalid memory limit returns error", func(t *testing.T) {
		repo := &mockDeploymentRepo{}
		uc := NewUsecase(repo)

		err := uc.UpdateResources(ctx, UpdateResourcesRequest{
			Name:        "web-app",
			MemoryLimit: "invalid-mem",
		})
		if err == nil {
			t.Fatal("expected error for invalid memory limit, got nil")
		}
	})

	t.Run("invalid cpu limit returns error", func(t *testing.T) {
		repo := &mockDeploymentRepo{}
		uc := NewUsecase(repo)

		err := uc.UpdateResources(ctx, UpdateResourcesRequest{
			Name:     "web-app",
			CPULimit: "invalid-cpu",
		})
		if err == nil {
			t.Fatal("expected error for invalid cpu limit, got nil")
		}
	})
}

func TestUsecase_OtherOperations(t *testing.T) {
	ctx := context.Background()
	repo := &mockDeploymentRepo{
		apps: []deployment.Application{
			{Name: "app1", Replicas: 2},
		},
	}
	uc := NewUsecase(repo)

	// List
	apps, err := uc.ListDeployments(ctx)
	if err != nil || len(apps) != 1 {
		t.Fatalf("ListDeployments failed: %v", err)
	}

	// Scale
	if err := uc.ScaleDeployment(ctx, "kubernetes", "c1", "ns", "app1", 5); err != nil {
		t.Fatalf("ScaleDeployment failed: %v", err)
	}
	if err := uc.ScaleDeployment(ctx, "kubernetes", "c1", "ns", "", 5); err == nil {
		t.Fatal("expected error for empty name in ScaleDeployment")
	}
	if err := uc.ScaleDeployment(ctx, "kubernetes", "c1", "ns", "app1", 105); err == nil {
		t.Fatal("expected error for replicas > 100")
	}

	// Restart
	if err := uc.RestartDeployment(ctx, "kubernetes", "c1", "ns", "app1"); err != nil {
		t.Fatalf("RestartDeployment failed: %v", err)
	}
	if err := uc.RestartDeployment(ctx, "kubernetes", "c1", "ns", ""); err == nil {
		t.Fatal("expected error for empty name in RestartDeployment")
	}

	// Delete
	if err := uc.DeleteDeployment(ctx, "kubernetes", "c1", "ns", "app1"); err != nil {
		t.Fatalf("DeleteDeployment failed: %v", err)
	}
	if err := uc.DeleteDeployment(ctx, "kubernetes", "c1", "ns", ""); err == nil {
		t.Fatal("expected error for empty name in DeleteDeployment")
	}

	// Create
	newApp := deployment.Application{
		Name:     "app2",
		Image:    "nginx:latest",
		Replicas: 1,
	}
	if err := uc.CreateDeployment(ctx, newApp); err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}
	if err := uc.CreateDeployment(ctx, deployment.Application{Name: ""}); err == nil {
		t.Fatal("expected error for empty name in CreateDeployment")
	}
	if err := uc.CreateDeployment(ctx, deployment.Application{Name: "app2", Image: ""}); err == nil {
		t.Fatal("expected error for empty image in CreateDeployment")
	}
}
