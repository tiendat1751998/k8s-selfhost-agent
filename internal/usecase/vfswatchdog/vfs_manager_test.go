package vfswatchdog_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/vfswatchdog"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/vfswatchdog"
)

// In-memory test providers
type testFSProvider struct {
	mounts       []usecase.MountStat
	culpritDirs  []string
	scannedFiles uint64
}

func (p *testFSProvider) ListMounts(ctx context.Context) ([]usecase.MountStat, error) {
	return p.mounts, nil
}

func (p *testFSProvider) StatFS(ctx context.Context, mountPoint string) (uint64, uint64, error) {
	for _, m := range p.mounts {
		if m.MountPoint == mountPoint {
			return m.TotalInodes, m.FreeInodes, nil
		}
	}
	return 1000000, 500000, nil
}

func (p *testFSProvider) WalkDirectoryInodes(ctx context.Context, root string, limit int) ([]string, uint64, error) {
	if limit > len(p.culpritDirs) {
		limit = len(p.culpritDirs)
	}
	return p.culpritDirs[:limit], p.scannedFiles, nil
}

type testProcScanner struct {
	processes []domain.DStateProcessInfo
}

func (s *testProcScanner) ScanDStateProcesses(ctx context.Context, maxDuration time.Duration) ([]domain.DStateProcessInfo, error) {
	return s.processes, nil
}

type testUnmounter struct {
	unmountedPaths []string
	shouldFail     bool
}

func (u *testUnmounter) LazyUnmount(ctx context.Context, mountPoint string) error {
	if u.shouldFail {
		return errors.New("simulated unmount failure: device busy")
	}
	u.unmountedPaths = append(u.unmountedPaths, mountPoint)
	return nil
}

type testPodInspector struct {
	pods []usecase.PodPIDInfo
}

func (i *testPodInspector) InspectPods(ctx context.Context) ([]usecase.PodPIDInfo, error) {
	return i.pods, nil
}

func TestVFSManager_ScanInodes_Normal(t *testing.T) {
	ctx := context.Background()
	provider := &testFSProvider{
		mounts: []usecase.MountStat{
			{MountPoint: "/", TotalInodes: 10000000, FreeInodes: 8000000},     // 20%
			{MountPoint: "/data", TotalInodes: 5000000, FreeInodes: 2500000}, // 50%
		},
	}

	mgr := usecase.NewVFSManager(usecase.WithFSStatProvider(provider))

	reports, err := mgr.ScanInodes(ctx)
	if err != nil {
		t.Fatalf("unexpected error scanning inodes: %v", err)
	}

	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}

	if reports[0].UsagePercentage != 20.0 {
		t.Errorf("expected 20%% usage, got %f", reports[0].UsagePercentage)
	}
	if len(reports[0].TopCulpritDirs) != 0 {
		t.Errorf("expected no culprit search on healthy mount, got %v", reports[0].TopCulpritDirs)
	}
}

func TestVFSManager_ScanInodes_ExhaustionAndCulprits(t *testing.T) {
	ctx := context.Background()
	provider := &testFSProvider{
		mounts: []usecase.MountStat{
			{MountPoint: "/", TotalInodes: 10000000, FreeInodes: 7000000},
			{
				MountPoint:  "/tmp",
				TotalInodes: 2000000,
				FreeInodes:  100000, // 95% used -> exceeds 85%
			},
		},
		culpritDirs: []string{
			"/tmp/npm-cache",
			"/tmp/build-artifacts",
			"/tmp/session-tokens",
		},
		scannedFiles: 1900000,
	}

	mgr := usecase.NewVFSManager(usecase.WithFSStatProvider(provider))

	reports, err := mgr.ScanInodes(ctx)
	if !errors.Is(err, domain.ErrVFSInodeExhaustion) {
		t.Fatalf("expected ErrVFSInodeExhaustion, got %v", err)
	}

	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}

	tmpReport := reports[1]
	if tmpReport.UsagePercentage != 95.0 {
		t.Errorf("expected 95%% usage, got %f", tmpReport.UsagePercentage)
	}

	if len(tmpReport.TopCulpritDirs) != 3 {
		t.Fatalf("expected 3 top culprit dirs, got %d", len(tmpReport.TopCulpritDirs))
	}

	if tmpReport.TopCulpritDirs[0] != "/tmp/npm-cache" {
		t.Errorf("expected top culprit /tmp/npm-cache, got %s", tmpReport.TopCulpritDirs[0])
	}
}

func TestVFSManager_FindTopInodeConsumers(t *testing.T) {
	ctx := context.Background()
	provider := &testFSProvider{
		culpritDirs: []string{
			"/var/tmp/cache-1",
			"/var/tmp/cache-2",
		},
		scannedFiles: 50000,
	}

	mgr := usecase.NewVFSManager(usecase.WithFSStatProvider(provider))

	// Validation check on empty mount point
	_, err := mgr.FindTopInodeConsumers(ctx, "", 10)
	if err == nil {
		t.Fatal("expected error on empty mount_point")
	}

	culprits, err := mgr.FindTopInodeConsumers(ctx, "/var/tmp", 2)
	if err != nil {
		t.Fatalf("unexpected error finding top consumers: %v", err)
	}

	if len(culprits) != 2 || culprits[0] != "/var/tmp/cache-1" {
		t.Errorf("unexpected culprits returned: %v", culprits)
	}
}

func TestVFSManager_CleanTempFiles_WhitelistAndAge(t *testing.T) {
	ctx := context.Background()
	mgr := usecase.NewVFSManager()

	// 1. Rejection of non-whitelisted paths
	_, err := mgr.CleanTempFiles(ctx, "/home/user/documents", 24*time.Hour)
	if err == nil {
		t.Fatal("expected error cleaning non-whitelisted directory")
	}

	var dErr *domainerrors.DomainError
	if !errors.As(err, &dErr) || dErr.Code != domainerrors.CodeValidation {
		t.Errorf("expected validation domain error, got %v", err)
	}

	// 2. Allowed pruning in temporary directory
	tempDir, err := os.MkdirTemp("", "vfswatchdog_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Since tempDir on OS might be in /tmp or OS temp (which starts with /tmp or /var/tmp or os temp),
	// let's test whitelist logic directly
	if !usecase.IsWhitelistedTempDir("/tmp") {
		t.Error("expected /tmp to be whitelisted")
	}
	if !usecase.IsWhitelistedTempDir("/tmp/subfolder/nested") {
		t.Error("expected /tmp/subfolder/nested to be whitelisted")
	}
	if !usecase.IsWhitelistedTempDir("/var/tmp/worker") {
		t.Error("expected /var/tmp/worker to be whitelisted")
	}
	if !usecase.IsWhitelistedTempDir("/cache/build") {
		t.Error("expected /cache/build to be whitelisted")
	}
	if usecase.IsWhitelistedTempDir("/etc/nginx") {
		t.Error("expected /etc/nginx NOT to be whitelisted")
	}
	if usecase.IsWhitelistedTempDir("/root") {
		t.Error("expected /root NOT to be whitelisted")
	}

	// Create test files in tempDir
	oldFile := filepath.Join(tempDir, "old_file.tmp")
	if err := os.WriteFile(oldFile, []byte("temp"), 0644); err != nil {
		t.Fatalf("failed to create old file: %v", err)
	}

	// Set mod time to 48 hours ago
	past := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(oldFile, past, past)

	newFile := filepath.Join(tempDir, "new_file.tmp")
	if err := os.WriteFile(newFile, []byte("fresh"), 0644); err != nil {
		t.Fatalf("failed to create new file: %v", err)
	}

	// If tempDir is whitelisted on current OS, test CleanTempFiles directly
	if usecase.IsWhitelistedTempDir(tempDir) {
		pruned, err := mgr.CleanTempFiles(ctx, tempDir, 24*time.Hour)
		if err != nil {
			t.Fatalf("failed to clean temp files: %v", err)
		}
		if pruned != 1 {
			t.Errorf("expected 1 file pruned, got %d", pruned)
		}
		if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
			t.Errorf("expected old file to be deleted")
		}
		if _, err := os.Stat(newFile); err != nil {
			t.Errorf("expected new file to remain untouched")
		}
	}
}

func TestVFSManager_DetectDStateProcesses_AndLazyUnmount(t *testing.T) {
	ctx := context.Background()
	stuckProc := domain.DStateProcessInfo{
		PID:          8821,
		Command:      "postgres_backup_nfs",
		WaitDuration: 12 * time.Minute, // > 5m
		WchanSymbol:  "nfs_file_write",
		MountPoint:   "/mnt/nfs_backup",
	}

	healthyProc := domain.DStateProcessInfo{
		PID:          9012,
		Command:      "tar_pack",
		WaitDuration: 1 * time.Minute, // < 5m
		WchanSymbol:  "sync_inodes",
		MountPoint:   "/mnt/nfs_backup",
	}

	procScanner := &testProcScanner{
		processes: []domain.DStateProcessInfo{stuckProc, healthyProc},
	}
	unmounter := &testUnmounter{}

	mgr := usecase.NewVFSManager(
		usecase.WithProcessScanner(procScanner),
		usecase.WithUnmountExecutor(unmounter),
	)

	// Detection
	procs, err := mgr.DetectDStateProcesses(ctx, 5*time.Minute)
	if !errors.Is(err, domain.ErrVFSDStateDeadlock) {
		t.Fatalf("expected ErrVFSDStateDeadlock, got %v", err)
	}

	if len(procs) != 2 {
		t.Fatalf("expected 2 processes returned, got %d", len(procs))
	}

	// Trigger lazy unmount on the hung mount point
	unmountErr := mgr.RemountOrUnmountHungNFS(ctx, stuckProc.MountPoint)
	if unmountErr != nil {
		t.Fatalf("unexpected error during lazy unmount: %v", unmountErr)
	}

	if len(unmounter.unmountedPaths) != 1 || unmounter.unmountedPaths[0] != "/mnt/nfs_backup" {
		t.Errorf("expected unmount of /mnt/nfs_backup, got %v", unmounter.unmountedPaths)
	}

	// Unmount failure simulation
	unmounter.shouldFail = true
	failErr := mgr.RemountOrUnmountHungNFS(ctx, "/mnt/nfs_backup")
	if !errors.Is(failErr, domain.ErrNFSUnmountFailed) {
		t.Fatalf("expected ErrNFSUnmountFailed, got %v", failErr)
	}
}

func TestVFSManager_EnforcePIDLimits_AndZombieMitigation(t *testing.T) {
	ctx := context.Background()

	t.Run("Normal pods within limit", func(t *testing.T) {
		inspector := &testPodInspector{
			pods: []usecase.PodPIDInfo{
				{
					PodNamespace:          "default",
					PodName:               "api-server-1",
					ShareProcessNamespace: true,
					InitSubreaper:         true,
					CurrentPIDs:           50,
					ZombieCount:           0,
				},
			},
		}

		mgr := usecase.NewVFSManager(usecase.WithPodPIDInspector(inspector))
		if err := mgr.EnforcePIDLimits(ctx, 100); err != nil {
			t.Errorf("unexpected error enforcing limits: %v", err)
		}
	})

	t.Run("Pod exceeding max PIDs limit", func(t *testing.T) {
		inspector := &testPodInspector{
			pods: []usecase.PodPIDInfo{
				{
					PodNamespace:          "default",
					PodName:               "fork-bomb-pod",
					ShareProcessNamespace: false,
					InitSubreaper:         false,
					CurrentPIDs:           250,
					ZombieCount:           0,
				},
			},
		}

		mgr := usecase.NewVFSManager(usecase.WithPodPIDInspector(inspector))
		err := mgr.EnforcePIDLimits(ctx, 100)
		if !errors.Is(err, domain.ErrPIDQuotaExceeded) {
			t.Fatalf("expected ErrPIDQuotaExceeded, got %v", err)
		}
	})

	t.Run("Zombie accumulation without subreaper", func(t *testing.T) {
		inspector := &testPodInspector{
			pods: []usecase.PodPIDInfo{
				{
					PodNamespace:          "production",
					PodName:               "legacy-worker-without-tini",
					ShareProcessNamespace: false,
					InitSubreaper:         false,
					CurrentPIDs:           40,
					ZombieCount:           15, // Zombies accumulating
				},
			},
		}

		mgr := usecase.NewVFSManager(usecase.WithPodPIDInspector(inspector))
		err := mgr.EnforcePIDLimits(ctx, 500)
		if !errors.Is(err, domain.ErrPIDQuotaExceeded) {
			t.Fatalf("expected ErrPIDQuotaExceeded due to zombie risk, got %v", err)
		}
	})
}

func TestVFSManager_ConcurrentLoadAndMetrics(t *testing.T) {
	ctx := context.Background()
	provider := &testFSProvider{
		mounts: []usecase.MountStat{
			{MountPoint: "/", TotalInodes: 1000000, FreeInodes: 500000},
		},
		culpritDirs:  []string{"/tmp"},
		scannedFiles: 10,
	}

	mgr := usecase.NewVFSManager(usecase.WithFSStatProvider(provider))

	const numWorkers = 50
	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = mgr.ScanInodes(ctx)
				_, _ = mgr.FindTopInodeConsumers(ctx, "/", 1)
			}
		}()
	}

	wg.Wait()

	inodeMetric, _, _ := mgr.GetMetrics()
	if inodeMetric.ScannedFiles.Load() == 0 {
		t.Errorf("expected ScannedFiles to be accumulated concurrently")
	}
}

func TestVFSManager_RemountOrUnmountHungNFS_RejectsHyphenAndEmpty(t *testing.T) {
	mgr := usecase.NewVFSManager()
	ctx := context.Background()

	badMounts := []string{"", "   ", "-f", "-l", "--help", "-o remount"}
	for _, bad := range badMounts {
		err := mgr.RemountOrUnmountHungNFS(ctx, bad)
		if err == nil {
			t.Errorf("expected RemountOrUnmountHungNFS to reject invalid mount %q, got nil", bad)
		}
	}
}

func TestVFSManager_DefaultFSProvider_MountHandling(t *testing.T) {
	mgr := usecase.NewVFSManager()
	ctx := context.Background()

	reports, err := mgr.ScanInodes(ctx)
	if err != nil {
		if !strings.Contains(err.Error(), "/proc/mounts") {
			t.Errorf("expected error mentioning /proc/mounts, got %v", err)
		}
	} else {
		if len(reports) == 0 {
			t.Errorf("expected non-empty reports when /proc/mounts is available")
		}
	}
}
