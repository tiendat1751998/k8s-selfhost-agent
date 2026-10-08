package syncutil_test

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/syncutil"
)

func TestKeyedMutex_ConcurrentSameKey(t *testing.T) {
	km := syncutil.NewKeyedMutex()
	key := "cluster-a/default/my-release"

	const total = 10
	var successCount int64
	var inProgressCount int64

	startBarrier := make(chan struct{})
	holdRelease := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier
			if km.TryLock(key) {
				atomic.AddInt64(&successCount, 1)
				<-holdRelease
				km.Unlock(key)
			} else {
				// Contention produces ErrOperationInProgress
				err := fmt.Errorf("%w: release %s is busy", syncutil.ErrOperationInProgress, key)
				if errors.Is(err, syncutil.ErrOperationInProgress) {
					atomic.AddInt64(&inProgressCount, 1)
				}
			}
		}()
	}

	// Release all goroutines simultaneously
	close(startBarrier)
	// Give them time to attempt TryLock while 1 holds the lock
	time.Sleep(50 * time.Millisecond)
	// Release the lock holder
	close(holdRelease)
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful lock, got %d", successCount)
	}
	if inProgressCount != 9 {
		t.Fatalf("expected exactly 9 in-progress errors, got %d", inProgressCount)
	}

	// After release, key should be lockable again
	if !km.TryLock(key) {
		t.Fatalf("expected key to be lockable after release")
	}
	km.Unlock(key)
}

func TestKeyedMutex_DifferentKeysParallel(t *testing.T) {
	km := syncutil.NewKeyedMutex()
	const count = 20

	var wg sync.WaitGroup
	var acquiredCount int64

	for i := 0; i < count; i++ {
		wg.Add(1)
		key := fmt.Sprintf("cluster/ns/release-%d", i)
		go func(k string) {
			defer wg.Done()
			if km.TryLock(k) {
				atomic.AddInt64(&acquiredCount, 1)
				time.Sleep(5 * time.Millisecond)
				km.Unlock(k)
			}
		}(key)
	}

	wg.Wait()

	if acquiredCount != int64(count) {
		t.Fatalf("expected all %d different keys to acquire lock in parallel, got %d", count, acquiredCount)
	}
}

func BenchmarkKeyedMutex_TryLock(b *testing.B) {
	km := syncutil.NewKeyedMutex()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var id int
		for pb.Next() {
			id++
			key := fmt.Sprintf("bench-key-%d", id%500)
			if km.TryLock(key) {
				km.Unlock(key)
			}
		}
	})
}
