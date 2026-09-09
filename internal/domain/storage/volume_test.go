package storage

import (
	"encoding/json"
	"testing"
)

func TestStorageDomain_IsHealthy(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{
			name:     "Healthy status",
			status:   HealthStatusHealthy,
			expected: true,
		},
		{
			name:     "Degraded status",
			status:   HealthStatusDegraded,
			expected: false,
		},
		{
			name:     "Faulted status",
			status:   HealthStatusFaulted,
			expected: false,
		},
		{
			name:     "Empty status",
			status:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &DistributedVolume{
				Name:         "vol-1",
				HealthStatus: tt.status,
			}
			if got := v.IsHealthy(); got != tt.expected {
				t.Errorf("IsHealthy() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStorageDomain_Validate(t *testing.T) {
	validVol := &DistributedVolume{
		Name:         "pvc-mysql",
		Namespace:    "default",
		StorageClass: "longhorn",
	}
	if err := validVol.Validate(); err != nil {
		t.Fatalf("expected valid volume, got error: %v", err)
	}

	emptyVol := &DistributedVolume{
		Name: "",
	}
	if err := emptyVol.Validate(); err != ErrEmptyVolumeName {
		t.Fatalf("expected ErrEmptyVolumeName, got: %v", err)
	}
}

func TestStorageDomain_VolumeExpandRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		size    int64
		wantErr bool
	}{
		{
			name:    "positive size (10Gi)",
			size:    10 * 1024 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "zero size",
			size:    0,
			wantErr: true,
		},
		{
			name:    "negative size",
			size:    -1024,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := VolumeExpandRequest{NewSizeBytes: tt.size}
			err := req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStorageDomain_JSONSerialization(t *testing.T) {
	vol := DistributedVolume{
		Name:             "data-volume",
		Namespace:        "prod",
		StorageClass:     "longhorn",
		CapacityBytes:    10737418240,
		UsedBytes:        3758096384,
		ReplicationCount: 3,
		HealthStatus:     HealthStatusHealthy,
		AttachedNode:     "node-worker-1",
		Replicas: []VolumeReplica{
			{
				NodeID:   "uid-1",
				NodeName: "node-worker-1",
				Mode:     "RW",
				Size:     10737418240,
			},
			{
				NodeID:   "uid-2",
				NodeName: "node-worker-2",
				Mode:     "RW",
				Size:     10737418240,
			},
			{
				NodeID:   "uid-3",
				NodeName: "node-worker-3",
				Mode:     "RW",
				Size:     10737418240,
			},
		},
	}

	data, err := json.Marshal(vol)
	if err != nil {
		t.Fatalf("failed to marshal volume: %v", err)
	}

	var decoded DistributedVolume
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal volume: %v", err)
	}

	if decoded.Name != vol.Name || decoded.ReplicationCount != 3 || len(decoded.Replicas) != 3 {
		t.Errorf("decoded volume mismatch: %+v", decoded)
	}
	if !decoded.IsHealthy() {
		t.Errorf("expected decoded volume to be healthy")
	}
}

func TestStorageDomain_SnapshotResult_JSON(t *testing.T) {
	res := VolumeSnapshotResult{
		SnapshotName: "pvc-snap-12345",
		VolumeName:   "pvc-data",
		Status:       SnapshotStatusReady,
		CreatedAt:    "2026-09-03T18:00:00Z",
	}

	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("failed to marshal snapshot result: %v", err)
	}

	var decoded VolumeSnapshotResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal snapshot result: %v", err)
	}

	if decoded.SnapshotName != "pvc-snap-12345" || decoded.Status != "Ready" {
		t.Errorf("decoded snapshot result mismatch: %+v", decoded)
	}
}
