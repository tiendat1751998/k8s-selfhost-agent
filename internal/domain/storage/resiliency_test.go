package storage

import (
	"errors"
	"sync"
	"testing"
	"time"
	"unsafe"
)

func TestCephOSDStatus_HealthAndRepair(t *testing.T) {
	healthy := CephOSDStatus{
		OSDID:         0,
		Host:          "storage-node-01",
		Status:        OSDStatusUp,
		RocksDBHealth: RocksDBHealthOK,
		UpdatedAt:     time.Now(),
	}
	if err := healthy.Validate(); err != nil {
		t.Fatalf("expected healthy OSD to be valid, got: %v", err)
	}
	if !healthy.IsHealthy() {
		t.Errorf("expected healthy.IsHealthy() to be true")
	}
	if healthy.RequiresRepair() {
		t.Errorf("expected healthy.RequiresRepair() to be false")
	}

	corrupted := CephOSDStatus{
		OSDID:         1,
		Host:          "storage-node-02",
		Status:        OSDStatusCrashLooping,
		RocksDBHealth: RocksDBHealthCorrupted,
		LastCrashLog:  "Corruption: bad entry in WAL",
		UpdatedAt:     time.Now(),
	}
	if err := corrupted.Validate(); err != nil {
		t.Fatalf("expected corrupted OSD to be valid struct, got: %v", err)
	}
	if corrupted.IsHealthy() {
		t.Errorf("expected corrupted.IsHealthy() to be false")
	}
	if !corrupted.RequiresRepair() {
		t.Errorf("expected corrupted.RequiresRepair() to be true")
	}

	// Validation guards
	invalidID := CephOSDStatus{OSDID: -1, Host: "node-1", Status: OSDStatusUp, RocksDBHealth: RocksDBHealthOK}
	if err := invalidID.Validate(); !errors.Is(err, ErrInvalidOSDID) {
		t.Errorf("expected ErrInvalidOSDID, got: %v", err)
	}

	invalidHost := CephOSDStatus{OSDID: 2, Host: "", Status: OSDStatusUp, RocksDBHealth: RocksDBHealthOK}
	if err := invalidHost.Validate(); err == nil {
		t.Errorf("expected validation error for empty Host")
	}

	invalidStatus := CephOSDStatus{OSDID: 3, Host: "node-3", Status: "UNKNOWN", RocksDBHealth: RocksDBHealthOK}
	if err := invalidStatus.Validate(); err == nil {
		t.Errorf("expected validation error for invalid Status")
	}
}

func TestMinIOScannerTuning_Validation(t *testing.T) {
	valid := MinIOScannerTuning{
		CurrentSpeed:     ScannerSpeedDefault,
		ActiveClientIOPS: 1200,
		HealQueueLength:  15,
		RecommendedSpeed: ScannerSpeedSlow,
		AutoTunedAt:      time.Now(),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid MinIOScannerTuning, got: %v", err)
	}

	invalidSpeed := MinIOScannerTuning{
		CurrentSpeed:     "turbo_boost",
		RecommendedSpeed: ScannerSpeedDefault,
	}
	if err := invalidSpeed.Validate(); err == nil {
		t.Errorf("expected validation error for invalid speed")
	}

	negativeIOPS := MinIOScannerTuning{
		CurrentSpeed:     ScannerSpeedDefault,
		RecommendedSpeed: ScannerSpeedDefault,
		ActiveClientIOPS: -10,
	}
	if err := negativeIOPS.Validate(); err == nil {
		t.Errorf("expected validation error for negative IOPS")
	}
}

func TestIOTracker_CacheLinePaddingAndConcurrency(t *testing.T) {
	tracker := NewIOTracker()

	// Verify struct size and padding
	size := unsafe.Sizeof(*tracker)
	if size < 64 {
		t.Fatalf("expected IOTracker size >= 64 bytes for cache line isolation, got %d", size)
	}

	tracker.RecordClientIOPS(2500)
	if got := tracker.GetClientIOPS(); got != 2500 {
		t.Errorf("expected IOPS 2500, got %d", got)
	}

	// Concurrency test: crawler routines vs client requests concurrently
	var wg sync.WaitGroup
	iterations := 500

	// Concurrent client requests
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				tracker.IncWaitingRequests()
				tracker.RecordClientIOPS(int64(j * 10))
				tracker.DecWaitingRequests()
			}
		}()
	}

	// Concurrent background crawler routines
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				tracker.IncCrawlerScans()
			}
		}()
	}

	wg.Wait()

	if tracker.GetCrawlerScans() != int64(4*iterations) {
		t.Errorf("expected %d crawler scans, got %d", 4*iterations, tracker.GetCrawlerScans())
	}
	if tracker.GetWaitingRequests() != 0 {
		t.Errorf("expected waiting requests to return to 0, got %d", tracker.GetWaitingRequests())
	}
}

func TestBoundedHealingQueue_BoundedCapacityAndDropMetrics(t *testing.T) {
	queue := NewBoundedHealingQueue(2)

	task1 := HealTask{ID: "task-1", ClusterID: "ceph-prod", Target: "osd.0", Priority: 1, CreatedAt: time.Now()}
	task2 := HealTask{ID: "task-2", ClusterID: "ceph-prod", Target: "osd.1", Priority: 2, CreatedAt: time.Now()}
	task3 := HealTask{ID: "task-3", ClusterID: "ceph-prod", Target: "osd.2", Priority: 3, CreatedAt: time.Now()}

	// Enqueue within capacity
	ok, err := queue.TryEnqueueOrDrop(task1)
	if !ok || err != nil {
		t.Fatalf("failed to enqueue task1: %v", err)
	}
	ok, err = queue.TryEnqueueOrDrop(task2)
	if !ok || err != nil {
		t.Fatalf("failed to enqueue task2: %v", err)
	}

	// Enqueue exceeding capacity -> must drop and throttle
	ok, err = queue.TryEnqueueOrDrop(task3)
	if ok || !errors.Is(err, ErrHealingQueueFull) {
		t.Fatalf("expected ErrHealingQueueFull on capacity overflow, got ok=%v, err=%v", ok, err)
	}

	metrics := queue.Metrics()
	if metrics.Capacity != 2 {
		t.Errorf("expected capacity 2, got %d", metrics.Capacity)
	}
	if metrics.Length != 2 {
		t.Errorf("expected length 2, got %d", metrics.Length)
	}
	if metrics.Enqueued != 2 {
		t.Errorf("expected enqueued 2, got %d", metrics.Enqueued)
	}
	if metrics.Dropped != 1 {
		t.Errorf("expected dropped 1, got %d", metrics.Dropped)
	}
	if metrics.Throttled != 1 {
		t.Errorf("expected throttled 1, got %d", metrics.Throttled)
	}

	// Dequeue
	d1, ok := queue.Dequeue()
	if !ok || d1.ID != "task-1" {
		t.Fatalf("expected task-1, got %v", d1)
	}
	d2, ok := queue.Dequeue()
	if !ok || d2.ID != "task-2" {
		t.Fatalf("expected task-2, got %v", d2)
	}
	_, ok = queue.Dequeue()
	if ok {
		t.Fatal("expected empty queue to return false")
	}

	metrics = queue.Metrics()
	if metrics.Length != 0 || metrics.Dequeued != 2 {
		t.Errorf("expected empty queue with dequeued=2, got %+v", metrics)
	}
}

func TestBoundedHealingQueue_Concurrency(t *testing.T) {
	queue := NewBoundedHealingQueue(100)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				task := HealTask{
					ID:        "concurrent-task",
					ClusterID: "c1",
					Target:    "target",
					CreatedAt: time.Now(),
				}
				_, _ = queue.TryEnqueueOrDrop(task)
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				queue.Dequeue()
				time.Sleep(100 * time.Microsecond)
			}
		}()
	}

	wg.Wait()
	metrics := queue.Metrics()
	if metrics.Enqueued == 0 {
		t.Errorf("expected enqueued > 0, got 0")
	}
}
