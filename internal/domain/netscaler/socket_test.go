package netscaler_test

import (
	"sync"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/netscaler"
)

func TestCacheLinePaddingAlignment(t *testing.T) {
	counter := netscaler.PaddedCounter{}
	// The struct size must be exactly 64 bytes (1 cache line)
	assert.Equal(t, uintptr(64), unsafe.Sizeof(counter), "PaddedCounter must occupy exactly 64 bytes for cache line isolation")
}

func TestPaddedCounter_ConcurrentOperations(t *testing.T) {
	counter := &netscaler.PaddedCounter{}
	const goroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				counter.Inc()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(goroutines*iterations), counter.Load())

	counter.Add(-500)
	assert.Equal(t, int64(goroutines*iterations-500), counter.Load())

	counter.Store(42)
	assert.Equal(t, int64(42), counter.Load())
}

func TestNewSocketMetrics_Valid(t *testing.T) {
	metrics, err := netscaler.NewSocketMetrics(
		23000,  // time wait
		24000,  // allocated ports
		28232,  // total ports
		5,      // backlog drops
		200000, // conntrack entries
		262144, // conntrack max
	)

	require.NoError(t, err)
	require.NotNil(t, metrics)
	assert.Equal(t, int64(23000), metrics.TCPTimeWaitCount)
	assert.Equal(t, int64(24000), metrics.AllocatedPorts)
	assert.Equal(t, int64(28232), metrics.TotalEphemeralPorts)
	assert.InDelta(t, 85.01, metrics.PortUtilizationPct, 0.01)
	assert.Equal(t, int64(5), metrics.ListenBacklogDrops)
	assert.Equal(t, int64(200000), metrics.ConntrackEntries)
	assert.Equal(t, int64(262144), metrics.ConntrackMax)
	assert.InDelta(t, 76.29, metrics.ConntrackUsagePct, 0.01)

	assert.True(t, metrics.IsPortStarved())
	assert.True(t, metrics.IsConntrackSaturated())
	assert.True(t, metrics.HasListenDrops())
}

func TestNewSocketMetrics_InvalidInputs(t *testing.T) {
	_, err := netscaler.NewSocketMetrics(-1, 100, 28232, 0, 100, 1000)
	assert.Error(t, err)

	_, err = netscaler.NewSocketMetrics(100, -1, 28232, 0, 100, 1000)
	assert.Error(t, err)

	_, err = netscaler.NewSocketMetrics(100, 100, 0, 0, 100, 1000)
	assert.Error(t, err)

	_, err = netscaler.NewSocketMetrics(100, 100, 28232, -1, 100, 1000)
	assert.Error(t, err)

	_, err = netscaler.NewSocketMetrics(100, 100, 28232, 0, 100, -1)
	assert.Error(t, err)
}

func TestKernelNetConfig_Validate(t *testing.T) {
	validCfg := netscaler.KernelNetConfig{
		Somaxconn:            65535,
		TCPMaxSynBacklog:     65535,
		TCPTwReuse:           1,
		ConntrackMax:         1048576,
		ConntrackTCPTimeWait: 30,
	}
	assert.NoError(t, validCfg.Validate())

	invalidSomaxconn := validCfg
	invalidSomaxconn.Somaxconn = 0
	assert.Error(t, invalidSomaxconn.Validate())

	invalidSyn := validCfg
	invalidSyn.TCPMaxSynBacklog = -10
	assert.Error(t, invalidSyn.Validate())

	invalidTwReuse := validCfg
	invalidTwReuse.TCPTwReuse = 5
	assert.Error(t, invalidTwReuse.Validate())

	invalidCtMax := validCfg
	invalidCtMax.ConntrackMax = 5000000 // exceeds safety ceiling
	assert.Error(t, invalidCtMax.Validate())

	zeroCtMax := validCfg
	zeroCtMax.ConntrackMax = 0
	assert.Error(t, zeroCtMax.Validate())
}

func TestCoreSocketTelemetry_PaddingAndIsolation(t *testing.T) {
	telemetry := netscaler.CoreSocketTelemetry{}
	// Verify that each counter address differs by at least 64 bytes to guarantee cache line separation
	p1 := uintptr(unsafe.Pointer(&telemetry.TimeWaitCounter))
	p2 := uintptr(unsafe.Pointer(&telemetry.AllocatedPortCounter))
	p3 := uintptr(unsafe.Pointer(&telemetry.ListenDropCounter))
	p4 := uintptr(unsafe.Pointer(&telemetry.ConntrackEntryCounter))

	assert.GreaterOrEqual(t, p2-p1, uintptr(64))
	assert.GreaterOrEqual(t, p3-p2, uintptr(64))
	assert.GreaterOrEqual(t, p4-p3, uintptr(64))
}
