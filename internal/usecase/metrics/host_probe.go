package metrics

import (
	"context"
	"os"
	"runtime"
	"strings"
	"time"
)

// HostTelemetry holds raw measured operating system and hardware resource metrics for the host machine.
type HostTelemetry struct {
	Hostname      string
	OS            string
	Arch          string
	OSDistro      string
	KernelVersion string
	CPUPercent    float64
	CPUCount      int
	MemoryUsed    int64
	MemoryTotal   int64
	MemoryPercent float64
	Processes     int
	UptimeSeconds int64
}

// HostProber defines the capability to probe host OS hardware and resource telemetry.
type HostProber interface {
	ProbeHost(ctx context.Context) HostTelemetry
}

var defaultProber HostProber = newPlatformHostProber()

func getHostTelemetry(ctx context.Context) HostTelemetry {
	if defaultProber != nil {
		return defaultProber.ProbeHost(ctx)
	}
	return probeFallbackHostTelemetry()
}

func probeFallbackHostTelemetry() HostTelemetry {
	hName, _ := os.Hostname()
	if hName == "" {
		hName = "standalone-host"
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	usedMem := int64(m.Alloc)
	sysMem := int64(m.Sys)
	if sysMem < usedMem {
		sysMem = usedMem
	}
	memPct := 0.0
	if sysMem > 0 {
		memPct = float64(usedMem) / float64(sysMem) * 100.0
	}

	return HostTelemetry{
		Hostname:      hName,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		OSDistro:      strings.Title(runtime.GOOS),
		CPUCount:      runtime.NumCPU(),
		MemoryUsed:    usedMem,
		MemoryTotal:   sysMem,
		MemoryPercent: memPct,
		Processes:     runtime.NumGoroutine(),
		UptimeSeconds: int64(time.Since(processStartTime).Seconds()),
	}
}

var processStartTime = time.Now()
