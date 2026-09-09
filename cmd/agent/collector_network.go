package main

import (
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

	var elapsed float64
	if !c.prevNetTime.IsZero() {
		elapsed = now.Sub(c.prevNetTime).Seconds()
	}
	if elapsed <= 0 {
		elapsed = 1.0
	}

	interfaces := make([]NetworkInterfaceMetrics, 0)
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

		rxBytes, _ := strconv.ParseInt(fields[0], 10, 64)
		txBytes, _ := strconv.ParseInt(fields[8], 10, 64)

		var rxRate, txRate int64
		prev, ok := c.prevNetStats[iface]
		if ok && elapsed > 0 {
			if rxBytes >= prev.rxBytes {
				rxRate = int64(float64(rxBytes-prev.rxBytes) / elapsed)
			}
			if txBytes >= prev.txBytes {
				txRate = int64(float64(txBytes-prev.txBytes) / elapsed)
			}
		}

		c.prevNetStats[iface] = netDevStat{
			rxBytes: rxBytes,
			txBytes: txBytes,
		}

		interfaces = append(interfaces, NetworkInterfaceMetrics{
			Name:          iface,
			RxBytesPerSec: rxRate,
			TxBytesPerSec: txRate,
		})

		totalRx += rxRate
		totalTx += txRate
	}

	c.prevNetTime = now

	return NetworkMetrics{
		Interfaces:         interfaces,
		TotalRxBytesPerSec: totalRx,
		TotalTxBytesPerSec: totalTx,
	}
}
