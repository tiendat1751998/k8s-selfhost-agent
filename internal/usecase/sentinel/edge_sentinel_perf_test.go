package sentinel

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestEdgeSentinel_PERF05_ConcurrentShards(t *testing.T) {
	executor := &mockExecutor{}
	listener := &mockListener{}
	s := NewEdgeSentinel(executor, listener)

	const (
		numGoroutines = 20
		numNodes      = 100
		iterations    = 50
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				nodeID := fmt.Sprintf("node-%d", (gid*iterations+i)%numNodes)
				snap := NodeHardwareSnapshot{
					NodeID:            nodeID,
					DiskUsagePct:      float64(50 + (i % 45)),
					CgroupMemUsagePct: float64(40 + (i % 50)),
					Timestamp:         time.Now(),
				}
				_, err := s.EvaluateSnapshot(context.Background(), snap)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		}(g)
	}

	wg.Wait()

	if s.TickCount() != uint64(numGoroutines*iterations) {
		t.Fatalf("expected %d ticks, got %d", numGoroutines*iterations, s.TickCount())
	}
}
