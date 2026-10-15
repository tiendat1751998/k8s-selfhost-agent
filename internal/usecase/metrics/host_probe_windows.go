//go:build windows
package metrics

import (
	"context"
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32DLL              = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32DLL.NewProc("GlobalMemoryStatusEx")
	procGetSystemTimes       = kernel32DLL.NewProc("GetSystemTimes")
	procGetTickCount64       = kernel32DLL.NewProc("GetTickCount64")
)

type memoryStatusEx struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

type platformHostProber struct {
	mu           sync.Mutex
	prevIdle     uint64
	prevKernel   uint64
	prevUser     uint64
	prevTime     time.Time
	prevCPUUsage float64
}

func newPlatformHostProber() HostProber {
	p := &platformHostProber{
		prevTime: time.Now(),
	}
	var idle, kernel, user windows.Filetime
	if procGetSystemTimes.Find() == nil {
		r1, _, _ := procGetSystemTimes.Call(
			uintptr(unsafe.Pointer(&idle)),
			uintptr(unsafe.Pointer(&kernel)),
			uintptr(unsafe.Pointer(&user)),
		)
		if r1 != 0 {
			p.prevIdle = fileTimeToUint64(idle)
			p.prevKernel = fileTimeToUint64(kernel)
			p.prevUser = fileTimeToUint64(user)
		}
	}
	return p
}

func fileTimeToUint64(ft windows.Filetime) uint64 {
	return (uint64(ft.HighDateTime) << 32) | uint64(ft.LowDateTime)
}

func (p *platformHostProber) ProbeHost(ctx context.Context) HostTelemetry {
	hName, _ := os.Hostname()
	if hName == "" {
		hName = "windows-host"
	}

	// 1. Real Physical RAM via GlobalMemoryStatusEx
	var totalMem, usedMem int64
	var memPercent float64

	if procGlobalMemoryStatusEx.Find() == nil {
		var mem memoryStatusEx
		mem.cbSize = uint32(unsafe.Sizeof(mem))
		r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))
		if r1 != 0 {
			totalMem = int64(mem.ullTotalPhys)
			if mem.ullTotalPhys >= mem.ullAvailPhys {
				usedMem = int64(mem.ullTotalPhys - mem.ullAvailPhys)
			}
			memPercent = float64(mem.dwMemoryLoad)
		}
	}

	if totalMem <= 0 {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		totalMem = int64(ms.Sys)
		usedMem = int64(ms.Alloc)
		if totalMem > 0 {
			memPercent = (float64(usedMem) / float64(totalMem)) * 100.0
		}
	}

	// 2. Real System CPU via GetSystemTimes delta
	p.mu.Lock()
	defer p.mu.Unlock()

	cpuPercent := p.prevCPUUsage
	var idle, kernel, user windows.Filetime
	now := time.Now()

	if procGetSystemTimes.Find() == nil {
		r1, _, _ := procGetSystemTimes.Call(
			uintptr(unsafe.Pointer(&idle)),
			uintptr(unsafe.Pointer(&kernel)),
			uintptr(unsafe.Pointer(&user)),
		)
		if r1 != 0 {
			currIdle := fileTimeToUint64(idle)
			currKernel := fileTimeToUint64(kernel)
			currUser := fileTimeToUint64(user)

			if p.prevKernel > 0 && p.prevUser > 0 {
				kernelDelta := currKernel - p.prevKernel
				userDelta := currUser - p.prevUser
				idleDelta := currIdle - p.prevIdle
				totalDelta := kernelDelta + userDelta

				if totalDelta > idleDelta && totalDelta > 0 {
					busyDelta := totalDelta - idleDelta
					usage := (float64(busyDelta) / float64(totalDelta)) * 100.0
					if usage >= 0.0 && usage <= 100.0 {
						cpuPercent = usage
						p.prevCPUUsage = usage
					}
				} else if totalDelta > 0 && idleDelta >= totalDelta {
					cpuPercent = 0.0
					p.prevCPUUsage = 0.0
				}
			}

			p.prevIdle = currIdle
			p.prevKernel = currKernel
			p.prevUser = currUser
			p.prevTime = now
		}
	}

	// 3. Uptime
	uptimeSec := int64(time.Since(processStartTime).Seconds())
	if procGetTickCount64.Find() == nil {
		ret, _, _ := procGetTickCount64.Call()
		if ret > 0 {
			uptimeSec = int64(ret / 1000)
		}
	}

	return HostTelemetry{
		Hostname:      hName,
		OS:            "windows",
		Arch:          runtime.GOARCH,
		OSDistro:      "Windows",
		CPUPercent:    cpuPercent,
		CPUCount:      runtime.NumCPU(),
		MemoryUsed:    usedMem,
		MemoryTotal:   totalMem,
		MemoryPercent: memPercent,
		Processes:     runtime.NumGoroutine(),
		UptimeSeconds: uptimeSec,
	}
}
