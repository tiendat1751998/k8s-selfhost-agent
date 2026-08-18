package logging_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

func TestRingBuffer_PushAndGetAll(t *testing.T) {
	rb := logging.NewRingBuffer(3)

	for i := 1; i <= 5; i++ {
		rb.Push(logging.LogEntry{
			Message: fmt.Sprintf("log message %d", i),
		})
	}

	all := rb.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 entries in ring buffer, got %d", len(all))
	}

	// Should contain last 3: 3, 4, 5
	if all[0].Message != "log message 3" || all[1].Message != "log message 4" || all[2].Message != "log message 5" {
		t.Errorf("unexpected ring buffer content: %+v", all)
	}
}

func TestLogAggregator_PubSubAndFiltering(t *testing.T) {
	agg := logging.NewLogAggregator(100)

	filter := logging.LogFilter{
		Namespace: "prod",
		Pod:       "payment-service-1",
		Level:     "ERROR",
	}

	sub, historical := agg.Subscribe("sub-1", filter, 10)
	defer agg.Unsubscribe("sub-1")

	if len(historical) != 0 {
		t.Errorf("expected 0 historical logs initially, got %d", len(historical))
	}

	// Ingest matching log
	agg.Ingest(logging.LogEntry{
		Namespace: "prod",
		Pod:       "payment-service-1",
		Container: "app",
		Level:     "ERROR",
		Message:   "database connection timeout on pool acquire",
	})

	// Ingest non-matching log (INFO level)
	agg.Ingest(logging.LogEntry{
		Namespace: "prod",
		Pod:       "payment-service-1",
		Container: "app",
		Level:     "INFO",
		Message:   "heartbeat ping ok",
	})

	// Ingest non-matching log (Different Pod)
	agg.Ingest(logging.LogEntry{
		Namespace: "prod",
		Pod:       "auth-service-2",
		Container: "app",
		Level:     "ERROR",
		Message:   "invalid jwt token",
	})

	select {
	case entry := <-sub.Ch:
		if entry.Level != "ERROR" || entry.Pod != "payment-service-1" {
			t.Errorf("received unexpected log entry: %+v", entry)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for matching log entry")
	}

	// Verify no second item in channel
	select {
	case entry := <-sub.Ch:
		t.Fatalf("unexpected extra entry in channel: %+v", entry)
	default:
		// OK
	}
}
