package rdmawatch

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestPaddedStructs_Initialization(t *testing.T) {
	acc := PaddedPFCAccumulator{}
	assert.Equal(t, uint64(0), acc.GetRxPauses())

	mon := PaddedFabricBandwidthMonitor{}
	assert.Equal(t, float64(0), mon.GetBandwidthGBps())

	cnt := PaddedRDMACounters{}
	assert.Equal(t, uint64(0), cnt.pfcPausesRx.Load())
}

func TestPaddedPFCAccumulator_ConcurrentAccess(t *testing.T) {
	acc := &PaddedPFCAccumulator{}
	const goroutines = 50
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				rx := uint64(id*iterations + i)
				tx := uint64(id*iterations + i/2)
				acc.Record(rx, tx, 1000)
			}
		}(g)
	}

	wg.Wait()

	assert.Positive(t, acc.GetRxPauses())
	assert.Positive(t, acc.GetTxPauses())
	assert.Positive(t, acc.GetRxRateSec())
}

func TestPaddedFabricBandwidthMonitor_ConcurrentAccess(t *testing.T) {
	mon := &PaddedFabricBandwidthMonitor{}
	const goroutines = 50
	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				mon.Record(895.5, uint64(id))
			}
		}(g)
	}

	wg.Wait()

	assert.Equal(t, uint64(goroutines*iterations), mon.GetSampleCount())
	assert.InDelta(t, 895.5, mon.GetBandwidthGBps(), 0.001)
}

func TestPaddedRDMACounters_ConcurrentSafety(t *testing.T) {
	c := &PaddedRDMACounters{}
	const goroutines = 30
	const iterations = 300

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Writer goroutines
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				c.RecordTelemetry(uint64(id*100), uint64(id*50), 900.0, uint64(i))
				c.IncrementDeadlockAlert()
			}
		}(g)
	}

	// Reader goroutines
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = c.GetPFCPausesRx()
				_ = c.GetPFCPausesTx()
				_ = c.GetBandwidthGBps()
				_ = c.GetRxDrops()
				_ = c.GetDeadlockAlerts()
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, uint64(goroutines*iterations), c.GetDeadlockAlerts())
	assert.InDelta(t, 900.0, c.GetBandwidthGBps(), 0.001)
}

func TestDomainErrors_FormattingAndUnwrap(t *testing.T) {
	require.Error(t, ErrRDMAPFCDeadlock)
	require.Error(t, ErrGPUFabricAsymmetric)
	require.Error(t, ErrRDMAMemlockRestricted)

	var domErr *domainerrors.DomainError
	require.True(t, errors.As(ErrRDMAPFCDeadlock, &domErr))
	assert.Equal(t, CodeRDMAPFCDeadlock, domErr.Code)

	require.True(t, errors.As(ErrGPUFabricAsymmetric, &domErr))
	assert.Equal(t, CodeGPUFabricAsymmetric, domErr.Code)

	require.True(t, errors.As(ErrRDMAMemlockRestricted, &domErr))
	assert.Equal(t, CodeRDMAMemlockRestricted, domErr.Code)
}
