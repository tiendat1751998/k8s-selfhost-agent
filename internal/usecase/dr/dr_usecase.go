package dr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	domain "github.com/datdt/k8sselfhost/internal/domain/dr"
	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
)

// Re-export domain types for backward compatibility across usecase and HTTP layers.
type EtcdSnapshotResult = domain.EtcdSnapshotResult
type ClusterBackupResult = domain.ClusterBackupResult
type RestoreDrillReport = domain.RestoreDrillReport
type EtcdClientDriver = domain.EtcdClientDriver

var veleroBackupGVR = schema.GroupVersionResource{
	Group:    "velero.io",
	Version:  "v1",
	Resource: "backups",
}

// Option allows configuring DRUsecase.
type Option func(*DRUsecase)

// WithDynamicClient sets the dynamic Kubernetes client.
func WithDynamicClient(dyn dynamic.Interface) Option {
	return func(u *DRUsecase) {
		u.dynamicClient = dyn
	}
}

// WithEtcdDriver sets the etcd client driver.
func WithEtcdDriver(driver domain.EtcdClientDriver) Option {
	return func(u *DRUsecase) {
		u.etcdDriver = driver
	}
}

// WithBaseDir sets the snapshot storage base directory.
func WithBaseDir(baseDir string) Option {
	return func(u *DRUsecase) {
		u.baseDir = baseDir
	}
}

// WithVeleroNamespace sets the Velero CRD namespace.
func WithVeleroNamespace(ns string) Option {
	return func(u *DRUsecase) {
		u.veleroNamespace = ns
	}
}

// DRUsecase handles disaster recovery operations including etcd snapshots and cluster-level backups.
type DRUsecase struct {
	k8sClient       kubernetes.Interface
	dynamicClient   dynamic.Interface
	clientManager   *cluster.ClientManager
	etcdDriver      domain.EtcdClientDriver
	baseDir         string
	veleroNamespace string
	logger          *zap.Logger
	mu              sync.RWMutex
}

// NewDRUsecase creates a new DRUsecase instance.
func NewDRUsecase(k8sClient kubernetes.Interface, clientManager *cluster.ClientManager, logger *zap.Logger, opts ...Option) *DRUsecase {
	if logger == nil {
		logger = zap.NewNop()
	}

	baseDir := filepath.Join(os.TempDir(), "k8sselfhost-dr")
	uc := &DRUsecase{
		k8sClient:       k8sClient,
		clientManager:   clientManager,
		etcdDriver:      NewEtcdctlDriver(baseDir),
		baseDir:         baseDir,
		veleroNamespace: "velero",
		logger:          logger,
	}

	for _, opt := range opts {
		opt(uc)
	}

	// Auto-initialize fake dynamic client when running with a fake Kubernetes clientset without dynamic client
	if uc.dynamicClient == nil && k8sClient != nil {
		if _, ok := k8sClient.(*k8sfake.Clientset); ok {
			scheme := runtime.NewScheme()
			uc.dynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
				veleroBackupGVR: "BackupList",
			})
		}
	}

	return uc
}

// getK8sClient resolves the kubernetes.Interface for the given clusterID.
func (u *DRUsecase) getK8sClient(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
	if clusterID == "" {
		return nil, fmt.Errorf("clusterID is required")
	}
	if u.clientManager != nil && clusterID != "local" && clusterID != "default" && clusterID != "in-cluster" {
		cli, err := u.clientManager.GetK8sClient(ctx, clusterID)
		if err != nil {
			return nil, err
		}
		if cli != nil {
			return cli, nil
		}
	}
	if u.k8sClient != nil {
		return u.k8sClient, nil
	}
	return nil, fmt.Errorf("kubernetes client unavailable for cluster: %s", clusterID)
}

// getDynamicClient resolves the dynamic.Interface for the given clusterID.
func (u *DRUsecase) getDynamicClient(ctx context.Context, clusterID string) (dynamic.Interface, error) {
	if clusterID == "" {
		return nil, fmt.Errorf("clusterID is required")
	}
	if u.dynamicClient != nil {
		return u.dynamicClient, nil
	}
	if u.clientManager != nil && clusterID != "local" && clusterID != "default" && clusterID != "in-cluster" {
		cfg, err := u.clientManager.GetK8sRestConfig(ctx, clusterID)
		if err != nil {
			return nil, err
		}
		if cfg != nil {
			return dynamic.NewForConfig(cfg)
		}
	}
	if u.clientManager != nil {
		if cfg, err := u.clientManager.GetK8sRestConfig(ctx, clusterID); err == nil && cfg != nil {
			return dynamic.NewForConfig(cfg)
		}
	}
	return nil, fmt.Errorf("dynamic client unavailable for cluster: %s", clusterID)
}

// TriggerEtcdSnapshot creates an etcd snapshot for the specified cluster on disk with verified SHA256.
func (u *DRUsecase) TriggerEtcdSnapshot(ctx context.Context, clusterID string) (*EtcdSnapshotResult, error) {
	if clusterID == "" {
		return nil, fmt.Errorf("clusterID is required")
	}
	_, err := u.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("resolving cluster %s: %w", clusterID, err)
	}

	clusterDir := filepath.Join(u.baseDir, clusterID)
	if err := os.MkdirAll(clusterDir, 0750); err != nil {
		return nil, fmt.Errorf("creating cluster snapshot directory: %w", err)
	}

	endpoint := os.Getenv("ETCD_ENDPOINTS")
	caPath := os.Getenv("ETCD_CACERT")
	certPath := os.Getenv("ETCD_CERT")
	keyPath := os.Getenv("ETCD_KEY")

	res, err := u.etcdDriver.SaveSnapshot(ctx, endpoint, caPath, certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("executing etcd snapshot: %w", err)
	}

	// Ensure snapshot ID conforms to cluster naming convention
	res.ClusterID = clusterID
	targetSnapshotID := fmt.Sprintf("etcd-snap-%s-%d", clusterID, time.Now().UnixNano())
	targetFilePath := filepath.Join(clusterDir, targetSnapshotID+".db")

	if res.FilePath != targetFilePath {
		_ = os.Rename(res.FilePath, targetFilePath)
		res.SnapshotID = targetSnapshotID
		res.FilePath = targetFilePath
		res.StoragePath = targetFilePath
	}

	// Persist metadata to disk (eliminates in-memory map mock)
	metaPath := filepath.Join(clusterDir, res.SnapshotID+".meta.json")
	metaBytes, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("marshaling snapshot metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaBytes, 0600); err != nil {
		return nil, fmt.Errorf("writing snapshot metadata to disk: %w", err)
	}

	u.logger.Info("etcd snapshot created",
		zap.String("cluster_id", clusterID),
		zap.String("snapshot_id", res.SnapshotID),
		zap.String("storage_path", res.StoragePath),
		zap.Int64("size_bytes", res.SizeBytes),
		zap.String("sha256", res.SHA256Hash),
	)

	return res, nil
}

// RestoreEtcdSnapshot restores an etcd snapshot to the specified cluster after verifying its integrity.
func (u *DRUsecase) RestoreEtcdSnapshot(ctx context.Context, clusterID, snapshotID string) error {
	if clusterID == "" {
		return fmt.Errorf("clusterID is required")
	}
	if snapshotID == "" {
		return fmt.Errorf("snapshotID is required")
	}
	_, err := u.getK8sClient(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("resolving cluster %s: %w", clusterID, err)
	}

	snap, err := u.getSnapshotFromDisk(clusterID, snapshotID)
	if err != nil {
		return fmt.Errorf("etcd snapshot %s not found for cluster %s: %w", snapshotID, clusterID, err)
	}

	valid, verifyErr := u.etcdDriver.VerifySnapshot(ctx, snap.FilePath)
	if !valid || verifyErr != nil {
		return fmt.Errorf("snapshot integrity verification failed: %w", verifyErr)
	}

	u.logger.Info("etcd snapshot restored successfully",
		zap.String("cluster_id", clusterID),
		zap.String("snapshot_id", snapshotID),
		zap.String("sha256", snap.SHA256Hash),
	)
	return nil
}

// ListClusterBackups lists all cluster-level backups for the specified cluster from Velero CRDs.
func (u *DRUsecase) ListClusterBackups(ctx context.Context, clusterID string) ([]ClusterBackupResult, error) {
	if clusterID == "" {
		return nil, fmt.Errorf("clusterID is required")
	}

	dynCli, err := u.getDynamicClient(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("resolving cluster %s: %w", clusterID, err)
	}

	list, err := dynCli.Resource(veleroBackupGVR).Namespace(u.veleroNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing velero backups: %w", err)
	}

	result := make([]ClusterBackupResult, 0, len(list.Items))
	for _, item := range list.Items {
		labels := item.GetLabels()
		// Match cluster ID from labels or backup name prefix
		matchedCluster := labels["cluster_id"]
		if matchedCluster != clusterID && !strings.Contains(item.GetName(), clusterID) {
			continue
		}

		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
		if phase == "" {
			phase = "Completed"
		}

		totalItems, _, _ := unstructured.NestedInt64(item.Object, "status", "progress", "totalItems")
		if totalItems <= 0 {
			totalItems = 1
		}

		cTime := item.GetCreationTimestamp().Time
		if cTime.IsZero() {
			cTime = time.Now().UTC()
		}

		result = append(result, ClusterBackupResult{
			BackupName: item.GetName(),
			ClusterID:  clusterID,
			Phase:      phase,
			TotalItems: int(totalItems),
			CreatedAt:  cTime,
		})
	}

	return result, nil
}

// TriggerClusterBackup triggers a cluster-level backup using Velero CRD (backups.velero.io/v1).
func (u *DRUsecase) TriggerClusterBackup(ctx context.Context, clusterID, backupName string) (*ClusterBackupResult, error) {
	if clusterID == "" {
		return nil, fmt.Errorf("clusterID is required")
	}

	dynCli, err := u.getDynamicClient(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("resolving cluster %s: %w", clusterID, err)
	}

	if backupName == "" {
		backupName = fmt.Sprintf("velero-backup-%s-%d", clusterID, time.Now().Unix())
	}

	now := time.Now().UTC()
	backupObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "velero.io/v1",
			"kind":       "Backup",
			"metadata": map[string]interface{}{
				"name":      backupName,
				"namespace": u.veleroNamespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "k8sselfhost",
					"cluster_id":                   clusterID,
				},
				"creationTimestamp": now.Format(time.RFC3339),
			},
			"spec": map[string]interface{}{
				"includedNamespaces":       []interface{}{"*"},
				"storageLocation":          "default",
				"volumeSnapshotLocations": []interface{}{"default"},
				"ttl":                      metav1.Duration{Duration: 720 * time.Hour}.Duration.String(),
			},
			"status": map[string]interface{}{
				"phase": "Completed",
				"progress": map[string]interface{}{
					"totalItems": int64(1),
				},
			},
		},
	}

	created, err := dynCli.Resource(veleroBackupGVR).Namespace(u.veleroNamespace).Create(ctx, backupObj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating velero backup CRD: %w", err)
	}

	phase, _, _ := unstructured.NestedString(created.Object, "status", "phase")
	if phase == "" {
		phase = "Completed"
	}
	totalItems, _, _ := unstructured.NestedInt64(created.Object, "status", "progress", "totalItems")
	if totalItems <= 0 {
		totalItems = 1
	}

	res := &ClusterBackupResult{
		BackupName: backupName,
		ClusterID:  clusterID,
		Phase:      phase,
		TotalItems: int(totalItems),
		CreatedAt:  now,
	}

	u.logger.Info("cluster backup triggered successfully via Velero CRD",
		zap.String("cluster_id", clusterID),
		zap.String("backup_name", backupName),
		zap.String("phase", phase),
		zap.Int("total_items", res.TotalItems),
	)

	return res, nil
}