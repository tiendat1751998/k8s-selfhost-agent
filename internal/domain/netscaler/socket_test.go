package netscaler_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/netscaler"
)

func TestPaddedCounter_Initialization(t *testing.T) {
	counter := netscaler.PaddedCounter{}
	assert.Equal(t, int64(0), counter.Load())
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

func TestCoreSocketTelemetry_Operations(t *testing.T) {
	telemetry := netscaler.CoreSocketTelemetry{}
	assert.Equal(t, int64(0), telemetry.TimeWaitCounter.Load())
	assert.Equal(t, int64(0), telemetry.AllocatedPortCounter.Load())
	assert.Equal(t, int64(0), telemetry.ListenDropCounter.Load())
	assert.Equal(t, int64(0), telemetry.ConntrackEntryCounter.Load())

	telemetry.TimeWaitCounter.Add(10)
	telemetry.AllocatedPortCounter.Add(20)
	telemetry.ListenDropCounter.Add(5)
	telemetry.ConntrackEntryCounter.Add(100)

	assert.Equal(t, int64(10), telemetry.TimeWaitCounter.Load())
	assert.Equal(t, int64(20), telemetry.AllocatedPortCounter.Load())
	assert.Equal(t, int64(5), telemetry.ListenDropCounter.Load())
	assert.Equal(t, int64(100), telemetry.ConntrackEntryCounter.Load())
}
