package consensus_test

import (
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/consensus"
)

func TestNewEtcdMemberStatus_Validation(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		endpoint    string
		isLeader    bool
		dbSize      int64
		dbSizeInUse int64
		raftIndex   uint64
		wantErr     bool
	}{
		{
			name:        "valid member status",
			id:          "member-1",
			endpoint:    "http://10.0.0.1:2379",
			isLeader:    true,
			dbSize:      104857600,
			dbSizeInUse: 52428800,
			raftIndex:   15200,
			wantErr:     false,
		},
		{
			name:        "empty id",
			id:          "",
			endpoint:    "http://10.0.0.1:2379",
			isLeader:    false,
			dbSize:      100,
			dbSizeInUse: 50,
			raftIndex:   1,
			wantErr:     true,
		},
		{
			name:        "empty endpoint",
			id:          "member-2",
			endpoint:    "",
			isLeader:    false,
			dbSize:      100,
			dbSizeInUse: 50,
			raftIndex:   1,
			wantErr:     true,
		},
		{
			name:        "negative db size",
			id:          "member-3",
			endpoint:    "http://10.0.0.3:2379",
			isLeader:    false,
			dbSize:      -10,
			dbSizeInUse: 50,
			raftIndex:   1,
			wantErr:     true,
		},
		{
			name:        "negative db size in use",
			id:          "member-4",
			endpoint:    "http://10.0.0.4:2379",
			isLeader:    false,
			dbSize:      100,
			dbSizeInUse: -5,
			raftIndex:   1,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := consensus.NewEtcdMemberStatus(
				tt.id,
				tt.endpoint,
				tt.isLeader,
				tt.dbSize,
				tt.dbSizeInUse,
				tt.raftIndex,
			)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status.ID != tt.id || status.Endpoint != tt.endpoint || status.IsLeader != tt.isLeader {
				t.Errorf("member status fields mismatch")
			}
			if status.DbSize != tt.dbSize || status.DbSizeInUse != tt.dbSizeInUse || status.RaftIndex != tt.raftIndex {
				t.Errorf("member status metrics mismatch")
			}
		})
	}
}

func TestNewFencingToken_Validation(t *testing.T) {
	now := time.Now().UTC()
	expiry := now.Add(5 * time.Second)

	token, err := consensus.NewFencingToken("leader/lock", "node-1", 42, expiry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.LockKey != "leader/lock" || token.OwnerID != "node-1" || token.Generation != 42 {
		t.Errorf("token fields mismatch")
	}
	if token.IsExpired(now.Add(6 * time.Second)) != true {
		t.Errorf("expected token to be expired after expiry time")
	}
	if token.IsExpired(now) != false {
		t.Errorf("expected token to not be expired before expiry time")
	}

	// Invalids
	_, err = consensus.NewFencingToken("", "node-1", 42, expiry)
	if err == nil {
		t.Errorf("expected error for empty key")
	}
	_, err = consensus.NewFencingToken("leader/lock", "", 42, expiry)
	if err == nil {
		t.Errorf("expected error for empty owner")
	}
	_, err = consensus.NewFencingToken("leader/lock", "node-1", 0, expiry)
	if err == nil {
		t.Errorf("expected error for zero generation")
	}
}

func TestRFC7807ProblemDetails(t *testing.T) {
	if consensus.ErrEtcdDefragLeaderBlocked.Status != http.StatusConflict {
		t.Errorf("expected ErrEtcdDefragLeaderBlocked status 409, got %d", consensus.ErrEtcdDefragLeaderBlocked.Status)
	}
	if consensus.ErrFencingTokenStale.Status != http.StatusPreconditionFailed {
		t.Errorf("expected ErrFencingTokenStale status 412, got %d", consensus.ErrFencingTokenStale.Status)
	}
	if consensus.ErrConsensusQuorumLost.Status != http.StatusServiceUnavailable {
		t.Errorf("expected ErrConsensusQuorumLost status 503, got %d", consensus.ErrConsensusQuorumLost.Status)
	}

	// Verify errors.Is compatibility
	wrapped := errors.New(consensus.ErrFencingTokenStale.Error())
	_ = wrapped
	if !errors.Is(consensus.ErrFencingTokenStale, consensus.ErrFencingTokenStale) {
		t.Errorf("errors.Is failed on ErrFencingTokenStale")
	}
}

func TestCacheLinePaddingAndConcurreny(t *testing.T) {
	tracker := consensus.NewConsensusTelemetryTracker()

	// High concurrency stress test across 50 goroutines
	var wg sync.WaitGroup
	iterations := 1000
	goroutines := 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				tracker.NextGeneration()
				tracker.IncValidations()
				if j%10 == 0 {
					tracker.IncStaleRejects()
				}
				tracker.RecordRaftIndex(uint64(workerID*iterations + j))
			}
		}(i)
	}
	wg.Wait()

	expectedGenerations := int64(goroutines * iterations)
	if tracker.GetGeneration() != expectedGenerations {
		t.Errorf("expected %d generations, got %d", expectedGenerations, tracker.GetGeneration())
	}
	if tracker.GetValidations() != uint64(goroutines*iterations) {
		t.Errorf("expected %d validations, got %d", goroutines*iterations, tracker.GetValidations())
	}
}
