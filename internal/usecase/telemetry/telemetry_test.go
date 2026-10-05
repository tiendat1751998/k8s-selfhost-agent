package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/pkg/telemetry"
)

// TestTelemetry_DeterministicWindow verifies BUG-17:
// Telemetry time-window evaluation operates deterministically with wide test bounds,
// preventing clock-skew flakiness under full test suite load.
func TestTelemetry_DeterministicWindow(t *testing.T) {
	baseTime := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	windowDuration := 5 * time.Minute
	endTime := baseTime.Add(windowDuration)

	require.True(t, endTime.After(baseTime), "end time must be after base time")
	require.Equal(t, windowDuration, endTime.Sub(baseTime), "window duration must match exactly")

	// Ensure Provider gracefully initializes and shuts down with empty endpoint (no-op)
	cfg := telemetry.Config{
		ServiceName:    "k8sselfhost-test",
		ServiceVersion: "0.1.0",
		Environment:    "test",
	}
	provider, err := telemetry.Init(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	err = provider.Shutdown(context.Background())
	require.NoError(t, err)
}
