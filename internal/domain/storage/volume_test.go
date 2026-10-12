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
	used := int64(3758096384)
	usagePct := 35.0
	vol := DistributedVolume{
		Name:             "data-volume",
		Namespace:        "prod",
		StorageClass:     "longhorn",
		CapacityBytes:    10737418240,
		UsedBytes:        &used,
		UsagePercentage:  &usagePct,
		UsageSource:      UsageSourceKubeletMetrics,
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
	if decoded.UsedBytes == nil || *decoded.UsedBytes != used {
		t.Errorf("expected used bytes %d, got %v", used, decoded.UsedBytes)
	}
	if !decoded.IsHealthy() {
		t.Errorf("expected decoded volume to be healthy")
	}
}

func TestStorageDomain_VolumeDetails(t *testing.T) {
	details := VolumeDetails{
		VolumeID:         "pvc-12345",
		PersistentVolume: "pv-12345",
		ClaimName:        "mysql-pvc",
		Namespace:        "default",
		StorageClassName: "standard",
		CapacityBytes:    100 * 1024 * 1024 * 1024,
		UsedBytes:        nil,
		UsagePercentage:  nil,
		UsageSource:      UsageSourceUnknown,
		AllowExpansion:   true,
		VolumeMode:       "Filesystem",
		AccessModes:      []string{"ReadWriteOnce"},
		Phase:            "Bound",
	}

	data, err := json.Marshal(details)
	if err != nil {
		t.Fatalf("failed to marshal volume details: %v", err)
	}

	var decoded VolumeDetails
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal volume details: %v", err)
	}

	if decoded.ClaimName != "mysql-pvc" || decoded.UsedBytes != nil || decoded.UsageSource != UsageSourceUnknown {
		t.Errorf("decoded details mismatch: %+v", decoded)
	}
}

func TestStorageDomain_Errors(t *testing.T) {
	shrinkErr := NewErrStorageShrinkUnsupported()
	if shrinkErr.Code != ErrCodeStorageShrinkUnsupported {
		t.Errorf("expected code %s, got %s", ErrCodeStorageShrinkUnsupported, shrinkErr.Code)
	}

	scErr := NewErrStorageClassNoExpansion("gp2")
	if scErr.Code != ErrCodeStorageClassNoExpansion {
		t.Errorf("expected code %s, got %s", ErrCodeStorageClassNoExpansion, scErr.Code)
	}

	lockErr := NewErrVolumeAttachmentLocked("pvc-db", "worker-node-1")
	if lockErr.Code != ErrCodeVolumeAttachmentLocked {
		t.Errorf("expected code %s, got %s", ErrCodeVolumeAttachmentLocked, lockErr.Code)
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
