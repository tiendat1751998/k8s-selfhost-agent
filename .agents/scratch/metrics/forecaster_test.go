package metrics

import (
	"testing"
	"time"
)

func TestForecaster_LinearRegression(t *testing.T) {
	tests := []struct {
		name          string
		data          []float64
		expectedSlope float64
		expectedInt   float64
	}{
		{
			name:          "empty",
			data:          []float64{},
			expectedSlope: 0,
			expectedInt:   0,
		},
		{
			name:          "single point",
			data:          []float64{50.0},
			expectedSlope: 0,
			expectedInt:   50.0,
		},
		{
			name:          "linear increasing trend",
			data:          []float64{10.0, 20.0, 30.0, 40.0},
			expectedSlope: 10.0,
			expectedInt:   10.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slope, intercept := LinearRegression(tt.data)
			if slope != tt.expectedSlope || intercept != tt.expectedInt {
				t.Errorf("LinearRegression() = (%v, %v), want (%v, %v)", slope, intercept, tt.expectedSlope, tt.expectedInt)
			}
		})
	}
}

func TestForecaster_ForecastLinearTrend(t *testing.T) {
	history := []float64{10.0, 20.0, 30.0}
	proj1 := ForecastLinearTrend(history, 1)
	if proj1 != 40.0 {
		t.Errorf("expected 40.0, got %f", proj1)
	}

	projCapped := ForecastLinearTrend(history, 20)
	if projCapped != 100.0 {
		t.Errorf("expected capped at 100.0, got %f", projCapped)
	}
}

func TestForecaster_CalculateTimeToExhaustion(t *testing.T) {
	// Current 50%, delta 5% per day -> 10 days
	duration, ok := CalculateTimeToExhaustion(50.0, 5.0, 100.0)
	if !ok {
		t.Fatal("expected exhaustion duration calculated")
	}
	expectedDays := 10.0
	actualDays := duration.Hours() / 24.0
	if actualDays != expectedDays {
		t.Errorf("expected %f days, got %f", expectedDays, actualDays)
	}

	// Zero or negative delta -> no exhaustion
	_, okZero := CalculateTimeToExhaustion(50.0, 0.0, 100.0)
	if okZero {
		t.Error("expected false for zero growth rate")
	}
}

func TestForecaster_ForecastSystem(t *testing.T) {
	forecaster := NewResourceForecaster()

	overview := &SystemOverview{
		TotalCPUPercent:  45.0,
		TotalMemPercent:  60.0,
		TotalDiskPercent: 30.0,
		Nodes: []NodeMetrics{
			{
				NodeID:        "node-1",
				NodeName:      "master-1",
				CPUPercent:    45.0,
				MemoryPercent: 60.0,
				DiskPercent:   30.0,
			},
		},
		CollectedAt: time.Now().UTC(),
	}

	sysForecast := forecaster.ForecastSystem(overview)
	if sysForecast == nil {
		t.Fatal("expected non-nil system forecast")
	}

	if sysForecast.CPU.CurrentUsage != 45.0 {
		t.Errorf("expected CPU current usage 45.0, got %f", sysForecast.CPU.CurrentUsage)
	}
	if len(sysForecast.PerNode) != 1 {
		t.Fatalf("expected 1 node forecast, got %d", len(sysForecast.PerNode))
	}
	if sysForecast.PerNode[0].NodeID != "node-1" {
		t.Errorf("expected nodeID node-1, got %s", sysForecast.PerNode[0].NodeID)
	}
}
