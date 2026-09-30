package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

func TestJournalTailer_UnavailableGracefulExit(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	tailer := NewJournalTailer(writer, slog.Default())
	// Override check to false
	tailer.availableCheck = func() bool { return false }

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		tailer.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Exited gracefully
	case <-time.After(1 * time.Second):
		t.Fatalf("JournalTailer did not exit gracefully when journalctl is unavailable")
	}
}

func TestJournalTailer_IngestsJSONEntries(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	tailer := NewJournalTailer(writer, slog.Default())
	tailer.availableCheck = func() bool { return true }

	// Simulate journalctl output using echo command or mock stream
	fakeOutput := `{"MESSAGE":"Service started successfully","__REALTIME_TIMESTAMP":"1726315200000000","_SYSTEMD_UNIT":"kubelet.service","PRIORITY":"6"}
{"MESSAGE":"Connection reset by peer","__REALTIME_TIMESTAMP":"1726315201000000","_SYSTEMD_UNIT":"docker.service","PRIORITY":"3"}
`
	tailer.streamReader = func(ctx context.Context) (io.ReadCloser, error) {
		return &stringCloser{Reader: strings.NewReader(fakeOutput)}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		tailer.Start(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("JournalTailer did not exit after cancel")
	}

	if err := writer.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	reader, err := logengine.NewReader(tempDir, dict)
	if err != nil {
		t.Fatalf("reader failed: %v", err)
	}
	defer reader.Close()

	results, err := reader.Search(context.Background(), logengine.QueryParams{Limit: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 journal entries, got %d", len(results))
	}
	if results[0].Service != "kubelet.service" {
		t.Errorf("expected service 'kubelet.service', got '%s'", results[0].Service)
	}
	if results[1].Level != "error" {
		t.Errorf("expected priority 3 to map to 'error', got '%s'", results[1].Level)
	}
}

type stringCloser struct {
	*strings.Reader
}

func (s *stringCloser) Close() error {
	return nil
}
