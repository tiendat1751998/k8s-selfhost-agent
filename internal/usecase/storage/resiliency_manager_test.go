package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	domainstorage "github.com/datdt/k8sselfhost/internal/domain/storage"
	"go.uber.org/zap"
)

// In-memory Ceph Gateway for deterministic testing without external binaries
type testCephGateway struct {
	osds          map[string][]domainstorage.CephOSDStatus
	repairedOSDs  []int
	repairModes   []int
	failRepairErr error
}

func newTestCephGateway() *testCephGateway {
	return &testCephGateway{
		osds:         make(map[string][]domainstorage.CephOSDStatus),
		repairedOSDs: make([]int, 0),
		repairModes:  make([]int, 0),
	}
}

func (g *testCephGateway) GetOSDStatusList(ctx context.Context, clusterID string) ([]domainstorage.CephOSDStatus, error) {
	osds, exists := g.osds[clusterID]
	if !exists {
		return nil, domainstorage.ErrCephOSDNotFound
	}
	return osds, nil
}

func (g *testCephGateway) RepairOSDBlueStore(ctx context.Context, clusterID string, osdID int, walRecoveryMode int) error {
	if g.failRepairErr != nil {
		return g.failRepairErr
	}
	g.repairedOSDs = append(g.repairedOSDs, osdID)
	g.repairModes = append(g.repairModes, walRecoveryMode)

	// Update in-memory state
	osds := g.osds[clusterID]
	for i := range osds {
		if osds[i].OSDID == osdID {
			osds[i].Status = domainstorage.OSDStatusUp
			osds[i].RocksDBHealth = domainstorage.RocksDBHealthOK
			osds[i].LastCrashLog = ""
			osds[i].UpdatedAt = time.Now()
		}
	}
	g.osds[clusterID] = osds
	return nil
}

// In-memory MinIO Scanner Gateway for deterministic testing
type testMinIOGateway struct {
	speeds   map[string]string
	trackers map[string]*domainstorage.IOTracker
	queues   map[string]*domainstorage.BoundedHealingQueue
}

func newTestMinIOGateway() *testMinIOGateway {
	return &testMinIOGateway{
		speeds:   make(map[string]string),
		trackers: make(map[string]*domainstorage.IOTracker),
		queues:   make(map[string]*domainstorage.BoundedHealingQueue),
	}
}

func (g *testMinIOGateway) GetScannerSpeed(ctx context.Context, clusterID string) (string, error) {
	speed, ok := g.speeds[clusterID]
	if !ok {
		return domainstorage.ScannerSpeedDefault, nil
	}
	return speed, nil
}

func (g *testMinIOGateway) SetScannerSpeed(ctx context.Context, clusterID string, speed string) error {
	g.speeds[clusterID] = speed
	return nil
}

func (g *testMinIOGateway) GetIOTracker(clusterID string) *domainstorage.IOTracker {
	if _, ok := g.trackers[clusterID]; !ok {
		g.trackers[clusterID] = domainstorage.NewIOTracker()
	}
	return g.trackers[clusterID]
}

func (g *testMinIOGateway) GetHealQueue(clusterID string) *domainstorage.BoundedHealingQueue {
	if _, ok := g.queues[clusterID]; !ok {
		g.queues[clusterID] = domainstorage.NewBoundedHealingQueue(100)
	}
	return g.queues[clusterID]
}

// In-memory Database Recovery Inspector
type testDatabaseInspector struct {
	isInRecoveryMap map[string]bool
	queryErr        error
}

func newTestDatabaseInspector() *testDatabaseInspector {
	return &testDatabaseInspector{
		isInRecoveryMap: make(map[string]bool),
	}
}

func (i *testDatabaseInspector) CheckIsInRecovery(ctx context.Context, dbConnString string) (bool, error) {
	if i.queryErr != nil {
		return false, i.queryErr
	}
	inRecovery, exists := i.isInRecoveryMap[dbConnString]
	if !exists {
		return false, errors.New("connection failed: host unreachable")
	}
	return inRecovery, nil
}

func TestResiliencyManager_AuditCephCluster(t *testing.T) {
	cephGW := newTestCephGateway()
	cephGW.osds["ceph-prod"] = []domainstorage.CephOSDStatus{
		{
			OSDID:         0,
			Host:          "ceph-node-1",
			Status:        domainstorage.OSDStatusUp,
			RocksDBHealth: domainstorage.RocksDBHealthOK,
			UpdatedAt:     time.Now(),
		},
		{
			OSDID:         1,
			Host:          "ceph-node-2",
			Status:        domainstorage.OSDStatusCrashLooping,
			RocksDBHealth: domainstorage.RocksDBHealthOK, // Will be flagged as CORRUPTED due to crash log
			LastCrashLog:  "Corruption: bad entry in WAL",
			UpdatedAt:     time.Now(),
		},
	}

	manager := NewResiliencyManager(
		zap.NewNop(),
		WithCephGateway(cephGW),
		WithMinIOGateway(newTestMinIOGateway()),
		WithDatabaseRecoveryInspector(newTestDatabaseInspector()),
	)

	// Valid cluster audit
	statuses, err := manager.AuditCephCluster(context.Background(), "ceph-prod")
	if err != nil {
		t.Fatalf("AuditCephCluster failed: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 OSD statuses, got %d", len(statuses))
	}

	// OSD 1 should be detected as CORRUPTED RocksDB
	if statuses[1].RocksDBHealth != domainstorage.RocksDBHealthCorrupted {
		t.Errorf("expected OSD 1 RocksDBHealth to be CORRUPTED, got %s", statuses[1].RocksDBHealth)
	}

	// Empty cluster ID guard
	_, err = manager.AuditCephCluster(context.Background(), "")
	if !errors.Is(err, domainstorage.ErrInvalidClusterID) {
		t.Errorf("expected ErrInvalidClusterID, got %v", err)
	}
}

func TestResiliencyManager_TriggerCephOSDRepair(t *testing.T) {
	cephGW := newTestCephGateway()
	cephGW.osds["ceph-prod"] = []domainstorage.CephOSDStatus{
		{
			OSDID:         3,
			Host:          "ceph-node-3",
			Status:        domainstorage.OSDStatusCrashLooping,
			RocksDBHealth: domainstorage.RocksDBHealthCorrupted,
			LastCrashLog:  "Corruption: bad entry in WAL",
			UpdatedAt:     time.Now(),
		},
	}

	manager := NewResiliencyManager(
		zap.NewNop(),
		WithCephGateway(cephGW),
		WithMinIOGateway(newTestMinIOGateway()),
		WithDatabaseRecoveryInspector(newTestDatabaseInspector()),
	)

	// Trigger repair on corrupted OSD 3
	err := manager.TriggerCephOSDRepair(context.Background(), "ceph-prod", 3)
	if err != nil {
		t.Fatalf("TriggerCephOSDRepair failed: %v", err)
	}

	if len(cephGW.repairedOSDs) != 1 || cephGW.repairedOSDs[0] != 3 {
		t.Errorf("expected OSD 3 to be repaired, got %v", cephGW.repairedOSDs)
	}
	if len(cephGW.repairModes) != 1 || cephGW.repairModes[0] != 2 {
		t.Errorf("expected walRecoveryMode 2 (PointInTimeRecovery), got %v", cephGW.repairModes)
	}

	// Repair non-existent OSD returns ErrCephOSDNotFound
	err = manager.TriggerCephOSDRepair(context.Background(), "ceph-prod", 999)
	if !errors.Is(err, domainstorage.ErrCephOSDNotFound) {
		t.Errorf("expected ErrCephOSDNotFound, got %v", err)
	}

	// Negative OSD ID guard
	err = manager.TriggerCephOSDRepair(context.Background(), "ceph-prod", -1)
	if !errors.Is(err, domainstorage.ErrInvalidOSDID) {
		t.Errorf("expected ErrInvalidOSDID, got %v", err)
	}
}

func TestResiliencyManager_AutoTuneMinIOScanner(t *testing.T) {
	minioGW := newTestMinIOGateway()
	manager := NewResiliencyManager(
		zap.NewNop(),
		WithCephGateway(newTestCephGateway()),
		WithMinIOGateway(minioGW),
		WithDatabaseRecoveryInspector(newTestDatabaseInspector()),
	)

	// Scenario 1: High client IOPS (> 2000) -> must throttle to "slowest" to prevent I/O lock starvation
	tracker := minioGW.GetIOTracker("minio-cluster-1")
	tracker.RecordClientIOPS(2500)
	tuning, err := manager.AutoTuneMinIOScanner(context.Background(), "minio-cluster-1")
	if err != nil {
		t.Fatalf("AutoTuneMinIOScanner failed: %v", err)
	}
	if tuning.RecommendedSpeed != domainstorage.ScannerSpeedSlowest {
		t.Errorf("expected slowest under high IOPS, got %s", tuning.RecommendedSpeed)
	}

	// Scenario 2: Moderate load (1200 IOPS) -> throttles to "slow"
	tracker.RecordClientIOPS(1200)
	tuning, err = manager.AutoTuneMinIOScanner(context.Background(), "minio-cluster-1")
	if err != nil {
		t.Fatalf("AutoTuneMinIOScanner failed: %v", err)
	}
	if tuning.RecommendedSpeed != domainstorage.ScannerSpeedSlow {
		t.Errorf("expected slow under 1200 IOPS, got %s", tuning.RecommendedSpeed)
	}

	// Scenario 3: Normal load (400 IOPS) -> "default"
	tracker.RecordClientIOPS(400)
	tuning, err = manager.AutoTuneMinIOScanner(context.Background(), "minio-cluster-1")
	if err != nil {
		t.Fatalf("AutoTuneMinIOScanner failed: %v", err)
	}
	if tuning.RecommendedSpeed != domainstorage.ScannerSpeedDefault {
		t.Errorf("expected default under 400 IOPS, got %s", tuning.RecommendedSpeed)
	}

	// Scenario 4: Low load (80 IOPS) -> "fast"
	tracker.RecordClientIOPS(80)
	tuning, err = manager.AutoTuneMinIOScanner(context.Background(), "minio-cluster-1")
	if err != nil {
		t.Fatalf("AutoTuneMinIOScanner failed: %v", err)
	}
	if tuning.RecommendedSpeed != domainstorage.ScannerSpeedFast {
		t.Errorf("expected fast under 80 IOPS, got %s", tuning.RecommendedSpeed)
	}

	// Scenario 5: Idle window (20 IOPS) -> scales up to "fastest" to heal bit-rot
	tracker.RecordClientIOPS(20)
	tuning, err = manager.AutoTuneMinIOScanner(context.Background(), "minio-cluster-1")
	if err != nil {
		t.Fatalf("AutoTuneMinIOScanner failed: %v", err)
	}
	if tuning.RecommendedSpeed != domainstorage.ScannerSpeedFastest {
		t.Errorf("expected fastest during idle window, got %s", tuning.RecommendedSpeed)
	}
}

func TestResiliencyManager_VerifySafeDatabaseTarget(t *testing.T) {
	dbInspector := newTestDatabaseInspector()
	primaryConn := "postgres://postgres:secret@db-primary:5432/postgres"
	replicaConn := "postgres://postgres:secret@db-replica:5432/postgres"

	// primary: pg_is_in_recovery = false
	dbInspector.isInRecoveryMap[primaryConn] = false
	// replica: pg_is_in_recovery = true
	dbInspector.isInRecoveryMap[replicaConn] = true

	manager := NewResiliencyManager(
		zap.NewNop(),
		WithCephGateway(newTestCephGateway()),
		WithMinIOGateway(newTestMinIOGateway()),
		WithDatabaseRecoveryInspector(dbInspector),
	)

	// Primary node must strictly return false AND ErrPrimaryDatabaseProtected
	safe, err := manager.VerifySafeDatabaseTarget(context.Background(), primaryConn)
	if safe {
		t.Errorf("expected Primary node to NOT be a safe destructive target")
	}
	if !errors.Is(err, domainstorage.ErrPrimaryDatabaseProtected) {
		t.Errorf("expected ErrPrimaryDatabaseProtected on Primary node, got %v", err)
	}

	// Replica / Standby node must return true, nil
	safe, err = manager.VerifySafeDatabaseTarget(context.Background(), replicaConn)
	if !safe || err != nil {
		t.Errorf("expected Replica node to be safe target, got safe=%v, err=%v", safe, err)
	}

	// Empty conn string
	_, err = manager.VerifySafeDatabaseTarget(context.Background(), "")
	if !errors.Is(err, domainstorage.ErrInvalidDBConnString) {
		t.Errorf("expected ErrInvalidDBConnString, got %v", err)
	}
}
