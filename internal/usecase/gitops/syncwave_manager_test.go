package gitops_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
)

// fakeGitCommander simulates git merge-base responses for deterministic unit testing.
type fakeGitCommander struct {
	ancestorMap map[string]bool // key: "ancestor:descendant"
}

func (f *fakeGitCommander) IsAncestor(ctx context.Context, repoPath, ancestorSHA, descendantSHA string) (bool, error) {
	key := fmt.Sprintf("%s:%s", ancestorSHA, descendantSHA)
	return f.ancestorMap[key], nil
}

// TestVerifyCommitMonotonicOrdering_RejectsOlderCommit tests rejection when incoming commit is an ancestor of current.
func TestVerifyCommitMonotonicOrdering_RejectsOlderCommit(t *testing.T) {
	ctx := context.Background()
	fakeGit := &fakeGitCommander{
		ancestorMap: map[string]bool{
			"commit-old:commit-hotfix": true,  // commit-old is an ancestor of commit-hotfix
			"commit-hotfix:commit-old": false, // commit-hotfix is NOT an ancestor of commit-old
		},
	}

	manager := usecaseGitops.NewSyncWaveManager(usecaseGitops.WithGitCommander(fakeGit))

	// Incoming commit is old; current running commit is hotfix
	isNewer, err := manager.VerifyCommitMonotonicOrdering(ctx, "/repos/app", "commit-old", "commit-hotfix")
	if isNewer {
		t.Fatal("expected isNewer to be false for older commit")
	}
	if err == nil {
		t.Fatal("expected error for obsolete commit delivery, got nil")
	}
	if !errors.Is(err, domainGitops.ErrGitOpsObsoleteWebhook) {
		t.Fatalf("expected ErrGitOpsObsoleteWebhook, got: %v", err)
	}

	metrics := manager.Metrics()
	if metrics.WebhookDroppedObsoleteCount.Load() != 1 {
		t.Fatalf("expected WebhookDroppedObsoleteCount to be 1, got %d", metrics.WebhookDroppedObsoleteCount.Load())
	}
}

// TestVerifyCommitMonotonicOrdering_AcceptsNewerCommit tests acceptance when current is ancestor of incoming.
func TestVerifyCommitMonotonicOrdering_AcceptsNewerCommit(t *testing.T) {
	ctx := context.Background()
	fakeGit := &fakeGitCommander{
		ancestorMap: map[string]bool{
			"commit-old:commit-newer": true,
			"commit-newer:commit-old": false,
		},
	}

	manager := usecaseGitops.NewSyncWaveManager(usecaseGitops.WithGitCommander(fakeGit))

	isNewer, err := manager.VerifyCommitMonotonicOrdering(ctx, "/repos/app", "commit-newer", "commit-old")
	if !isNewer {
		t.Fatal("expected isNewer to be true for newer commit")
	}
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	metrics := manager.Metrics()
	if metrics.WebhookProcessedCount.Load() != 1 {
		t.Fatalf("expected WebhookProcessedCount to be 1, got %d", metrics.WebhookProcessedCount.Load())
	}
}

// TestVerifyCommitMonotonicOrdering_IdenticalCommit tests rejection when incoming matches current commit.
func TestVerifyCommitMonotonicOrdering_IdenticalCommit(t *testing.T) {
	ctx := context.Background()
	manager := usecaseGitops.NewSyncWaveManager()

	isNewer, err := manager.VerifyCommitMonotonicOrdering(ctx, "/repos/app", "commit-same", "commit-same")
	if isNewer {
		t.Fatal("expected isNewer to be false for identical commit")
	}
	if !errors.Is(err, domainGitops.ErrGitOpsObsoleteWebhook) {
		t.Fatalf("expected ErrGitOpsObsoleteWebhook, got: %v", err)
	}
}

// TestVerifyCommitMonotonicOrdering_RealGitRepo tests with a real on-disk git repository.
func TestVerifyCommitMonotonicOrdering_RealGitRepo(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize git repo
	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s", args, string(out))
		}
		return string(out)
	}

	runGit("init")
	runGit("config", "user.email", "ci@k8sselfhost.io")
	runGit("config", "user.name", "CI Bot")

	// Commit 1
	file1 := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(file1, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "file.txt")
	runGit("commit", "-m", "first commit")
	cmdRev1 := exec.Command("git", "rev-parse", "HEAD")
	cmdRev1.Dir = tempDir
	sha1Out, _ := cmdRev1.Output()
	sha1 := string(sha1Out)[:40]

	// Commit 2
	if err := os.WriteFile(file1, []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("commit", "-am", "second commit")
	cmdRev2 := exec.Command("git", "rev-parse", "HEAD")
	cmdRev2.Dir = tempDir
	sha2Out, _ := cmdRev2.Output()
	sha2 := string(sha2Out)[:40]

	manager := usecaseGitops.NewSyncWaveManager() // uses DefaultGitCommander
	ctx := context.Background()

	// SHA1 is older than SHA2 -> incoming SHA1 when current is SHA2 should be rejected
	isNewer, err := manager.VerifyCommitMonotonicOrdering(ctx, tempDir, sha1, sha2)
	if isNewer || !errors.Is(err, domainGitops.ErrGitOpsObsoleteWebhook) {
		t.Fatalf("expected SHA1 rejected as obsolete, got isNewer=%v, err=%v", isNewer, err)
	}

	// Incoming SHA2 when current is SHA1 should be accepted
	isNewer2, err2 := manager.VerifyCommitMonotonicOrdering(ctx, tempDir, sha2, sha1)
	if !isNewer2 || err2 != nil {
		t.Fatalf("expected SHA2 accepted as newer, got isNewer=%v, err=%v", isNewer2, err2)
	}
}

// TestAuditSyncWaveCycle_DetectsCycles tests detection of circular dependencies in DAG.
func TestAuditSyncWaveCycle_DetectsCycles(t *testing.T) {
	ctx := context.Background()
	manager := usecaseGitops.NewSyncWaveManager()

	// Cyclic manifests: Service/frontend depends on Deployment/backend, which depends on Service/frontend
	manifest1 := []byte(`
apiVersion: v1
kind: Service
metadata:
  name: frontend
  annotations:
    argocd.argoproj.io/sync-wave: "1"
    argocd.argoproj.io/sync-wave-depends-on: "Deployment/backend"
`)
	manifest2 := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend
  annotations:
    argocd.argoproj.io/sync-wave: "1"
    argocd.argoproj.io/sync-wave-depends-on: "Service/frontend"
`)

	hasCycle, details, err := manager.AuditSyncWaveCycle(ctx, [][]byte{manifest1, manifest2})
	if !hasCycle {
		t.Fatal("expected cycle to be detected")
	}
	if err == nil || !errors.Is(err, domainGitops.ErrGitOpsCircularSyncWave) {
		t.Fatalf("expected ErrGitOpsCircularSyncWave, got: %v", err)
	}
	if details == "" {
		t.Fatal("expected cycle details to be populated")
	}
}

// TestAuditSyncWaveCycle_DetectsWaveInversion tests detection of sync-wave inversion deadlocks.
func TestAuditSyncWaveCycle_DetectsWaveInversion(t *testing.T) {
	ctx := context.Background()
	manager := usecaseGitops.NewSyncWaveManager()

	// Manifest A is in Wave 0, but depends on Manifest B in Wave 2 (Wave 0 awaits Wave 2, deadlock)
	manifestA := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  annotations:
    argocd.argoproj.io/sync-wave: "0"
    argocd.argoproj.io/sync-wave-depends-on: "Service/db"
`)
	manifestB := []byte(`
apiVersion: v1
kind: Service
metadata:
  name: db
  annotations:
    argocd.argoproj.io/sync-wave: "2"
`)

	hasCycle, details, err := manager.AuditSyncWaveCycle(ctx, [][]byte{manifestA, manifestB})
	if !hasCycle {
		t.Fatal("expected sync-wave inversion to be flagged as cycle/deadlock")
	}
	if err == nil || !errors.Is(err, domainGitops.ErrGitOpsCircularSyncWave) {
		t.Fatalf("expected ErrGitOpsCircularSyncWave, got: %v", err)
	}
	if details == "" {
		t.Fatal("expected cycle details for inversion")
	}
}

// TestAuditSyncWaveCycle_AcyclicValid tests that a valid DAG passes with no cycle.
func TestAuditSyncWaveCycle_AcyclicValid(t *testing.T) {
	ctx := context.Background()
	manager := usecaseGitops.NewSyncWaveManager()

	manifest1 := []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-cfg
  annotations:
    argocd.argoproj.io/sync-wave: "-1"
`)
	manifest2 := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-deploy
  annotations:
    argocd.argoproj.io/sync-wave: "0"
    argocd.argoproj.io/sync-wave-depends-on: "ConfigMap/app-cfg"
`)
	manifest3 := []byte(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-ing
  annotations:
    argocd.argoproj.io/sync-wave: "1"
    argocd.argoproj.io/sync-wave-depends-on: "Deployment/app-deploy"
`)

	hasCycle, details, err := manager.AuditSyncWaveCycle(ctx, [][]byte{manifest1, manifest2, manifest3})
	if hasCycle {
		t.Fatalf("expected no cycle, got details: %s", details)
	}
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

// TestDetectAndReconcileDrift_ConflictDetected tests drift conflict when kubectl edit modified conflicting fields.
func TestDetectAndReconcileDrift_ConflictDetected(t *testing.T) {
	ctx := context.Background()

	// Live object with kubectl-edit managing image
	liveObj := []byte(`{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "payment-api",
    "namespace": "prod",
    "managedFields": [
      {
        "manager": "kubectl-edit",
        "operation": "Update",
        "fieldsV1": {
          "f:spec": {
            "f:template": {
              "f:spec": {
                "f:containers": {
                  "k:{\"name\":\"payment\"}": {
                    "f:image": {}
                  }
                }
              }
            }
          }
        }
      }
    ]
  },
  "spec": {
    "template": {
      "spec": {
        "containers": [
          {"name": "payment", "image": "payment:hotfix-manual"}
        ]
      }
    }
  }
}`)

	// New chart manifest updating image
	newChartManifest := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payment-api
  namespace: prod
spec:
  template:
    spec:
      containers:
        - name: payment
          image: payment:v2.0.0
`)

	fakeCluster := &fakeClusterStateProvider{
		resources: map[string][]byte{
			"prod/Deployment/payment-api": liveObj,
		},
	}

	manager := usecaseGitops.NewSyncWaveManager(usecaseGitops.WithLiveClusterStateProvider(fakeCluster))

	res, err := manager.DetectAndReconcileDrift(ctx, "payment-api", "prod", newChartManifest)
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if !res.HasDrift {
		t.Fatal("expected HasDrift to be true")
	}
	if !res.ConflictDetected {
		t.Fatal("expected ConflictDetected to be true")
	}
	if res.AutoResolved {
		t.Fatal("expected AutoResolved to be false")
	}
	if err == nil || !errors.Is(err, domainGitops.ErrGitOpsDriftConflict) {
		t.Fatalf("expected ErrGitOpsDriftConflict, got: %v", err)
	}

	metrics := manager.Metrics()
	if metrics.DriftConflictsCount.Load() != 1 {
		t.Fatalf("expected DriftConflictsCount 1, got %d", metrics.DriftConflictsCount.Load())
	}
}

// TestDetectAndReconcileDrift_AutoResolved tests non-conflicting drift (e.g. out-of-band replicas or label).
func TestDetectAndReconcileDrift_AutoResolved(t *testing.T) {
	ctx := context.Background()

	// Live object with kubectl-edit managing replicas
	liveObj := []byte(`{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "worker",
    "namespace": "prod",
    "managedFields": [
      {
        "manager": "kubectl-edit",
        "operation": "Update",
        "fieldsV1": {
          "f:spec": {
            "f:replicas": {}
          }
        }
      }
    ]
  },
  "spec": {
    "replicas": 5
  }
}`)

	// New chart manifest does NOT specify replicas (managed by HPA / autoscaler)
	newChartManifest := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
  namespace: prod
spec:
  template:
    spec:
      containers:
        - name: worker
          image: worker:v1.5.0
`)

	fakeCluster := &fakeClusterStateProvider{
		resources: map[string][]byte{
			"prod/Deployment/worker": liveObj,
		},
	}

	manager := usecaseGitops.NewSyncWaveManager(usecaseGitops.WithLiveClusterStateProvider(fakeCluster))

	res, err := manager.DetectAndReconcileDrift(ctx, "worker", "prod", newChartManifest)
	if err != nil {
		t.Fatalf("expected no error for auto-resolved drift, got: %v", err)
	}
	if !res.HasDrift {
		t.Fatal("expected HasDrift to be true")
	}
	if res.ConflictDetected {
		t.Fatal("expected ConflictDetected to be false")
	}
	if !res.AutoResolved {
		t.Fatal("expected AutoResolved to be true")
	}

	metrics := manager.Metrics()
	if metrics.DriftAutoResolvedCount.Load() != 1 {
		t.Fatalf("expected DriftAutoResolvedCount 1, got %d", metrics.DriftAutoResolvedCount.Load())
	}
}

// TestSyncWaveManager_HighConcurrency verifies thread safety under high concurrent goroutine pressure.
func TestSyncWaveManager_HighConcurrency(t *testing.T) {
	ctx := context.Background()
	fakeGit := &fakeGitCommander{
		ancestorMap: map[string]bool{
			"c-old:c-new": true,
		},
	}
	manager := usecaseGitops.NewSyncWaveManager(usecaseGitops.WithGitCommander(fakeGit))

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				_, _ = manager.VerifyCommitMonotonicOrdering(ctx, "/repo", "c-new", "c-old")
			} else {
				_, _ = manager.VerifyCommitMonotonicOrdering(ctx, "/repo", "c-old", "c-new")
			}
		}(i)
	}

	wg.Wait()

	metrics := manager.Metrics()
	total := metrics.WebhookProcessedCount.Load() + metrics.WebhookDroppedObsoleteCount.Load()
	if total != goroutines {
		t.Fatalf("expected total operations %d, got %d", goroutines, total)
	}
}

type fakeClusterStateProvider struct {
	resources map[string][]byte
}

func (f *fakeClusterStateProvider) GetLiveResource(ctx context.Context, namespace, kind, name string) ([]byte, error) {
	key := fmt.Sprintf("%s/%s/%s", namespace, kind, name)
	if data, ok := f.resources[key]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("resource not found: %s", key)
}
