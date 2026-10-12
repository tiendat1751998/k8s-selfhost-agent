package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

func TestParseCRILogLine(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		wantOK        bool
		wantStream    string
		wantFlag      string
		wantContent   string
		wantTimestamp string
	}{
		{
			name:          "standard stdout final line",
			line:          "2026-10-12T10:15:30.123456789Z stdout F Starting HTTP server on port 8080",
			wantOK:        true,
			wantStream:    "stdout",
			wantFlag:      "F",
			wantContent:   "Starting HTTP server on port 8080",
			wantTimestamp: "2026-10-12T10:15:30.123456789Z",
		},
		{
			name:          "standard stderr final line",
			line:          "2026-10-12T10:15:31.000000000Z stderr F Connection failed to redis-master",
			wantOK:        true,
			wantStream:    "stderr",
			wantFlag:      "F",
			wantContent:   "Connection failed to redis-master",
			wantTimestamp: "2026-10-12T10:15:31.000000000Z",
		},
		{
			name:          "partial log line chunk",
			line:          "2026-10-12T10:15:32.500000000Z stdout P trace header part 1 ",
			wantOK:        true,
			wantStream:    "stdout",
			wantFlag:      "P",
			wantContent:   "trace header part 1 ",
			wantTimestamp: "2026-10-12T10:15:32.500000000Z",
		},
		{
			name:          "empty content line",
			line:          "2026-10-12T10:15:33.000000000Z stdout F ",
			wantOK:        true,
			wantStream:    "stdout",
			wantFlag:      "F",
			wantContent:   "",
			wantTimestamp: "2026-10-12T10:15:33.000000000Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok := parseCRILine(tt.line)
			if !tt.wantOK {
				assert.False(t, ok)
				return
			}
			require.True(t, ok)
			assert.Equal(t, tt.wantStream, parsed.Stream)
			assert.Equal(t, tt.wantFlag, parsed.Flag)
			assert.Equal(t, tt.wantContent, parsed.Content)
			expectedTime, _ := time.Parse(time.RFC3339Nano, tt.wantTimestamp)
			assert.True(t, expectedTime.Equal(parsed.Timestamp))
		})
	}
}

func TestExtractPodMeta(t *testing.T) {
	// Standard path format: /var/log/pods/<namespace>_<pod-name>_<pod-uid>/<container-name>/0.log
	path := "/var/log/pods/prod-apps_frontend-web-7f89d-4x8k2_a1b2c3d4-e5f6-7890/nginx/0.log"
	meta := extractPodMetaFromPath(path)

	assert.Equal(t, "prod-apps", meta.Namespace)
	assert.Equal(t, "frontend-web-7f89d-4x8k2", meta.PodName)
	assert.Equal(t, "nginx", meta.ContainerName)
	assert.Equal(t, "frontend-web-7f89d-4x8k2", meta.ServiceName)
}

type recordingWriter struct {
	mu      sync.Mutex
	entries []logengine.Entry
}

func (r *recordingWriter) Write(e logengine.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
	return nil
}

func (r *recordingWriter) getEntries() []logengine.Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := make([]logengine.Entry, len(r.entries))
	copy(copied, r.entries)
	return copied
}

func TestPodTailer_PartialLineAssembly(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cri-pod-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create CRI pod log directory structure
	podDir := filepath.Join(tmpDir, "default_payment-svc-99_uuid123", "app")
	require.NoError(t, os.MkdirAll(podDir, 0755))
	logFile := filepath.Join(podDir, "0.log")

	// Write 2 partial chunks (P) followed by final (F)
	content := "2026-10-12T12:00:00.100000000Z stdout P [STACK TRACE] Line 1: NullPointerException: \n" +
		"2026-10-12T12:00:00.100000000Z stdout P \tat com.app.Service.process(Service.java:42)\n" +
		"2026-10-12T12:00:00.100000000Z stdout F \tat com.app.Main.main(Main.java:10)\n" +
		"2026-10-12T12:00:01.000000000Z stdout F Next log line\n"

	require.NoError(t, os.WriteFile(logFile, []byte(content), 0644))

	recWriter := &recordingWriter{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tailer := NewPodTailer(tmpDir, nil, logger)
	tailer.entryWriter = recWriter.Write
	tailer.pollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tailer.pollOnce(ctx)

	entries := recWriter.getEntries()
	require.Len(t, entries, 2, "must assemble 3 chunks into 1 entry + 1 normal entry")

	expectedStackTrace := "[STACK TRACE] Line 1: NullPointerException: " +
		"\tat com.app.Service.process(Service.java:42)" +
		"\tat com.app.Main.main(Main.java:10)"
	assert.Equal(t, expectedStackTrace, entries[0].Message)
	assert.Equal(t, "Next log line", entries[1].Message)
}

func TestPodTailer_LogRotation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cri-rotation-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	podDir := filepath.Join(tmpDir, "prod_auth-svc-11_uuid456", "auth")
	require.NoError(t, os.MkdirAll(podDir, 0755))
	logFile := filepath.Join(podDir, "0.log")

	// 1. Initial write
	line1 := "2026-10-12T14:00:00.000000000Z stdout F Log line 1 before rotation\n"
	require.NoError(t, os.WriteFile(logFile, []byte(line1), 0644))

	recWriter := &recordingWriter{}
	tailer := NewPodTailer(tmpDir, nil, slog.Default())
	tailer.entryWriter = recWriter.Write
	tailer.pollInterval = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tailer.pollOnce(ctx)
	require.Len(t, recWriter.getEntries(), 1)
	assert.Equal(t, "Log line 1 before rotation", recWriter.getEntries()[0].Message)

	// 2. Rotate file: 0.log -> 0.log.1, create new 0.log
	rotatedFile := filepath.Join(podDir, "0.log.1")
	require.NoError(t, os.Rename(logFile, rotatedFile))

	line2 := "2026-10-12T14:00:05.000000000Z stdout F Log line 2 after rotation\n"
	require.NoError(t, os.WriteFile(logFile, []byte(line2), 0644))

	// 3. Poll again - should detect rotation and read line 2
	tailer.pollOnce(ctx)

	entries := recWriter.getEntries()
	require.Len(t, entries, 2)
	assert.Equal(t, "Log line 2 after rotation", entries[1].Message)
}
