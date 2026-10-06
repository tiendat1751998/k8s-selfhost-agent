package metrics

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"go.uber.org/zap"
)

func TestCollector_ContainerStats_BoundedConcurrency(t *testing.T) {
	const totalContainers = 24
	var containers []container.Summary
	for i := 0; i < totalContainers; i++ {
		cid := fmt.Sprintf("c-%02d", i)
		containers = append(containers, container.Summary{
			ID:    cid,
			Names: []string{"/" + cid},
			Image: "test-app:latest",
			State: "running",
		})
	}

	var currentConcurrent int32
	var maxConcurrent int32

	mockDocker := &mockDockerAPI{
		info: system.Info{
			ID:   "engine-test",
			Name: "test-host",
		},
		containers: containers,
		onStats: func(ctx context.Context, containerID string) {
			cur := atomic.AddInt32(&currentConcurrent, 1)
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if cur <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, cur) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			atomic.AddInt32(&currentConcurrent, -1)
		},
	}

	collector := NewCollector(mockDocker, nil, nil, zap.NewNop())
	overview, err := collector.CollectOnce(context.Background())
	if err != nil {
		t.Fatalf("CollectOnce failed: %v", err)
	}

	if len(overview.Containers) != totalContainers {
		t.Fatalf("expected %d containers, got %d", totalContainers, len(overview.Containers))
	}

	maxObserved := atomic.LoadInt32(&maxConcurrent)
	if maxObserved > 8 {
		t.Errorf("expected max concurrent stats requests <= 8, but observed %d", maxObserved)
	}
	if maxObserved == 0 {
		t.Errorf("expected at least 1 concurrent stats request, got 0")
	}
}

func TestCollector_ContainerStats_NoGoroutineLeak(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	const totalContainers = 20
	var containers []container.Summary
	for i := 0; i < totalContainers; i++ {
		cid := fmt.Sprintf("c-leak-%02d", i)
		containers = append(containers, container.Summary{
			ID:    cid,
			Names: []string{"/" + cid},
			Image: "test-app:latest",
			State: "running",
		})
	}

	mockDocker := &mockDockerAPI{
		info: system.Info{
			ID:   "engine-test",
			Name: "test-host",
		},
		containers: containers,
		onStats: func(ctx context.Context, containerID string) {
			time.Sleep(5 * time.Millisecond)
		},
	}

	collector := NewCollector(mockDocker, nil, nil, zap.NewNop())
	for i := 0; i < 5; i++ {
		overview, err := collector.CollectOnce(context.Background())
		if err != nil {
			t.Fatalf("CollectOnce failed on iteration %d: %v", i, err)
		}
		if len(overview.Containers) != totalContainers {
			t.Fatalf("expected %d containers, got %d", totalContainers, len(overview.Containers))
		}
	}

	time.Sleep(50 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()

	if finalGoroutines-initialGoroutines > 10 {
		t.Errorf("potential goroutine leak: initial=%d, final=%d", initialGoroutines, finalGoroutines)
	}
}
