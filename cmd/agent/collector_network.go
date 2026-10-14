package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type netDevStat struct {
	rxBytes int64
	txBytes int64
}

func (c *SystemCollector) collectNetwork(now time.Time) NetworkMetrics {
	netDevPath := filepath.Join(c.procPath, "net", "dev")
	data, err := os.ReadFile(netDevPath)
	if err != nil {
		return NetworkMetrics{
			Interfaces:         make([]NetworkInterfaceMetrics, 0),
			TotalRxBytesPerSec: 0,
			TotalTxBytesPerSec: 0,
		}
	}

	c.mu.RLock()
	prevNetStats := c.prevNetStats
	prevNetTime := c.prevNetTime
	c.mu.RUnlock()

	var elapsed float64
	if !prevNetTime.IsZero() {
		elapsed = now.Sub(prevNetTime).Seconds()
	}
	if elapsed <= 0 {
		elapsed = 1.0
	}

	interfaces := make([]NetworkInterfaceMetrics, 0)
	newNetStats := make(map[string]netDevStat)
	var totalRx, totalTx int64

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}

		rxBytes, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			slog.Warn("failed to parse network rxBytes", slog.String("interface", iface), slog.Any("error", err))
			continue
		}
		txBytes, err := strconv.ParseInt(fields[8], 10, 64)
		if err != nil {
			slog.Warn("failed to parse network txBytes", slog.String("interface", iface), slog.Any("error", err))
			continue
		}

		var rxRate, txRate int64
		prev, ok := prevNetStats[iface]
		if ok && elapsed > 0 {
			if rxBytes >= prev.rxBytes {
				rxRate = int64(float64(rxBytes-prev.rxBytes) / elapsed)
			}
			if txBytes >= prev.txBytes {
				txRate = int64(float64(txBytes-prev.txBytes) / elapsed)
			}
		}

		newNetStats[iface] = netDevStat{
			rxBytes: rxBytes,
			txBytes: txBytes,
		}

		interfaces = append(interfaces, NetworkInterfaceMetrics{
			Name:          iface,
			RxBytesPerSec: rxRate,
			TxBytesPerSec: txRate,
		})

		if isPhysicalNetworkInterface(iface) {
			totalRx += rxRate
			totalTx += txRate
		}
	}

	c.mu.Lock()
	c.prevNetStats = newNetStats
	c.prevNetTime = now
	c.mu.Unlock()

	return NetworkMetrics{
		Interfaces:         interfaces,
		TotalRxBytesPerSec: totalRx,
		TotalTxBytesPerSec: totalTx,
	}
}

// isPhysicalNetworkInterface returns true for physical host network interfaces (eth*, en*, wl*, bond*)
// and filters out loopback (lo), container veths (veth*), and bridge devices (docker0, br-*, cni*).
func isPhysicalNetworkInterface(name string) bool {
	if name == "lo" || strings.HasPrefix(name, "veth") || name == "docker0" || strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "cni") {
		return false
	}
	return strings.HasPrefix(name, "eth") ||
		strings.HasPrefix(name, "en") ||
		strings.HasPrefix(name, "wl") ||
		strings.HasPrefix(name, "bond")
}

