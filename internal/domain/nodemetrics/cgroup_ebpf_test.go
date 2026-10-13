package nodemetrics_test

import (
	"strings"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
)

func TestCGroupV2_ParseMemoryEvents(t *testing.T) {
	rawContent := `low 15
high 50
max 4
oom 1
oom_kill 1`

	event, err := nodemetrics.ParseCGroupMemoryEvents(rawContent)
	if err != nil {
		t.Fatalf("unexpected error parsing cgroup memory events: %v", err)
	}

	if event.LowEvents != 15 {
		t.Errorf("expected LowEvents 15, got %d", event.LowEvents)
	}
	if event.HighEvents != 50 {
		t.Errorf("expected HighEvents 50, got %d", event.HighEvents)
	}
	if event.MaxEvents != 4 {
		t.Errorf("expected MaxEvents 4, got %d", event.MaxEvents)
	}
	if event.OOMKillEvents != 1 {
		t.Errorf("expected OOMKillEvents 1, got %d", event.OOMKillEvents)
	}
}

func TestCGroupV2_ThrottlingSpikeDetection(t *testing.T) {
	// Delta > 10 in 1 min
	if !nodemetrics.IsThrottlingSpike(12, 1*time.Minute) {
		t.Errorf("expected 12 delta in 1m to trigger throttling spike")
	}
	if nodemetrics.IsThrottlingSpike(8, 1*time.Minute) {
		t.Errorf("expected 8 delta in 1m not to trigger throttling spike")
	}

	// Delta > 20 in 30s
	if !nodemetrics.IsThrottlingSpike(25, 30*time.Second) {
		t.Errorf("expected 25 delta in 30s to trigger throttling spike")
	}
	if nodemetrics.IsThrottlingSpike(15, 30*time.Second) {
		t.Errorf("expected 15 delta in 30s not to trigger throttling spike")
	}
}

func TestEBPF_AutoResizeCalculation(t *testing.T) {
	// 220,000 / 262,144 entries = 83.92% (CRITICAL)
	ratio, status, recommended := nodemetrics.CalculateEBPFRecommendation(220000, 262144)
	if ratio < 0.83 || ratio > 0.84 {
		t.Errorf("unexpected ratio: %f", ratio)
	}
	if status != nodemetrics.StatusCritical {
		t.Errorf("expected status CRITICAL, got %s", status)
	}
	if recommended != 524288 {
		t.Errorf("expected recommended 524288, got %d", recommended)
	}

	// 200,000 / 262,144 entries = 76.29% (WARNING)
	_, statusW, recommendedW := nodemetrics.CalculateEBPFRecommendation(200000, 262144)
	if statusW != nodemetrics.StatusWarning {
		t.Errorf("expected status WARNING, got %s", statusW)
	}
	if recommendedW != 524288 {
		t.Errorf("expected recommended 524288, got %d", recommendedW)
	}

	// 100,000 / 262,144 entries = 38.15% (NORMAL)
	_, statusN, recommendedN := nodemetrics.CalculateEBPFRecommendation(100000, 262144)
	if statusN != nodemetrics.StatusNormal {
		t.Errorf("expected status NORMAL, got %s", statusN)
	}
	if recommendedN != 262144 {
		t.Errorf("expected recommended to remain 262144, got %d", recommendedN)
	}

	// Ceiling test: 2,097,152 entries max
	_, _, recommendedMax := nodemetrics.CalculateEBPFRecommendation(1500000, 1500000)
	if recommendedMax != nodemetrics.MaxEBPFMapEntries {
		t.Errorf("expected capped at %d, got %d", nodemetrics.MaxEBPFMapEntries, recommendedMax)
	}
}

func TestEBPF_MaxCapacityGuardrail(t *testing.T) {
	hostRAMBytes := int64(16 * 1024 * 1024 * 1024) // 16GB -> 1.6GB limit

	// Safe: 1,048,576 entries * 256 bytes = 256MB (< 1.6GB)
	err := nodemetrics.ValidateEBPFMapMemory(1048576, hostRAMBytes)
	if err != nil {
		t.Errorf("expected safe entries to pass guardrail, got: %v", err)
	}

	// Exceeds ceiling: 3,000,000 > 2,097,152
	err = nodemetrics.ValidateEBPFMapMemory(3000000, hostRAMBytes)
	if err == nil {
		t.Errorf("expected rejection when exceeding ceiling, got nil")
	}

	// Exceeds 10% RAM: on 1GB host (10% = 100MB), 1,000,000 entries = 256MB > 100MB
	err = nodemetrics.ValidateEBPFMapMemory(1000000, 1024*1024*1024)
	if err == nil {
		t.Errorf("expected rejection when exceeding 10%% host RAM, got nil")
	}
}

func BenchmarkParseCGroupEvents(b *testing.B) {
	rawContent := strings.Repeat("low 12\nhigh 142\nmax 0\noom 0\noom_kill 0\n", 10)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := nodemetrics.ParseCGroupMemoryEvents(rawContent)
		if err != nil {
			b.Fatal(err)
		}
	}
}
