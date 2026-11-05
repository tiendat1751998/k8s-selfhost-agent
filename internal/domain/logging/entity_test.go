package logging_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
)

func TestLogLevel_ParseAndValidation(t *testing.T) {
	tests := []struct {
		input    string
		expected logging.LogLevel
		valid    bool
	}{
		{"info", logging.LogLevelInfo, true},
		{"INFO", logging.LogLevelInfo, true},
		{"warn", logging.LogLevelWarn, true},
		{"warning", logging.LogLevelWarn, true},
		{"error", logging.LogLevelError, true},
		{"DEBUG", logging.LogLevelDebug, true},
		{"trace", logging.LogLevelTrace, true},
		{"fatal", logging.LogLevelFatal, true},
		{"invalid", "", false},
	}

	for _, tc := range tests {
		lvl, err := logging.ParseLogLevel(tc.input)
		if tc.valid {
			require.NoError(t, err)
			require.Equal(t, tc.expected, lvl)
			require.True(t, lvl.IsValid())
		} else {
			require.Error(t, err)
			require.False(t, lvl.IsValid())
		}
	}
}

func TestLogEntry_Validate(t *testing.T) {
	entry := logging.LogEntry{
		ClusterID: "cluster-1",
		Message:   "pod started",
	}
	// Missing TenantID returns ErrLogIngestionFailed
	err := entry.Validate()
	require.ErrorIs(t, err, logging.ErrLogIngestionFailed)

	entry.TenantID = "tenant-1"
	err = entry.Validate()
	require.NoError(t, err)
	require.False(t, entry.Timestamp.IsZero())
	require.Equal(t, "stdout", entry.Stream)
	require.Equal(t, logging.LogLevelInfo, entry.LogLevel)

	// Invalid log level returns ErrLogIngestionFailed
	entry.LogLevel = "bogus"
	err = entry.Validate()
	require.ErrorIs(t, err, logging.ErrLogIngestionFailed)
}

func TestLogFilter_ValidateAndSanitize(t *testing.T) {
	// Limit <= 0 defaults to 500
	defaultFilter := logging.LogFilter{
		TenantID: "tenant-1",
		Limit:    0,
	}
	defaultFilter.Sanitize()
	require.Equal(t, 500, defaultFilter.Limit)

	// Limit within [1, 10000] is preserved
	midFilter := logging.LogFilter{
		TenantID: "tenant-1",
		Limit:    5000,
	}
	midFilter.Sanitize()
	require.Equal(t, 5000, midFilter.Limit)

	// Limit > 10000 is clamped to 10000
	excessiveFilter := logging.LogFilter{
		TenantID: "tenant-1",
		Limit:    20000,
	}
	require.NoError(t, excessiveFilter.Validate())
	excessiveFilter.Sanitize()
	require.Equal(t, 10000, excessiveFilter.Limit)
	require.True(t, excessiveFilter.StartTime.IsZero())
	require.True(t, excessiveFilter.EndTime.IsZero())

	// Invalid log level in filter returns ErrInvalidLogQuery
	invalidLevelFilter := logging.LogFilter{
		TenantID: "tenant-1",
		LogLevel: "unknown",
	}
	require.ErrorIs(t, invalidLevelFilter.Validate(), logging.ErrInvalidLogQuery)

	// Invalid time range
	invalidFilter := logging.LogFilter{
		TenantID:  "tenant-1",
		StartTime: time.Now().Add(1 * time.Hour),
		EndTime:   time.Now(),
	}
	require.ErrorIs(t, invalidFilter.Validate(), logging.ErrInvalidLogQuery)
}

func TestNormalizeLogLevel_KlogAndStructured(t *testing.T) {
	cases := []struct {
		name       string
		levelInput string
		message    string
		want       logging.LogLevel
	}{
		{
			name:       "klog error header",
			levelInput: "",
			message:    `E1105 04:16:25.113311 1452 kubelet_node_status.go:106] "Unable to register node with API server" err="connection refused"`,
			want:       logging.LogLevelError,
		},
		{
			name:       "klog error header with default info level",
			levelInput: "info",
			message:    `E1105 04:16:25.113311 1452 kubelet_node_status.go:106] "Unable to register node with API server"`,
			want:       logging.LogLevelError,
		},
		{
			name:       "klog fatal header",
			levelInput: "",
			message:    `F1105 04:16:25.113311 1452 kubelet.go:106] "Fatal panic"`,
			want:       logging.LogLevelError,
		},
		{
			name:       "klog warning header",
			levelInput: "info",
			message:    `W1105 04:16:25.113311 1452 kubelet.go:106] "Disk space running low"`,
			want:       logging.LogLevelWarn,
		},
		{
			name:       "klog info header",
			levelInput: "",
			message:    `I1105 04:16:25.113311 1452 kubelet.go:106] "Syncing pods"`,
			want:       logging.LogLevelInfo,
		},
		{
			name:       "syslog-prefixed klog error header",
			levelInput: "info",
			message:    `Nov  5 04:16:25 node-1 kubelet[1452]: E1105 04:16:25.113311 1452 kubelet_node_status.go:106] "Unable to register node"`,
			want:       logging.LogLevelError,
		},
		{
			name:       "structured token level=error",
			levelInput: "info",
			message:    `2026-11-05T04:16:25Z level=error caller=server.go:42 msg="failed to dial db"`,
			want:       logging.LogLevelError,
		},
		{
			name:       "structured JSON level:error",
			levelInput: "",
			message:    `{"time":"2026-11-05T04:16:25Z","level":"error","msg":"connection reset by peer"}`,
			want:       logging.LogLevelError,
		},
		{
			name:       "structured token level=warn",
			levelInput: "",
			message:    `level=warn caller=pool.go:88 msg="connection slow"`,
			want:       logging.LogLevelWarn,
		},
		{
			name:       "preserves explicit warning level",
			levelInput: "warn",
			message:    `plain message without tokens`,
			want:       logging.LogLevelWarn,
		},
		{
			name:       "preserves explicit error level",
			levelInput: "error",
			message:    `I1105 04:16:25.113311 1452 sync.go:42] "Sync ok"`,
			want:       logging.LogLevelError,
		},
		{
			name:       "plain message defaults to info",
			levelInput: "",
			message:    `just a normal operational log`,
			want:       logging.LogLevelInfo,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := logging.NormalizeLogLevel(tc.levelInput, tc.message)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestLogEntry_Validate_NormalizesKlogLevel(t *testing.T) {
	entry := logging.LogEntry{
		TenantID:  "tenant-klog",
		ClusterID: "cluster-klog",
		Message:   `E1105 04:16:25.113311 1452 kubelet_node_status.go:106] "Unable to register node with API server"`,
		LogLevel:  logging.LogLevelInfo,
	}

	err := entry.Validate()
	require.NoError(t, err)
	require.Equal(t, logging.LogLevelError, entry.LogLevel)
}