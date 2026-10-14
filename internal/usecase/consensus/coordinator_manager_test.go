package consensus_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domainconsensus "github.com/datdt/k8sselfhost/internal/domain/consensus"
	usecaseconsensus "github.com/datdt/k8sselfhost/internal/usecase/consensus"
)

// fakeEtcdClient provides an in-memory, thread-safe test double for etcd operations.
type fakeEtcdClient struct {
	mu           sync.Mutex
	members      []*domainconsensus.EtcdMemberStatus
	defragOrder  []string
	stepDownFrom string
	stepDownTo   string
	stepDownErr  error
	defragErr    error
}

func newFakeEtcdClient(members []*domainconsensus.EtcdMemberStatus) *fakeEtcdClient {
	return &fakeEtcdClient{
		members:     members,
		defragOrder: make([]string, 0),
	}
}

func (f *fakeEtcdClient) ListMembers(ctx context.Context) ([]*domainconsensus.EtcdMemberStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	res := make([]*domainconsensus.EtcdMemberStatus, len(f.members))
	copy(res, f.members)
	return res, nil
}

func (f *fakeEtcdClient) MemberStatus(ctx context.Context, endpoint string) (*domainconsensus.EtcdMemberStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.members {
		if m.Endpoint == endpoint {
			return m, nil
		}
	}
	return nil, fmt.Errorf("member not found for endpoint %s", endpoint)
}

func (f *fakeEtcdClient) Defragment(ctx context.Context, endpoint string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.defragErr != nil {
		return f.defragErr
	}
	f.defragOrder = append(f.defragOrder, endpoint)
	return nil
}

func (f *fakeEtcdClient) MoveLeader(ctx context.Context, transfereeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stepDownErr != nil {
		return f.stepDownErr
	}
	var oldLeaderID string
	for _, m := range f.members {
		if m.IsLeader {
			oldLeaderID = m.ID
			m.IsLeader = false
		}
	}
	for _, m := range f.members {
		if m.ID == transfereeID {
			m.IsLeader = true
		}
	}
	f.stepDownFrom = oldLeaderID
	f.stepDownTo = transfereeID
	return nil
}

// fakePgBouncerClient provides an in-memory test double for PgBouncer interactions.
type fakePgBouncerClient struct {
	mu            sync.Mutex
	paused        bool
	resumed       bool
	relocatedAddr string
	killedCount   int
	killedHost    string
	pauseErr      error
	resumeErr     error
	killErr       error
	relocateErr   error
}

func (p *fakePgBouncerClient) Pause(ctx context.Context, poolerAddr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pauseErr != nil {
		return p.pauseErr
	}
	p.paused = true
	return nil
}

func (p *fakePgBouncerClient) Resume(ctx context.Context, poolerAddr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.resumeErr != nil {
		return p.resumeErr
	}
	p.resumed = true
	return nil
}

func (p *fakePgBouncerClient) KillStaleConnections(ctx context.Context, poolerAddr, staleHost string) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.killErr != nil {
		return 0, p.killErr
	}
	p.killedCount = 14
	p.killedHost = staleHost
	return 14, nil
}

func (p *fakePgBouncerClient) RelocatePool(ctx context.Context, poolerAddr, newMasterAddr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.relocateErr != nil {
		return p.relocateErr
	}
	p.relocatedAddr = newMasterAddr
	return nil
}

func TestCoordinatorManager_SafeDefragCluster_Success(t *testing.T) {
	ctx := context.Background()

	m1, _ := domainconsensus.NewEtcdMemberStatus("m1", "http://10.0.0.1:2379", false, 100, 50, 1000)
	m2, _ := domainconsensus.NewEtcdMemberStatus("m2", "http://10.0.0.2:2379", false, 100, 50, 1000)
	m3, _ := domainconsensus.NewEtcdMemberStatus("m3", "http://10.0.0.3:2379", true, 100, 50, 1000) // Leader

	etcdClient := newFakeEtcdClient([]*domainconsensus.EtcdMemberStatus{m1, m2, m3})
	pgClient := &fakePgBouncerClient{}

	mgr := usecaseconsensus.NewCoordinatorManager(
		etcdClient,
		pgClient,
		usecaseconsensus.WithCooldownDuration(1*time.Millisecond),
	)

	err := mgr.SafeDefragCluster(ctx, []string{m1.Endpoint, m2.Endpoint, m3.Endpoint})
	if err != nil {
		t.Fatalf("expected successful defrag, got err: %v", err)
	}

	etcdClient.mu.Lock()
	order := etcdClient.defragOrder
	stepDownTo := etcdClient.stepDownTo
	etcdClient.mu.Unlock()

	if len(order) != 3 {
		t.Fatalf("expected 3 nodes defragged, got %d (%v)", len(order), order)
	}

	// Verify non-leaders were defragged first, and leader was defragged last
	if order[0] != m1.Endpoint && order[0] != m2.Endpoint {
		t.Errorf("expected follower defragged first, got %s", order[0])
	}
	if order[1] != m1.Endpoint && order[1] != m2.Endpoint {
		t.Errorf("expected follower defragged second, got %s", order[1])
	}
	if order[2] != m3.Endpoint {
		t.Errorf("expected leader defragged last after stepdown, got %s", order[2])
	}
	if stepDownTo == "" {
		t.Errorf("expected leader stepdown transferee to be set")
	}

	// State machine check
	if mgr.GetDefragStateMachine().CurrentState() != domainconsensus.DefragStateCompleted {
		t.Errorf("expected defrag state machine to be Completed, got %v", mgr.GetDefragStateMachine().CurrentState())
	}
}

func TestCoordinatorManager_SafeDefragCluster_QuorumLost(t *testing.T) {
	ctx := context.Background()

	m1, _ := domainconsensus.NewEtcdMemberStatus("m1", "http://10.0.0.1:2379", true, 100, 50, 1000)

	// Single node cluster requires quorum 1, but if only 1 node with total 3 specified:
	etcdClient := newFakeEtcdClient([]*domainconsensus.EtcdMemberStatus{m1})
	pgClient := &fakePgBouncerClient{}

	mgr := usecaseconsensus.NewCoordinatorManager(
		etcdClient,
		pgClient,
		usecaseconsensus.WithCooldownDuration(1*time.Millisecond),
	)

	// Specified 3 endpoints, but only 1 healthy member in cluster -> Quorum Lost (1 < 3/2 + 1)
	err := mgr.SafeDefragCluster(ctx, []string{"http://10.0.0.1:2379", "http://10.0.0.2:2379", "http://10.0.0.3:2379"})
	if !errors.Is(err, domainconsensus.ErrConsensusQuorumLost) {
		t.Fatalf("expected ErrConsensusQuorumLost, got: %v", err)
	}
}

func TestCoordinatorManager_SafeDefragCluster_LeaderStepdownBlocked(t *testing.T) {
	ctx := context.Background()

	m1, _ := domainconsensus.NewEtcdMemberStatus("m1", "http://10.0.0.1:2379", false, 100, 50, 1000)
	m2, _ := domainconsensus.NewEtcdMemberStatus("m2", "http://10.0.0.2:2379", true, 100, 50, 1000) // Leader

	etcdClient := newFakeEtcdClient([]*domainconsensus.EtcdMemberStatus{m1, m2})
	etcdClient.stepDownErr = errors.New("raft election timeout on stepdown")
	pgClient := &fakePgBouncerClient{}

	mgr := usecaseconsensus.NewCoordinatorManager(
		etcdClient,
		pgClient,
		usecaseconsensus.WithCooldownDuration(1*time.Millisecond),
	)

	err := mgr.SafeDefragCluster(ctx, []string{m1.Endpoint, m2.Endpoint})
	if !errors.Is(err, domainconsensus.ErrEtcdDefragLeaderBlocked) {
		t.Fatalf("expected ErrEtcdDefragLeaderBlocked, got: %v", err)
	}
}

func TestCoordinatorManager_AcquireLockAndValidateFencing_Monotonic(t *testing.T) {
	ctx := context.Background()
	etcdClient := newFakeEtcdClient(nil)
	pgClient := &fakePgBouncerClient{}

	mgr := usecaseconsensus.NewCoordinatorManager(etcdClient, pgClient)

	// Acquire lock for key
	token1, err := mgr.AcquireLockWithFencing(ctx, "patroni/master", "client-1", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error acquiring lock: %v", err)
	}
	if token1.Generation <= 0 {
		t.Fatalf("expected positive generation token, got %d", token1.Generation)
	}

	// First validation commits generation 1 -> Valid
	valid, err := mgr.ValidateFencingToken(ctx, "patroni/master", token1.Generation)
	if err != nil || !valid {
		t.Fatalf("expected valid token on first commit, got valid=%v, err=%v", valid, err)
	}

	// Repeated validation with token 1 is stale (1 <= committed 1)
	valid, err = mgr.ValidateFencingToken(ctx, "patroni/master", token1.Generation)
	if !errors.Is(err, domainconsensus.ErrFencingTokenStale) {
		t.Fatalf("expected ErrFencingTokenStale on duplicate generation, got err=%v", err)
	}
	if valid {
		t.Errorf("expected valid to be false")
	}

	// Client 2 acquires lock -> gets generation 2
	token2, err := mgr.AcquireLockWithFencing(ctx, "patroni/master", "client-2", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token2.Generation <= token1.Generation {
		t.Fatalf("expected token2 generation (%d) > token1 generation (%d)", token2.Generation, token1.Generation)
	}

	// Old Client 1 awakes and attempts write with generation 1 -> Stale Rejection
	valid, err = mgr.ValidateFencingToken(ctx, "patroni/master", token1.Generation)
	if !errors.Is(err, domainconsensus.ErrFencingTokenStale) {
		t.Fatalf("expected ErrFencingTokenStale for old client token, got err=%v", err)
	}
	if valid {
		t.Errorf("expected valid=false for stale generation")
	}

	// Client 2 with generation 2 validates and commits -> Valid
	valid, err = mgr.ValidateFencingToken(ctx, "patroni/master", token2.Generation)
	if err != nil || !valid {
		t.Fatalf("expected valid token2 commit, got valid=%v, err=%v", valid, err)
	}
}

func TestCoordinatorManager_FlushPgBouncerStaleConnections(t *testing.T) {
	ctx := context.Background()
	etcdClient := newFakeEtcdClient(nil)
	pgClient := &fakePgBouncerClient{}

	mgr := usecaseconsensus.NewCoordinatorManager(etcdClient, pgClient)

	// Explicitly register active primary for pooler
	mgr.SetActivePrimary("10.0.0.10:6432", "10.0.0.1:5432")
	if primary := mgr.GetActivePrimary("10.0.0.10:6432"); primary != "10.0.0.1:5432" {
		t.Fatalf("expected active primary 10.0.0.1:5432, got %s", primary)
	}

	// First failover: severs connections to 10.0.0.1:5432 and relocates to 10.0.0.2:5432
	err := mgr.FlushPgBouncerStaleConnections(ctx, "10.0.0.10:6432", "10.0.0.2:5432")
	if err != nil {
		t.Fatalf("unexpected error flushing pgbouncer: %v", err)
	}

	pgClient.mu.Lock()
	paused := pgClient.paused
	resumed := pgClient.resumed
	relocated := pgClient.relocatedAddr
	killed := pgClient.killedCount
	killedHost := pgClient.killedHost
	pgClient.mu.Unlock()

	if !paused || !resumed {
		t.Errorf("expected pause and resume both true, got paused=%v, resumed=%v", paused, resumed)
	}
	if relocated != "10.0.0.2:5432" {
		t.Errorf("expected relocated to new master, got %s", relocated)
	}
	if killed == 0 {
		t.Errorf("expected killed stale connections > 0, got %d", killed)
	}
	if killedHost != "10.0.0.1:5432" {
		t.Errorf("expected demoted primary 10.0.0.1:5432 to be killed, got: %s", killedHost)
	}
	if current := mgr.GetActivePrimary("10.0.0.10:6432"); current != "10.0.0.2:5432" {
		t.Errorf("expected updated active primary 10.0.0.2:5432, got: %s", current)
	}

	// Second failover: automatically severs previous master 10.0.0.2:5432 and relocates to 10.0.0.3:5432
	err = mgr.FlushPgBouncerStaleConnections(ctx, "10.0.0.10:6432", "10.0.0.3:5432")
	if err != nil {
		t.Fatalf("unexpected error on second flush: %v", err)
	}
	pgClient.mu.Lock()
	secondKilledHost := pgClient.killedHost
	pgClient.mu.Unlock()
	if secondKilledHost != "10.0.0.2:5432" {
		t.Errorf("expected second demoted primary 10.0.0.2:5432 to be killed, got: %s", secondKilledHost)
	}
}

func TestCoordinatorManager_FencingToken_ExpiredLeaseRejected(t *testing.T) {
	ctx := context.Background()
	mgr := usecaseconsensus.NewCoordinatorManager(newFakeEtcdClient(nil), &fakePgBouncerClient{})

	token, err := mgr.AcquireLockWithFencing(ctx, "resource/lease", "client-a", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sleep until lease expires
	time.Sleep(25 * time.Millisecond)

	valid, err := mgr.ValidateFencingToken(ctx, "resource/lease", token.Generation)
	if valid {
		t.Errorf("expected valid to be false for expired lease")
	}
	if !errors.Is(err, domainconsensus.ErrFencingTokenStale) {
		t.Fatalf("expected ErrFencingTokenStale for expired lease, got: %v", err)
	}
}

func TestCoordinatorManager_ConcurrentLockAcquisition(t *testing.T) {
	ctx := context.Background()
	etcdClient := newFakeEtcdClient(nil)
	pgClient := &fakePgBouncerClient{}

	mgr := usecaseconsensus.NewCoordinatorManager(etcdClient, pgClient)

	var wg sync.WaitGroup
	var generations sync.Map
	concurrency := 50
	var duplicates atomic.Int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			token, err := mgr.AcquireLockWithFencing(ctx, "global/resource", fmt.Sprintf("worker-%d", workerID), 1*time.Minute)
			if err != nil {
				t.Errorf("worker %d acquire error: %v", workerID, err)
			} else if _, loaded := generations.LoadOrStore(token.Generation, true); loaded {
				duplicates.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if duplicates.Load() > 0 {
		t.Errorf("detected %d duplicate generation tokens across concurrent goroutines", duplicates.Load())
	}
}

type fakeLeaseStore struct {
	mu     sync.Mutex
	leases map[string]*domainconsensus.FencingLease
}

func newFakeLeaseStore() *fakeLeaseStore {
	return &fakeLeaseStore{leases: make(map[string]*domainconsensus.FencingLease)}
}

func (s *fakeLeaseStore) AcquireOrRenewLease(ctx context.Context, clusterID, holderID string, ttl time.Duration) (*domainconsensus.FencingLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if cur, ok := s.leases[clusterID]; ok {
		if now.Before(cur.ExpiresAt) && cur.HolderID != holderID {
			return nil, domainconsensus.ErrLeaseConflict
		}
		lease := &domainconsensus.FencingLease{ClusterID: clusterID, HolderID: holderID, FencingToken: cur.FencingToken + 1, AcquiredAt: now, ExpiresAt: now.Add(ttl)}
		s.leases[clusterID] = lease
		return lease, nil
	}
	lease := &domainconsensus.FencingLease{ClusterID: clusterID, HolderID: holderID, FencingToken: 1, AcquiredAt: now, ExpiresAt: now.Add(ttl)}
	s.leases[clusterID] = lease
	return lease, nil
}

func (s *fakeLeaseStore) GetActivePrimary(ctx context.Context, clusterID string) (string, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cur, ok := s.leases[clusterID]; ok {
		return cur.HolderID, cur.FencingToken, nil
	}
	return "", 0, nil
}

func (s *fakeLeaseStore) ValidateToken(ctx context.Context, clusterID string, token int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.leases[clusterID]
	if !ok || token != cur.FencingToken {
		return false, domainconsensus.ErrFencingTokenStale
	}
	if time.Now().UTC().After(cur.ExpiresAt) {
		return false, fmt.Errorf("%w: lock lease has expired", domainconsensus.ErrFencingTokenStale)
	}
	return true, nil
}

func TestCoordinatorManager_MultiReplicaHA_SharedLeaseStore(t *testing.T) {
	ctx := context.Background()
	sharedStore := newFakeLeaseStore()
	pgClientA := &fakePgBouncerClient{}
	pgClientB := &fakePgBouncerClient{}

	replicaA := usecaseconsensus.NewCoordinatorManager(
		newFakeEtcdClient(nil),
		pgClientA,
		usecaseconsensus.WithLeaseStore(sharedStore),
	)
	replicaB := usecaseconsensus.NewCoordinatorManager(
		newFakeEtcdClient(nil),
		pgClientB,
		usecaseconsensus.WithLeaseStore(sharedStore),
	)

	clusterID := "pg-cluster-prod"
	ttl := 20 * time.Millisecond

	// 1. Replica A acquires lease (token 1); Replica B is rejected while Replica A's lease is active
	leaseA, err := replicaA.AcquireLease(ctx, clusterID, "replica-a", ttl)
	if err != nil || leaseA.FencingToken != 1 || leaseA.HolderID != "replica-a" {
		t.Fatalf("unexpected replica A acquire: lease=%+v, err=%v", leaseA, err)
	}

	_, err = replicaB.AcquireLease(ctx, clusterID, "replica-b", ttl)
	if !errors.Is(err, domainconsensus.ErrLeaseConflict) {
		t.Fatalf("expected ErrLeaseConflict for replica B, got: %v", err)
	}

	// 2. When Replica A's lease expires, Replica B acquires lease (token 2 > token 1)
	time.Sleep(25 * time.Millisecond)

	leaseB, err := replicaB.AcquireLease(ctx, clusterID, "replica-b", ttl)
	if err != nil || leaseB.FencingToken != 2 || leaseB.HolderID != "replica-b" {
		t.Fatalf("expected replica B token 2, got: %+v (err=%v)", leaseB, err)
	}

	// 3. Replica A's stale token 1 is rejected by ValidateFencingToken
	valid, err := replicaA.ValidateFencingToken(ctx, clusterID, leaseA.FencingToken)
	if valid || !errors.Is(err, domainconsensus.ErrFencingTokenStale) {
		t.Fatalf("expected ErrFencingTokenStale for replica A, got: valid=%v, err=%v", valid, err)
	}

	// 4. ExecuteStonithFence fences Replica A dynamically
	fencedNode, err := replicaB.ExecuteStonithFence(ctx, clusterID)
	if err != nil || fencedNode != "replica-a" || pgClientB.killedHost != "replica-a" {
		t.Fatalf("expected fenced node replica-a with severed connections, got fenced=%s (host=%s, err=%v)", fencedNode, pgClientB.killedHost, err)
	}
}

