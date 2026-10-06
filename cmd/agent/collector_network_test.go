package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectNetwork_ExcludeVirtualInterfaces(t *testing.T) {
	tempDir := t.TempDir()
	netDir := filepath.Join(tempDir, "net")
	if err := os.MkdirAll(netDir, 0755); err != nil {
		t.Fatalf("failed to create net dir: %v", err)
	}
	devFile := filepath.Join(netDir, "dev")

	initialDev := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 10000000    100    0    0    0     0          0         0  5000000     50    0    0    0     0       0          0
enp3s0: 20000000    200    0    0    0     0          0         0 10000000    100    0    0    0     0       0          0
    lo: 50000000    500    0    0    0     0          0         0 50000000    500    0    0    0     0       0          0
docker0: 30000000   300    0    0    0     0          0         0 30000000    300    0    0    0     0       0          0
veth123: 15000000   150    0    0    0     0          0         0 15000000    150    0    0    0     0       0          0
br-abc0: 12000000   120    0    0    0     0          0         0 12000000    120    0    0    0     0       0          0
  cni0: 11000000    110    0    0    0     0          0         0 11000000    110    0    0    0     0       0          0
`
	if err := os.WriteFile(devFile, []byte(initialDev), 0644); err != nil {
		t.Fatalf("failed to write initial net/dev: %v", err)
	}

	collector := NewSystemCollector(tempDir, "", nil)

	// Baseline pass
	now := time.Now().UTC()
	collector.collectNetwork(now)

	// Updated pass after 1 second:
	// eth0: +1,000,000 rx, +500,000 tx
	// enp3s0: +2,000,000 rx, +1,000,000 tx
	// lo: +9,999,999 rx, +9,999,999 tx (should be excluded)
	// docker0: +8,888,888 rx, +8,888,888 tx (should be excluded)
	// veth123: +7,777,777 rx, +7,777,777 tx (should be excluded)
	// br-abc0: +6,666,666 rx, +6,666,666 tx (should be excluded)
	// cni0: +5,555,555 rx, +5,555,555 tx (should be excluded)
	updatedDev := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 11000000    110    0    0    0     0          0         0  5500000     55    0    0    0     0       0          0
enp3s0: 22000000    220    0    0    0     0          0         0 11000000    110    0    0    0     0       0          0
    lo: 59999999    600    0    0    0     0          0         0 59999999    600    0    0    0     0       0          0
docker0: 38888888   388    0    0    0     0          0         0 38888888    388    0    0    0     0       0          0
veth123: 22777777   227    0    0    0     0          0         0 22777777    227    0    0    0     0       0          0
br-abc0: 18666666   186    0    0    0     0          0         0 18666666    186    0    0    0     0       0          0
  cni0: 16555555    165    0    0    0     0          0         0 16555555    165    0    0    0     0       0          0
`
	if err := os.WriteFile(devFile, []byte(updatedDev), 0644); err != nil {
		t.Fatalf("failed to write updated net/dev: %v", err)
	}

	metrics := collector.collectNetwork(now.Add(1 * time.Second))

	expectedRx := int64(3000000) // 1,000,000 (eth0) + 2,000,000 (enp3s0)
	expectedTx := int64(1500000) // 500,000 (eth0) + 1,000,000 (enp3s0)

	if metrics.TotalRxBytesPerSec != expectedRx {
		t.Errorf("expected TotalRxBytesPerSec = %d, got %d (virtual interfaces not excluded)", expectedRx, metrics.TotalRxBytesPerSec)
	}
	if metrics.TotalTxBytesPerSec != expectedTx {
		t.Errorf("expected TotalTxBytesPerSec = %d, got %d (virtual interfaces not excluded)", expectedTx, metrics.TotalTxBytesPerSec)
	}
}
