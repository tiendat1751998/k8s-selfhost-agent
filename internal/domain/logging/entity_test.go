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
	// Limit <= 0 defaults to 100
	defaultFilter := logging.LogFilter{
		TenantID: "tenant-1",
		Limit:    0,
	}
	defaultFilter.Sanitize()
	require.Equal(t, 100, defaultFilter.Limit)

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
	require.False(t, excessiveFilter.StartTime.IsZero())
	require.False(t, excessiveFilter.EndTime.IsZero())
	require.True(t, excessiveFilter.EndTime.After(excessiveFilter.StartTime))

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