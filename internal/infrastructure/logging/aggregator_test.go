package logging_test

import (
	"encoding/json"
	"fmt"
	"strings"
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

func TestLogAggregator_NodeFiltering(t *testing.T) {
	agg := logging.NewLogAggregator(100)

	agg.Ingest(logging.LogEntry{
		Namespace: "system",
		Pod:       "agent-1",
		Node:      "worker1",
		Service:   "k8s-agent",
		Message:   "worker1 explicit node log",
	})
	agg.Ingest(logging.LogEntry{
		Namespace: "system",
		Pod:       "worker1-daemonset-9x",
		Node:      "", // empty node, pod contains worker1
		Service:   "k8s-agent",
		Message:   "worker1 fallback pod log",
	})
	agg.Ingest(logging.LogEntry{
		Namespace: "system",
		Pod:       "agent-2",
		Node:      "worker2",
		Service:   "k8s-agent",
		Message:   "worker2 explicit node log",
	})

	// Subscribe filtering by Node: "worker1" (case-insensitive)
	sub, history := agg.Subscribe("sub-node-worker1", logging.LogFilter{Node: "WORKER1"}, 10)
	defer agg.Unsubscribe("sub-node-worker1")

	if len(history) != 2 {
		t.Fatalf("expected 2 historical logs for node worker1, got %d", len(history))
	}
	if history[0].Message != "worker1 explicit node log" || history[1].Message != "worker1 fallback pod log" {
		t.Errorf("unexpected historical entries for worker1: %+v", history)
	}

	// Live ingest for worker1
	go func() {
		time.Sleep(20 * time.Millisecond)
		agg.Ingest(logging.LogEntry{
			Namespace: "system",
			Pod:       "agent-1",
			Node:      "worker1",
			Service:   "k8s-agent",
			Message:   "worker1 live log",
		})
	}()

	select {
	case entry := <-sub.Ch:
		if entry.Message != "worker1 live log" {
			t.Errorf("unexpected live log: %s", entry.Message)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for worker1 live log")
	}

	// Ingest for worker2 - subscriber should not receive it
	agg.Ingest(logging.LogEntry{
		Namespace: "system",
		Pod:       "agent-2",
		Node:      "worker2",
		Message:   "worker2 live log",
	})
	select {
	case entry := <-sub.Ch:
		t.Fatalf("unexpected entry for worker1 sub: %+v", entry)
	default:
		// OK
	}
}

func TestLogAggregator_ServiceFiltering(t *testing.T) {
	agg := logging.NewLogAggregator(100)

	agg.Ingest(logging.LogEntry{
		Namespace: "system",
		Pod:       "db-0",
		Node:      "masterdb",
		Service:   "postgres",
		Message:   "postgres explicit service log",
	})
	agg.Ingest(logging.LogEntry{
		Namespace: "system",
		Pod:       "ingress-abc",
		Node:      "k8smater",
		Container: "traefik", // empty service, container matches
		Message:   "traefik container log",
	})
	agg.Ingest(logging.LogEntry{
		Namespace: "system",
		Pod:       "nats-cluster-node1", // empty service/container, pod contains nats
		Node:      "worker1",
		Message:   "nats pod log",
	})

	// 1. Explicit Service
	_, histPG := agg.Subscribe("sub-pg", logging.LogFilter{Service: "POSTGRES"}, 10)
	defer agg.Unsubscribe("sub-pg")
	if len(histPG) != 1 || histPG[0].Message != "postgres explicit service log" {
		t.Fatalf("expected 1 log for service postgres, got %+v", histPG)
	}

	// 2. Container Fallback
	_, histTraefik := agg.Subscribe("sub-traefik", logging.LogFilter{Service: "traefik"}, 10)
	defer agg.Unsubscribe("sub-traefik")
	if len(histTraefik) != 1 || histTraefik[0].Message != "traefik container log" {
		t.Fatalf("expected 1 log for service traefik, got %+v", histTraefik)
	}

	// 3. Pod Fallback
	_, histNATS := agg.Subscribe("sub-nats", logging.LogFilter{Service: "nats"}, 10)
	defer agg.Unsubscribe("sub-nats")
	if len(histNATS) != 1 || histNATS[0].Message != "nats pod log" {
		t.Fatalf("expected 1 log for service nats, got %+v", histNATS)
	}
}

func TestLogAggregator_ContainerAndKeywordAndLevelFiltering(t *testing.T) {
	agg := logging.NewLogAggregator(100)

	agg.Ingest(logging.LogEntry{
		Namespace: "prod",
		Pod:       "order-service-1",
		Container: "app",
		Level:     "WARN",
		Message:   "deadlock detected on table orders",
	})
	agg.Ingest(logging.LogEntry{
		Namespace: "prod",
		Pod:       "order-service-1",
		Container: "sidecar",
		Level:     "WARN",
		Message:   "deadlock detected proxy",
	})
	agg.Ingest(logging.LogEntry{
		Namespace: "prod",
		Pod:       "order-service-1",
		Container: "app",
		Level:     "INFO",
		Message:   "deadlock resolved ok",
	})

	filter := logging.LogFilter{
		Container: "app",
		Level:     "warn", // case-insensitive check
		Keyword:   "deadlock",
	}

	_, history := agg.Subscribe("sub-compound", filter, 10)
	defer agg.Unsubscribe("sub-compound")

	if len(history) != 1 {
		t.Fatalf("expected 1 matching entry, got %d", len(history))
	}
	if history[0].Container != "app" || history[0].Level != "WARN" {
		t.Errorf("unexpected entry returned: %+v", history[0])
	}
}

func TestLogEntry_MarshalUnmarshalJSON(t *testing.T) {
	fixedTime := time.Date(2026, 9, 4, 13, 30, 0, 0, time.UTC)
	entry := logging.LogEntry{
		Timestamp: fixedTime,
		Namespace: "core",
		Pod:       "api-gw",
		Container: "proxy",
		Node:      "k8smater",
		Service:   "traefik",
		Stream:    "stdout",
		Level:     "INFO",
		Message:   "route loaded",
	}

	bytes, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(bytes)
	// Must contain backwards-compatible fields
	requiredKeys := []string{
		`"node":"k8smater"`,
		`"service":"traefik"`,
		`"host":"k8smater"`,
		`"app":"traefik"`,
		`"container":"proxy"`,
		`"pod":"api-gw"`,
		`"namespace":"core"`,
		`"level":"INFO"`,
		`"msg":"route loaded"`,
		`"message":"route loaded"`,
		`"time":`,
		`"timestamp":`,
	}
	for _, key := range requiredKeys {
		if !strings.Contains(jsonStr, key) {
			t.Errorf("marshaled json missing required field %q: %s", key, jsonStr)
		}
	}

	// Test Unmarshaling legacy fields
	legacyJSON := `{"host":"masterdb","app":"postgres","msg":"checkpoint complete","level":"INFO","time":"2026-09-04T13:30:00Z"}`
	var unmarshaled logging.LogEntry
	if err := json.Unmarshal([]byte(legacyJSON), &unmarshaled); err != nil {
		t.Fatalf("unmarshal legacy failed: %v", err)
	}

	if unmarshaled.Node != "masterdb" {
		t.Errorf("expected Node 'masterdb', got '%s'", unmarshaled.Node)
	}
	if unmarshaled.Service != "postgres" {
		t.Errorf("expected Service 'postgres', got '%s'", unmarshaled.Service)
	}
	if unmarshaled.Message != "checkpoint complete" {
		t.Errorf("expected Message 'checkpoint complete', got '%s'", unmarshaled.Message)
	}
	if unmarshaled.Timestamp.IsZero() {
		t.Errorf("expected non-zero Timestamp parsed from time")
	}

	// Test Unmarshaling canonical fields
	canonicalJSON := `{"node":"worker1","service":"nats","message":"ready","level":"INFO"}`
	var unmarshaledCanonical logging.LogEntry
	if err := json.Unmarshal([]byte(canonicalJSON), &unmarshaledCanonical); err != nil {
		t.Fatalf("unmarshal canonical failed: %v", err)
	}
	if unmarshaledCanonical.Node != "worker1" || unmarshaledCanonical.Service != "nats" || unmarshaledCanonical.Message != "ready" {
		t.Errorf("unexpected canonical unmarshal: %+v", unmarshaledCanonical)
	}
}

