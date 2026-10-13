package secdefense

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	domain "github.com/datdt/k8sselfhost/internal/domain/secdefense"
)

func TestSSRFIPv4MetadataBlocked(t *testing.T) {
	mgr := NewDefenseManager(nil)
	ctx := context.Background()

	result, err := mgr.ValidateURLAgainstSSRF(ctx, "http://169.254.169.254/latest/meta-data/")
	if err == nil {
		t.Fatal("expected error ErrSecSSRFMetadataAttempt, got nil")
	}
	if err != domain.ErrSecSSRFMetadataAttempt {
		t.Fatalf("expected ErrSecSSRFMetadataAttempt, got %v", err)
	}
	if result == nil || !result.IsBlocked {
		t.Fatal("expected result to be blocked")
	}
	if !result.IsCloudMetadata {
		t.Fatal("expected IsCloudMetadata to be true")
	}
}

func TestSSRFIPv6MetadataBlocked(t *testing.T) {
	mgr := NewDefenseManager(nil)
	ctx := context.Background()

	// AWS IPv6 metadata endpoint: [fd00:ec2::254]
	result, err := mgr.ValidateURLAgainstSSRF(ctx, "http://[fd00:ec2::254]/latest/meta-data/")
	if err == nil {
		t.Fatal("expected error ErrSecSSRFMetadataAttempt for IPv6 metadata, got nil")
	}
	if err != domain.ErrSecSSRFMetadataAttempt {
		t.Fatalf("expected ErrSecSSRFMetadataAttempt, got %v", err)
	}
	if result == nil || !result.IsBlocked {
		t.Fatal("expected result to be blocked")
	}
	if !result.IsCloudMetadata {
		t.Fatal("expected IsCloudMetadata to be true for IPv6 metadata")
	}
}

func TestSSRFIPv6LinkLocalAndLoopbackBlocked(t *testing.T) {
	mgr := NewDefenseManager(nil)
	ctx := context.Background()

	tests := []struct {
		name string
		url  string
	}{
		{"IPv6 loopback", "http://[::1]:8080/metrics"},
		{"IPv4 loopback", "http://127.0.0.1:8080/debug/pprof"},
		{"IPv6 link-local", "http://[fe80::1ff:fe00:3a60]:8080"},
		{"IPv6 ULA", "http://[fc00::1]:8080"},
		{"RFC 1918 class A", "http://10.100.1.5:8080"},
		{"RFC 1918 class B", "http://172.20.0.1:8080"},
		{"RFC 1918 class C", "http://192.168.1.50:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, _ := mgr.ValidateURLAgainstSSRF(ctx, tt.url)
			if res == nil || !res.IsBlocked {
				t.Fatalf("expected %s (%s) to be blocked", tt.name, tt.url)
			}
		})
	}
}

func TestSSRFPublicSafeURLAllowed(t *testing.T) {
	mgr := NewDefenseManager(nil)
	ctx := context.Background()

	// 1.1.1.1 is public Cloudflare DNS IP literal
	res, err := mgr.ValidateURLAgainstSSRF(ctx, "https://1.1.1.1/dns-query")
	if err != nil {
		t.Fatalf("unexpected error for public IP: %v", err)
	}
	if res.IsBlocked {
		t.Fatalf("expected public URL not to be blocked, reason: %s", res.BlockReason)
	}
}

func TestDNSRebindingDialRejection(t *testing.T) {
	// Simulate DNS rebinding: host looks like a public name but resolves to 169.254.169.254
	mockResolver := &net.Resolver{
		PreferGo: true,
	}

	mgr := NewDefenseManager(mockResolver)
	ctx := context.Background()

	// Verify CheckIP directly against blocked address
	ip := net.ParseIP("169.254.169.254")
	blocked, reason, isMeta := mgr.EvaluateIP(ip)
	if !blocked || !isMeta {
		t.Fatalf("expected 169.254.169.254 to be evaluated as blocked cloud metadata: blocked=%v, reason=%s", blocked, reason)
	}

	// Verify DialContext fails when trying to connect to blocked IP
	dialer := mgr.SafeDialer()
	conn, err := dialer.DialContext(ctx, "tcp", "169.254.169.254:80")
	if err == nil {
		if conn != nil {
			_ = conn.Close()
		}
		t.Fatal("expected SafeDialer to reject connection to 169.254.169.254, but succeeded")
	}
}

func TestDependencyConfusionAudit(t *testing.T) {
	tempDir := t.TempDir()
	lockfilePath := filepath.Join(tempDir, "package-lock.json")

	// Vulnerable package-lock.json where internal package @company/payment-service
	// is resolved to the public npm registry with inflated version 99.0.0
	maliciousLockfile := `{
  "name": "enterprise-app",
  "version": "1.0.0",
  "lockfileVersion": 2,
  "packages": {
    "node_modules/@company/payment-service": {
      "version": "99.0.0",
      "resolved": "https://registry.npmjs.org/@company/payment-service/-/payment-service-99.0.0.tgz"
    },
    "node_modules/lodash": {
      "version": "4.17.21",
      "resolved": "https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz"
    }
  }
}`

	if err := os.WriteFile(lockfilePath, []byte(maliciousLockfile), 0600); err != nil {
		t.Fatalf("failed to write lockfile: %v", err)
	}

	mgr := NewDefenseManager(nil)
	ctx := context.Background()

	report, err := mgr.AuditDependencies(ctx, lockfilePath, []string{"@company/"})
	if err == nil {
		t.Fatal("expected ErrSecDependencyConfusion error, got nil")
	}
	if err != domain.ErrSecDependencyConfusion {
		t.Fatalf("expected ErrSecDependencyConfusion, got %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.IsClean {
		t.Fatal("expected report.IsClean to be false")
	}
	if len(report.SuspiciousPackages) != 1 {
		t.Fatalf("expected 1 suspicious package, got %d", len(report.SuspiciousPackages))
	}
	if report.SuspiciousPackages[0].Name != "@company/payment-service" {
		t.Errorf("expected suspicious package name @company/payment-service, got %s", report.SuspiciousPackages[0].Name)
	}
}

func TestDependencyCleanAudit(t *testing.T) {
	tempDir := t.TempDir()
	lockfilePath := filepath.Join(tempDir, "package-lock.json")

	cleanLockfile := `{
  "name": "enterprise-app",
  "version": "1.0.0",
  "lockfileVersion": 2,
  "packages": {
    "node_modules/@company/payment-service": {
      "version": "1.4.2",
      "resolved": "https://npm.internal.company.corp/@company/payment-service/-/payment-service-1.4.2.tgz"
    },
    "node_modules/lodash": {
      "version": "4.17.21",
      "resolved": "https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz"
    }
  }
}`

	if err := os.WriteFile(lockfilePath, []byte(cleanLockfile), 0600); err != nil {
		t.Fatalf("failed to write lockfile: %v", err)
	}

	mgr := NewDefenseManager(nil)
	ctx := context.Background()

	report, err := mgr.AuditDependencies(ctx, lockfilePath, []string{"@company/"})
	if err != nil {
		t.Fatalf("unexpected error on clean lockfile: %v", err)
	}
	if !report.IsClean {
		t.Fatalf("expected clean report, but found suspicious packages: %+v", report.SuspiciousPackages)
	}
}

func TestValidateSidecarStartupOrdering(t *testing.T) {
	mgr := NewDefenseManager(nil)
	ctx := context.Background()

	// Case 1: Insecure pod - Envoy sidecar in regular containers
	insecurePodYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: order-service
spec:
  containers:
    - name: order-app
      image: app:1.0.0
    - name: envoy-sidecar
      image: envoyproxy/envoy:v1.28.0
`

	valid, err := mgr.ValidateSidecarStartupOrdering(ctx, []byte(insecurePodYAML))
	if valid || err != domain.ErrInvalidSidecarOrder {
		t.Fatalf("expected insecure pod to fail with ErrInvalidSidecarOrder, got valid=%v, err=%v", valid, err)
	}

	// Case 2: Insecure pod - Envoy in initContainers but restartPolicy NOT Always
	initWithoutAlwaysYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: order-service
spec:
  initContainers:
    - name: envoy-sidecar
      image: envoyproxy/envoy:v1.28.0
      restartPolicy: OnFailure
  containers:
    - name: order-app
      image: app:1.0.0
`

	valid2, err2 := mgr.ValidateSidecarStartupOrdering(ctx, []byte(initWithoutAlwaysYAML))
	if valid2 || err2 != domain.ErrInvalidSidecarOrder {
		t.Fatalf("expected initContainer without restartPolicy: Always to fail, got valid=%v, err=%v", valid2, err2)
	}

	// Case 3: Compliant pod - Envoy in initContainers with restartPolicy: Always (K8s 1.28+ native sidecar)
	compliantPodYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: order-service
spec:
  initContainers:
    - name: istio-proxy
      image: istio/proxyv2:1.20.0
      restartPolicy: Always
  containers:
    - name: order-app
      image: app:1.0.0
`

	valid3, err3 := mgr.ValidateSidecarStartupOrdering(ctx, []byte(compliantPodYAML))
	if !valid3 || err3 != nil {
		t.Fatalf("expected compliant native sidecar pod to pass, got valid=%v, err=%v", valid3, err3)
	}
}

func TestEnforceIMDSv2(t *testing.T) {
	mgr := NewDefenseManager(nil)
	ctx := context.Background()

	// Empty instance ID should fail
	if err := mgr.EnforceIMDSv2(ctx, ""); err == nil {
		t.Fatal("expected error for empty instance ID, got nil")
	}

	// Valid EC2 instance ID
	if err := mgr.EnforceIMDSv2(ctx, "i-0123456789abcdef0"); err != nil {
		t.Fatalf("expected successful IMDSv2 enforcement, got: %v", err)
	}
}

func TestPaddedMetricsConcurrentUnderDefenseLoad(t *testing.T) {
	mgr := NewDefenseManager(nil)
	ctx := context.Background()

	const workers = 20
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = mgr.ValidateURLAgainstSSRF(ctx, "http://169.254.169.254/latest/meta-data/")
				_, _ = mgr.ValidateURLAgainstSSRF(ctx, "http://[fd00:ec2::254]/latest/meta-data/")
				_ = mgr.EnforceIMDSv2(ctx, "i-test")
			}
		}()
	}

	wg.Wait()

	metrics := mgr.Metrics()
	if metrics.SSRFValidationsTotal.Load() == 0 {
		t.Fatal("expected SSRFValidationsTotal to be greater than 0")
	}
	if metrics.MetadataBlockedTotal.Load() == 0 {
		t.Fatal("expected MetadataBlockedTotal to be greater than 0")
	}
}
