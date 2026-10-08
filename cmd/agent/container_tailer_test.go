package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

type mockDockerClientForTailer struct {
	mu              sync.Mutex
	containers      []types.Container
	logStreams      map[string]string
	callCounts      map[string]int
	lastLogsOptions map[string]container.LogsOptions
	streamGenerators map[string]func(call int, opts container.LogsOptions) string
	errGenerators    map[string]func(call int) error
	binaryLogStreams map[string][]byte
}

func (m *mockDockerClientForTailer) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.containers, nil
}

func (m *mockDockerClientForTailer) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.callCounts == nil {
		m.callCounts = make(map[string]int)
	}
	if m.lastLogsOptions == nil {
		m.lastLogsOptions = make(map[string]container.LogsOptions)
	}
	m.callCounts[containerID]++
	m.lastLogsOptions[containerID] = options

	if m.errGenerators != nil && m.errGenerators[containerID] != nil {
		if err := m.errGenerators[containerID](m.callCounts[containerID]); err != nil {
			return nil, err
		}
	}

	if m.streamGenerators != nil && m.streamGenerators[containerID] != nil {
		content := m.streamGenerators[containerID](m.callCounts[containerID], options)
		return io.NopCloser(strings.NewReader(content)), nil
	}
	if m.binaryLogStreams != nil && m.binaryLogStreams[containerID] != nil {
		return io.NopCloser(bytes.NewReader(m.binaryLogStreams[containerID])), nil
	}

	content := m.logStreams[containerID]
	return io.NopCloser(strings.NewReader(content)), nil
}

func (m *mockDockerClientForTailer) Ping(ctx context.Context) (types.Ping, error) {
	return types.Ping{}, nil
}

func (m *mockDockerClientForTailer) Close() error {
	return nil
}

func TestContainerTailer_IngestsAndStopsCleanly(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	mockCli := &mockDockerClientForTailer{
		containers: []types.Container{
			{
				ID:    "c123456789012",
				Names: []string{"/web-api"},
				Labels: map[string]string{
					"com.docker.compose.service": "web-api",
				},
			},
		},
		logStreams: map[string]string{
			"c123456789012": "2026-09-14T12:00:00Z [INFO] Server started\n2026-09-14T12:00:01Z [ERROR] DB connection failed\n",
		},
	}

	tailer := NewContainerTailer(mockCli, writer, slog.Default())
	tailer.pollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		tailer.Start(ctx)
		close(done)
	}()

	// Wait for tailer to process stream
	time.Sleep(200 * time.Millisecond)

	// Flush writer to disk
	if err := writer.Flush(); err != nil {
		t.Fatalf("failed to flush writer: %v", err)
	}

	// Read back entries via Reader
	reader, err := logengine.NewReader(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log reader: %v", err)
	}
	defer reader.Close()

	results, err := reader.Search(context.Background(), logengine.QueryParams{
		Service: "web-api",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 log entries ingested, got %d", len(results))
	}

	if results[0].Service != "web-api" {
		t.Errorf("expected service 'web-api', got '%s'", results[0].Service)
	}

	// Cancel context to test clean termination
	cancel()

	select {
	case <-done:
		// Clean termination
	case <-time.After(2 * time.Second):
		t.Fatalf("ContainerTailer did not terminate cleanly on context cancellation")
	}
}

func TestContainerTailer_CatchesUpInitialLogsOnDiscovery(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	mockCli := &mockDockerClientForTailer{
		containers: []types.Container{
			{
				ID:     "c999888777666",
				Names:  []string{"/initial-svc"},
				Labels: map[string]string{"com.docker.compose.service": "initial-svc"},
			},
		},
		logStreams: map[string]string{
			"c999888777666": "2026-09-14T12:00:00Z [INFO] init log\n",
		},
	}

	tailer := NewContainerTailer(mockCli, writer, slog.Default())
	tailer.pollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tailer.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	tailer.Stop()

	mockCli.mu.Lock()
	opts := mockCli.lastLogsOptions["c999888777666"]
	mockCli.mu.Unlock()

	// On agent restart or initial container discovery (lastTimestamp is zero),
	// tail must default to "100" to prevent massive log replay floods.
	if opts.Tail != "100" {
		t.Fatalf("expected Tail option to be '100' on initial discovery, got %q", opts.Tail)
	}
}

func TestContainerTailer_TailFallbackOnZeroTimestamp(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	mockCli := &mockDockerClientForTailer{
		containers: []types.Container{
			{
				ID:     "c_restart_check",
				Names:  []string{"/restart-svc"},
				Labels: map[string]string{"com.docker.compose.service": "restart-svc"},
			},
		},
		logStreams: map[string]string{
			"c_restart_check": "2026-09-14T12:00:00Z [INFO] restart log\n",
		},
	}

	tailer := NewContainerTailer(mockCli, writer, slog.Default())
	tailer.tailLines = "" // simulate empty tailLines to verify fallback to "100"
	tailer.pollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tailer.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	tailer.Stop()

	mockCli.mu.Lock()
	opts := mockCli.lastLogsOptions["c_restart_check"]
	mockCli.mu.Unlock()

	if opts.Tail != "100" {
		t.Fatalf("expected Tail option fallback to '100' when lastTimestamp is zero and tailLines is empty, got %q", opts.Tail)
	}
}

func TestContainerTailer_RecoversOnEOFWhenContainerRotates(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	const containerID = "c_rotate_12345"
	mockCli := &mockDockerClientForTailer{
		containers: []types.Container{
			{
				ID:     containerID,
				Names:  []string{"/rotate-svc"},
				Labels: map[string]string{"com.docker.compose.service": "rotate-svc"},
			},
		},
		streamGenerators: map[string]func(call int, opts container.LogsOptions) string{
			containerID: func(call int, opts container.LogsOptions) string {
				if call == 1 {
					// First stream: reaches EOF (e.g. rotation event)
					return "2026-09-14T12:00:00Z [INFO] line before rotation\n"
				}
				// Subsequent stream after EOF/rotation: resumes with new logs
				return "2026-09-14T12:00:01Z [INFO] line after rotation\n"
			},
		},
	}

	tailer := NewContainerTailer(mockCli, writer, slog.Default())
	tailer.pollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tailer.Start(ctx)

	// Wait enough for first stream to hit EOF and reconnect/recover
	time.Sleep(400 * time.Millisecond)
	tailer.Stop()

	mockCli.mu.Lock()
	callCount := mockCli.callCounts[containerID]
	mockCli.mu.Unlock()

	if callCount < 2 {
		t.Fatalf("expected tailer to reconnect on EOF after log rotation (callCount >= 2), got %d calls", callCount)
	}

	// Verify both logs were ingested
	_ = writer.Flush()
	reader, err := logengine.NewReader(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log reader: %v", err)
	}
	defer reader.Close()

	results, err := reader.Search(context.Background(), logengine.QueryParams{
		Service: "rotate-svc",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 entries ingested across rotations, got %d", len(results))
	}
}

func createStdCopyFrame(stream byte, payload string) []byte {
	b := []byte(payload)
	hdr := make([]byte, 8+len(b))
	hdr[0] = stream
	binary.BigEndian.PutUint32(hdr[4:8], uint32(len(b)))
	copy(hdr[8:], b)
	return hdr
}

func TestContainerTailer_MultiplexedStdCopy_ConcurrentNoDataRace(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	const containerID = "c_stdcopy_race"
	var buf bytes.Buffer
	for i := 0; i < 50; i++ {
		stdoutLine := fmt.Sprintf("2026-09-14T12:00:%02d.000Z [INFO] stdout line %d\n", i%60, i)
		stderrLine := fmt.Sprintf("2026-09-14T12:00:%02d.000Z [WARN] stderr line %d\n", i%60, i)
		buf.Write(createStdCopyFrame(1, stdoutLine))
		buf.Write(createStdCopyFrame(2, stderrLine))
	}

	mockCli := &mockDockerClientForTailer{
		containers: []types.Container{
			{
				ID:     containerID,
				Names:  []string{"/race-svc"},
				Labels: map[string]string{"com.docker.compose.service": "race-svc"},
			},
		},
		binaryLogStreams: map[string][]byte{
			containerID: buf.Bytes(),
		},
	}

	tailer := NewContainerTailer(mockCli, writer, slog.Default())
	tailer.pollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tailer.Start(ctx)

	time.Sleep(200 * time.Millisecond)
	tailer.Stop()
}

func TestContainerTailer_SeenAtLastTS_CappedOnTimestampFlood(t *testing.T) {
	tailer := NewContainerTailer(nil, nil, slog.Default())
	var lastTS time.Time
	seenAtLastTS := make(map[string]bool)

	var buf bytes.Buffer
	const floodCount = 25000
	for i := 0; i < floodCount; i++ {
		buf.WriteString(fmt.Sprintf("2026-09-14T12:00:00.000000000Z message line %d\n", i))
	}

	reader := io.NopCloser(&buf)
	tailer.readContainerLogStream(context.Background(), reader, "flood-svc", &lastTS, seenAtLastTS)

	if len(seenAtLastTS) > 10000 {
		t.Fatalf("expected seenAtLastTS to be capped <= 10000 on identical-timestamp flood, got %d", len(seenAtLastTS))
	}
	if len(seenAtLastTS) == 0 {
		t.Fatalf("expected seenAtLastTS to retain recent entries, got 0")
	}
}

func TestContainerTailer_BackoffDelaysToCapAndResetAfterSuccess(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	const containerID = "c_backoff_test"
	mockCli := &mockDockerClientForTailer{
		containers: []types.Container{
			{
				ID:    containerID,
				Names: []string{"/backoff-svc"},
				Labels: map[string]string{
					"com.docker.compose.service": "backoff-svc",
				},
			},
		},
		errGenerators: map[string]func(call int) error{
			containerID: func(call int) error {
				if call <= 10 || call >= 12 {
					return fmt.Errorf("simulated docker error on call %d", call)
				}
				return nil
			},
		},
		streamGenerators: map[string]func(call int, opts container.LogsOptions) string{
			containerID: func(call int, opts container.LogsOptions) string {
				if call == 11 {
					return "2026-09-14T12:00:00Z [INFO] stream success\n"
				}
				return ""
			},
		},
	}

	tailer := NewContainerTailer(mockCli, writer, slog.Default())
	tailer.pollInterval = 50 * time.Millisecond
	tailer.eofRetryInterval = 50 * time.Millisecond

	// Inject deterministic exponential backoff (no random jitter for predictable progression)
	tailer.newBackOffFn = func() backoff.BackOff {
		bo := backoff.NewExponentialBackOff()
		bo.InitialInterval = 200 * time.Millisecond
		bo.Multiplier = 1.5
		bo.MaxInterval = 5 * time.Second
		bo.RandomizationFactor = 0
		return bo
	}

	var recordedDelays []time.Duration
	var delayMu sync.Mutex
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tailer.sleepFn = func(sCtx context.Context, d time.Duration) error {
		delayMu.Lock()
		recordedDelays = append(recordedDelays, d)
		count := len(recordedDelays)
		delayMu.Unlock()

		if count >= 13 {
			cancel()
			return context.Canceled
		}
		return nil
	}

	tailer.Start(ctx)

	delayMu.Lock()
	delays := append([]time.Duration(nil), recordedDelays...)
	delayMu.Unlock()

	if len(delays) < 12 {
		t.Fatalf("expected at least 12 recorded delays, got %d: %+v", len(delays), delays)
	}

	// 1. Initial delay should be 200ms
	if delays[0] != 200*time.Millisecond {
		t.Errorf("expected first retry delay 200ms, got %v", delays[0])
	}

	// 2. Delays should monotonically increase up to 5s cap
	for i := 1; i < 9; i++ {
		if delays[i] <= delays[i-1] && delays[i] < 5*time.Second {
			t.Errorf("expected delay[%d] (%v) > delay[%d] (%v)", i, delays[i], i-1, delays[i-1])
		}
		if delays[i] > 5*time.Second {
			t.Errorf("expected delay[%d] <= 5s cap, got %v", i, delays[i])
		}
	}

	// 3. Delays at attempts 9 and 10 should be capped at 5s
	if delays[8] != 5*time.Second {
		t.Errorf("expected delay[8] to be capped at 5s, got %v", delays[8])
	}
	if delays[9] != 5*time.Second {
		t.Errorf("expected delay[9] to be capped at 5s, got %v", delays[9])
	}

	// 4. After call 11 succeeds and hits EOF (sleeps eofRetryInterval 50ms),
	// call 12 fails -> NextBackOff should be reset back to 200ms!
	if delays[10] != 50*time.Millisecond {
		t.Errorf("expected delay[10] to be EOF retry interval 50ms, got %v", delays[10])
	}
	if delays[11] != 200*time.Millisecond {
		t.Errorf("expected delay[11] after reset to be initial interval 200ms, got %v", delays[11])
	}
	if delays[12] != 300*time.Millisecond {
		t.Errorf("expected delay[12] after reset to increase to 300ms, got %v", delays[12])
	}
}

type mockBackOffSpy struct {
	nextCalls  int
	resetCalls int
}

func (m *mockBackOffSpy) NextBackOff() time.Duration {
	m.nextCalls++
	return 100 * time.Millisecond
}

func (m *mockBackOffSpy) Reset() {
	m.resetCalls++
}

func TestContainerTailer_BackoffResetInjectedSpy(t *testing.T) {
	tempDir := t.TempDir()
	dict := logengine.NewLabelDictionary()
	writer, err := logengine.NewWriter(tempDir, dict)
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	const containerID = "c_spy_test"
	mockCli := &mockDockerClientForTailer{
		containers: []types.Container{
			{
				ID:    containerID,
				Names: []string{"/spy-svc"},
				Labels: map[string]string{
					"com.docker.compose.service": "spy-svc",
				},
			},
		},
		errGenerators: map[string]func(call int) error{
			containerID: func(call int) error {
				if call == 1 {
					return fmt.Errorf("initial error")
				}
				return nil
			},
		},
		streamGenerators: map[string]func(call int, opts container.LogsOptions) string{
			containerID: func(call int, opts container.LogsOptions) string {
				return "2026-09-14T12:00:00Z [INFO] spy log\n"
			},
		},
	}

	tailer := NewContainerTailer(mockCli, writer, slog.Default())
	tailer.pollInterval = 50 * time.Millisecond

	spy := &mockBackOffSpy{}
	tailer.newBackOffFn = func() backoff.BackOff {
		return spy
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tailer.sleepFn = func(sCtx context.Context, d time.Duration) error {
		if spy.resetCalls >= 1 {
			cancel()
			return context.Canceled
		}
		return nil
	}

	tailer.Start(ctx)

	if spy.nextCalls == 0 {
		t.Errorf("expected NextBackOff to be called on error")
	}
	if spy.resetCalls == 0 {
		t.Errorf("expected Reset to be called upon successful stream")
	}
}

