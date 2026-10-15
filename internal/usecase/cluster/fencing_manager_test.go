package cluster_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	domainCluster "github.com/datdt/k8sselfhost/internal/domain/cluster"
	usecaseCluster "github.com/datdt/k8sselfhost/internal/usecase/cluster"
)

// MockSTONITHExecutor verifies hardware execution commands.
type mockSTONITHExecutor struct {
	powerOffErr     error
	cloudStopErr    error
	networkIsoErr   error
	powerOffCalls   int
	cloudStopCalls  int
	networkIsoCalls int
}

func (m *mockSTONITHExecutor) PowerOffIPMI(ctx context.Context, nodeID, dc string) error {
	m.powerOffCalls++
	return m.powerOffErr
}

func (m *mockSTONITHExecutor) StopCloudInstance(ctx context.Context, nodeID, dc string) error {
	m.cloudStopCalls++
	return m.cloudStopErr
}

func (m *mockSTONITHExecutor) IsolateNetwork(ctx context.Context, nodeID, dc string) error {
	m.networkIsoCalls++
	return m.networkIsoErr
}

// MockFencingChannelVerifier simulates independent multi-channel verification.
type mockChannelVerifier struct {
	powerOffConfirmed bool
	powerOffErr       error
	pingLossConfirmed bool
	pingLossErr       error
	storageConfirmed  bool
	storageErr        error
}

func (m *mockChannelVerifier) VerifyPowerOff(ctx context.Context, nodeID, dc string) (bool, error) {
	return m.powerOffConfirmed, m.powerOffErr
}

func (m *mockChannelVerifier) VerifyNetworkLoss(ctx context.Context, nodeID, dc string) (bool, error) {
	return m.pingLossConfirmed, m.pingLossErr
}

func (m *mockChannelVerifier) VerifyStorageReservation(ctx context.Context, nodeID, dc string) (bool, error) {
	return m.storageConfirmed, m.storageErr
}

// MockClusterStateProvider simulates cluster metadata and topology.
type mockClusterStateProvider struct {
	primaryNodeID  string
	replicationLag time.Duration
	err            error
}

func (m *mockClusterStateProvider) GetClusterPrimaryNode(ctx context.Context, clusterID string) (string, time.Duration, error) {
	return m.primaryNodeID, m.replicationLag, m.err
}

// MockK8sSecurityApplier simulates NetworkPolicy and IMDS enforcement.
type mockK8sSecurityApplier struct {
	appliedPolicies map[string][]byte
	nodeConfigs     map[string]*usecaseCluster.NodeIMDSConfig
	applyErr        error
	enforceErr      error
}

func newMockK8sSecurityApplier() *mockK8sSecurityApplier {
	return &mockK8sSecurityApplier{
		appliedPolicies: make(map[string][]byte),
		nodeConfigs:     make(map[string]*usecaseCluster.NodeIMDSConfig),
	}
}

func (m *mockK8sSecurityApplier) ApplyNetworkPolicy(ctx context.Context, clusterID string, policyYAML []byte) error {
	if m.applyErr != nil {
		return m.applyErr
	}
	m.appliedPolicies[clusterID] = policyYAML
	return nil
}

func (m *mockK8sSecurityApplier) ListClusterNodes(ctx context.Context, clusterID string) ([]string, error) {
	nodes := make([]string, 0, len(m.nodeConfigs))
	for n := range m.nodeConfigs {
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (m *mockK8sSecurityApplier) GetNodeIMDSConfig(ctx context.Context, clusterID, nodeID string) (*usecaseCluster.NodeIMDSConfig, error) {
	cfg, ok := m.nodeConfigs[nodeID]
	if !ok {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}
	return cfg, nil
}

func (m *mockK8sSecurityApplier) EnforceNodeIMDSv2(ctx context.Context, clusterID, nodeID string) error {
	if m.enforceErr != nil {
		return m.enforceErr
	}
	if cfg, ok := m.nodeConfigs[nodeID]; ok {
		cfg.HttpTokens = "required"
		cfg.HttpPutResponseHopLimit = 1
	}
	return nil
}

func TestExecuteSTONITH_MultiChannelVerificationSuccess(t *testing.T) {
	ctx := context.Background()
	executor := &mockSTONITHExecutor{}
	verifier := &mockChannelVerifier{
		powerOffConfirmed: true,
		pingLossConfirmed: true,
	}
	stateProvider := &mockClusterStateProvider{primaryNodeID: "node-1"}
	k8sApplier := newMockK8sSecurityApplier()

	manager := usecaseCluster.NewFencingManager(executor, verifier, stateProvider, k8sApplier, zap.NewNop())
	report, err := manager.ExecuteSTONITH(ctx, "node-1", "dc-east")

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "node-1", report.TargetNodeID)
	assert.Equal(t, "dc-east", report.Datacenter)
	assert.Equal(t, domainCluster.IPMI_POWER_OFF, report.Method)
	assert.True(t, report.IsFenced, "Should be fenced when at least 2 independent channels confirm")
	assert.True(t, report.ConfirmedAt.After(time.Time{}))
	assert.Contains(t, report.OutputLog[len(report.OutputLog)-1], "Multi-channel verification confirmed")
}

func TestExecuteSTONITH_MultiChannelVerificationFailure_OnlyOneChannel(t *testing.T) {
	ctx := context.Background()
	executor := &mockSTONITHExecutor{}
	// Channel 1 confirms power off, but Channel 2 fails ping loss verification (host still replies)
	verifier := &mockChannelVerifier{
		powerOffConfirmed: true,
		pingLossConfirmed: false,
	}
	stateProvider := &mockClusterStateProvider{primaryNodeID: "node-1"}
	k8sApplier := newMockK8sSecurityApplier()

	manager := usecaseCluster.NewFencingManager(executor, verifier, stateProvider, k8sApplier, zap.NewNop())
	report, err := manager.ExecuteSTONITH(ctx, "node-1", "dc-east")

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.False(t, report.IsFenced, "Must NOT mark fenced if only 1 channel confirms (split-brain risk)")
	assert.Contains(t, report.OutputLog[len(report.OutputLog)-1], "Multi-channel verification failed")
}

func TestValidateFailoverReadiness_PrimaryNotFenced_RejectsWithSplitBrainWarning(t *testing.T) {
	ctx := context.Background()
	executor := &mockSTONITHExecutor{}
	verifier := &mockChannelVerifier{}
	stateProvider := &mockClusterStateProvider{
		primaryNodeID:  "old-primary-01",
		replicationLag: 12 * time.Millisecond,
	}
	k8sApplier := newMockK8sSecurityApplier()

	manager := usecaseCluster.NewFencingManager(executor, verifier, stateProvider, k8sApplier, zap.NewNop())
	status, err := manager.ValidateFailoverReadiness(ctx, "prod-cluster")

	require.Error(t, err)
	assert.True(t, errors.Is(err, domainCluster.ErrFailoverPrimaryNotFenced))
	require.NotNil(t, status)
	assert.False(t, status.CanPromote)
	assert.False(t, status.OldPrimaryFenced)
	assert.Contains(t, status.RejectReason, "Old primary node 'old-primary-01' is not confirmed fenced. Promoting replica risks split-brain writes.")
}

func TestValidateFailoverReadiness_GracePeriodActive_RejectsFailover(t *testing.T) {
	ctx := context.Background()
	executor := &mockSTONITHExecutor{}
	verifier := &mockChannelVerifier{}
	stateProvider := &mockClusterStateProvider{primaryNodeID: "node-db-01"}
	k8sApplier := newMockK8sSecurityApplier()

	manager := usecaseCluster.NewFencingManager(executor, verifier, stateProvider, k8sApplier, zap.NewNop())
	manager.SetGracePeriod(60 * time.Second)

	// Simulate heartbeat lost 15 seconds ago (within 60s grace period)
	manager.RecordHeartbeatLoss("node-db-01", time.Now().Add(-15*time.Second))

	status, err := manager.ValidateFailoverReadiness(ctx, "prod-cluster")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domainCluster.ErrGracePeriodActive))
	require.NotNil(t, status)
	assert.False(t, status.CanPromote)
	assert.False(t, status.GraceDelayPassed)
	assert.Contains(t, status.RejectReason, "Grace period active")
}

func TestValidateFailoverReadiness_HeartbeatRecovery_AbortsFailover(t *testing.T) {
	executor := &mockSTONITHExecutor{}
	verifier := &mockChannelVerifier{}
	stateProvider := &mockClusterStateProvider{primaryNodeID: "node-db-01"}
	k8sApplier := newMockK8sSecurityApplier()

	manager := usecaseCluster.NewFencingManager(executor, verifier, stateProvider, k8sApplier, zap.NewNop())
	manager.SetGracePeriod(60 * time.Second)

	// Mark heartbeat lost
	manager.RecordHeartbeatLoss("node-db-01", time.Now().Add(-20*time.Second))

	// Heartbeat recovers within grace period (transient network hiccup resolved)
	aborted := manager.RecordHeartbeatRecovery("node-db-01")
	assert.True(t, aborted, "Failover should be aborted when primary recovers within grace period")

	tracker := manager.GetHeartbeatTracker("node-db-01")
	require.NotNil(t, tracker)
	assert.Equal(t, domainCluster.HeartbeatStatusHealthy, tracker.GetState())
}

func TestValidateFailoverReadiness_FencedAndGracePassed_AllowsPromotion(t *testing.T) {
	ctx := context.Background()
	executor := &mockSTONITHExecutor{}
	verifier := &mockChannelVerifier{
		powerOffConfirmed: true,
		pingLossConfirmed: true,
	}
	stateProvider := &mockClusterStateProvider{
		primaryNodeID:  "old-primary-01",
		replicationLag: 5 * time.Millisecond,
	}
	k8sApplier := newMockK8sSecurityApplier()

	manager := usecaseCluster.NewFencingManager(executor, verifier, stateProvider, k8sApplier, zap.NewNop())
	manager.SetGracePeriod(60 * time.Second)

	// Simulate heartbeat loss 75 seconds ago (> 60s grace period)
	manager.RecordHeartbeatLoss("old-primary-01", time.Now().Add(-75*time.Second))

	// Execute STONITH with confirmed multi-channel isolation
	report, fenceErr := manager.ExecuteSTONITH(ctx, "old-primary-01", "dc-east")
	require.NoError(t, fenceErr)
	require.True(t, report.IsFenced)

	// Now validate promotion readiness
	status, err := manager.ValidateFailoverReadiness(ctx, "prod-cluster")
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.CanPromote)
	assert.True(t, status.OldPrimaryFenced)
	assert.True(t, status.GraceDelayPassed)
	assert.Empty(t, status.RejectReason)
}

func TestEnforceIMDSv2Security_GeneratesZeroTrustNetworkPolicyAndHardensNodes(t *testing.T) {
	ctx := context.Background()
	executor := &mockSTONITHExecutor{}
	verifier := &mockChannelVerifier{}
	stateProvider := &mockClusterStateProvider{primaryNodeID: "node-1"}
	k8sApplier := newMockK8sSecurityApplier()

	// Setup cluster nodes with IMDSv1 configuration (insecure)
	k8sApplier.nodeConfigs["node-1"] = &usecaseCluster.NodeIMDSConfig{
		NodeID:                  "node-1",
		HttpTokens:              "optional", // Insecure IMDSv1 allowed
		HttpPutResponseHopLimit: 2,
	}
	k8sApplier.nodeConfigs["node-2"] = &usecaseCluster.NodeIMDSConfig{
		NodeID:                  "node-2",
		HttpTokens:              "required",
		HttpPutResponseHopLimit: 1,
	}

	manager := usecaseCluster.NewFencingManager(executor, verifier, stateProvider, k8sApplier, zap.NewNop())
	err := manager.EnforceIMDSv2Security(ctx, "cluster-k8s-01")
	require.NoError(t, err)

	// Verify NetworkPolicy applied blocking 169.254.169.254/32
	policyBytes, exists := k8sApplier.appliedPolicies["cluster-k8s-01"]
	require.True(t, exists)
	policyStr := string(policyBytes)
	assert.Contains(t, policyStr, "169.254.169.254/32")
	assert.Contains(t, policyStr, "block-imds-metadata-access")

	// Verify node-1 was hardened to IMDSv2
	assert.Equal(t, "required", k8sApplier.nodeConfigs["node-1"].HttpTokens)
	assert.Equal(t, 1, k8sApplier.nodeConfigs["node-1"].HttpPutResponseHopLimit)
}

func TestEnforceIMDSv2Security_BlockedAccessFailsWithDomainError(t *testing.T) {
	ctx := context.Background()
	executor := &mockSTONITHExecutor{}
	verifier := &mockChannelVerifier{}
	stateProvider := &mockClusterStateProvider{primaryNodeID: "node-1"}
	k8sApplier := newMockK8sSecurityApplier()
	k8sApplier.nodeConfigs["node-1"] = &usecaseCluster.NodeIMDSConfig{
		NodeID:                  "node-1",
		HttpTokens:              "optional",
		HttpPutResponseHopLimit: 2,
	}
	// Simulate enforcement failure (e.g. cloud provider IAM token lockout)
	k8sApplier.enforceErr = errors.New("metadata token locked: unauthorized access blocked")

	manager := usecaseCluster.NewFencingManager(executor, verifier, stateProvider, k8sApplier, zap.NewNop())
	err := manager.EnforceIMDSv2Security(ctx, "cluster-k8s-01")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domainCluster.ErrIMDSAccessBlocked))
}

func TestConcurrentHeartbeatAndFencing_RaceSafety(t *testing.T) {
	ctx := context.Background()
	executor := &mockSTONITHExecutor{}
	verifier := &mockChannelVerifier{
		powerOffConfirmed: true,
		pingLossConfirmed: true,
	}
	stateProvider := &mockClusterStateProvider{primaryNodeID: "node-1"}
	k8sApplier := newMockK8sSecurityApplier()

	manager := usecaseCluster.NewFencingManager(executor, verifier, stateProvider, k8sApplier, zap.NewNop())

	var wg sync.WaitGroup
	const routines = 10

	for i := 0; i < routines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			nodeID := fmt.Sprintf("node-%d", id%3)
			for j := 0; j < 50; j++ {
				manager.RecordHeartbeatLoss(nodeID, time.Now().Add(-10*time.Second))
				manager.RecordHeartbeatRecovery(nodeID)
				_, _ = manager.ExecuteSTONITH(ctx, nodeID, "dc-west")
			}
		}(i)
	}

	wg.Wait()
}

func TestFencingManager_NilVerifierFailsExplicitly(t *testing.T) {
	ctx := context.Background()
	executor := &mockSTONITHExecutor{}
	stateProvider := &mockClusterStateProvider{primaryNodeID: "node-1"}
	k8sApplier := newMockK8sSecurityApplier()

	manager := usecaseCluster.NewFencingManager(executor, nil, stateProvider, k8sApplier, zap.NewNop())
	_, err := manager.ExecuteSTONITH(ctx, "node-1", "dc-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fencing channel verifier is unconfigured")
}

func TestK8sNodeVerifier_Success(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "dead-node"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
					{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionTrue},
				},
			},
		},
	)

	verifier := usecaseCluster.NewK8sNodeVerifier(client)
	powerOff, err := verifier.VerifyPowerOff(ctx, "dead-node", "dc-1")
	require.NoError(t, err)
	assert.True(t, powerOff)

	netLoss, err := verifier.VerifyNetworkLoss(ctx, "dead-node", "dc-1")
	require.NoError(t, err)
	assert.True(t, netLoss)

	storageOk, err := verifier.VerifyStorageReservation(ctx, "dead-node", "dc-1")
	require.NoError(t, err)
	assert.True(t, storageOk)
}
