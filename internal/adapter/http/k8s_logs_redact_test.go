package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

// blockingPipeReader blocks Read until ctx is canceled or closed
type blockingReader struct {
	closeChan chan struct{}
}

func newBlockingReader() *blockingReader {
	return &blockingReader{closeChan: make(chan struct{})}
}

func (b *blockingReader) Read(p []byte) (n int, err error) {
	<-b.closeChan
	return 0, io.EOF
}

func (b *blockingReader) Close() error {
	select {
	case <-b.closeChan:
	default:
		close(b.closeChan)
	}
	return nil
}

func TestK8sLogsHandler_StreamSinglePodLogs_RedactionAndCancellation(t *testing.T) {
	h := NewK8sLogsHandler(nil, nil)

	t.Run("redacts secrets in stream", func(t *testing.T) {
		rawLog := "connecting to postgres://admin:superSecret123@db.prod.internal:5432/main\n"
		stream := io.NopCloser(strings.NewReader(rawLog))

		req := httptest.NewRequest(http.MethodGet, "/k8s/local/pods/my-pod/logs?follow=true", nil)
		rec := httptest.NewRecorder()

		h.streamSinglePodLogs(rec, req, stream)

		body := rec.Body.String()
		if strings.Contains(body, "superSecret123") {
			t.Fatalf("sensitive password leaked in streamed log: %s", body)
		}
		if !strings.Contains(body, "[REDACTED_SECRET]") {
			t.Fatalf("expected redacted placeholder in streamed log, got: %s", body)
		}
	})

	t.Run("context cancellation unblocks scanner without leak", func(t *testing.T) {
		reader := newBlockingReader()
		ctx, cancel := context.WithCancel(context.Background())

		req := httptest.NewRequest(http.MethodGet, "/k8s/local/pods/my-pod/logs?follow=true", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		done := make(chan struct{})
		go func() {
			h.streamSinglePodLogs(rec, req, reader)
			close(done)
		}()

		// Give it a moment to block inside scanner.Scan()
		time.Sleep(30 * time.Millisecond)

		// Cancel context; scanner should be unblocked immediately by closing reader
		cancel()

		select {
		case <-done:
			// Passed
		case <-time.After(1 * time.Second):
			t.Fatal("streamSinglePodLogs failed to exit on context cancellation within 1s")
		}
	})
}

func TestLogStreamHandler_Redaction(t *testing.T) {
	aggregator := logging.NewLogAggregator(100)
	h := NewLogStreamHandler(aggregator)

	masked := h.maskLogMessage("{\"password\": \"my_ultra_secret_pw\"}")
	if strings.Contains(masked, "my_ultra_secret_pw") {
		t.Fatalf("expected secret to be redacted, got: %s", masked)
	}
	if !strings.Contains(masked, "[REDACTED_SECRET]") {
		t.Fatalf("expected redacted secret placeholder, got: %s", masked)
	}
}
