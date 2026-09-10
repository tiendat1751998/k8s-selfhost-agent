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
	// Missing TenantID
	err := entry.Validate()
	require.ErrorIs(t, err, logging.ErrInvalidLogQuery)

	entry.TenantID = "tenant-1"
	err = entry.Validate()
	require.NoError(t, err)
	require.False(t, entry.Timestamp.IsZero())
	require.Equal(t, "stdout", entry.Stream)
	require.Equal(t, logging.LogLevelInfo, entry.LogLevel)
}

func TestLogFilter_ValidateAndSanitize(t *testing.T) {
	filter := logging.LogFilter{
		TenantID: "tenant-1",
		Limit:    500, // Should be clamped to 100
	}
	require.NoError(t, filter.Validate())

	filter.Sanitize()
	require.Equal(t, 100, filter.Limit)
	require.False(t, filter.StartTime.IsZero())
	require.False(t, filter.EndTime.IsZero())
	require.True(t, filter.EndTime.After(filter.StartTime))

	// Invalid time range
	invalidFilter := logging.LogFilter{
		TenantID:  "tenant-1",
		StartTime: time.Now().Add(1 * time.Hour),
		EndTime:   time.Now(),
	}
	require.ErrorIs(t, invalidFilter.Validate(), logging.ErrInvalidLogQuery)
}
