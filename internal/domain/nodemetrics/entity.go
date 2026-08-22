// Package nodemetrics provides domain entities and repository ports for per-node historical time-series telemetry.
package nodemetrics

import "time"

// NodeMetricRollup represents a consolidated hardware and workload telemetry sample for a single compute node.
type NodeMetricRollup struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	NodeID         string    `json:"node_id"`
	NodeName       string    `json:"node_name"`
	CPUPercent     float64   `json:"cpu_percent"`
	CPUPeak        float64   `json:"cpu_peak"`
	MemUsedBytes   int64     `json:"mem_used_bytes"`
	MemTotalBytes  int64     `json:"mem_total_bytes"`
	MemPercent     float64   `json:"mem_percent"`
	DiskUsedBytes  int64     `json:"disk_used_bytes"`
	DiskTotalBytes int64     `json:"disk_total_bytes"`
	DiskPercent    float64   `json:"disk_percent"`
	RxBytesPerSec  int64     `json:"rx_bytes_per_sec"`
	TxBytesPerSec  int64     `json:"tx_bytes_per_sec"`
	ProcessCount   int       `json:"process_count"`
	ContainerCount int       `json:"container_count"`
	Status         string    `json:"status"`     // "online" | "degraded" | "offline"
	Resolution     string    `json:"resolution"` // "1m" | "1h" | "1d"
	RecordedAt     time.Time `json:"recorded_at"`
}

// NodeHistoryQuery contains parameters for fetching node historical telemetry.
type NodeHistoryQuery struct {
	TenantID   string
	NodeID     string
	Resolution string
	StartTime  time.Time
	EndTime    time.Time
	Limit      int
}

// NodeHistoricalSummary provides high-level summary KPIs over the requested window.
type NodeHistoricalSummary struct {
	NodeID         string    `json:"node_id"`
	NodeName       string    `json:"node_name"`
	AvgCPUPercent  float64   `json:"avg_cpu_percent"`
	PeakCPUPercent float64   `json:"peak_cpu_percent"`
	AvgMemPercent  float64   `json:"avg_mem_percent"`
	PeakMemPercent float64   `json:"peak_mem_percent"`
	PeakRxBytesSec int64     `json:"peak_rx_bytes_sec"`
	PeakTxBytesSec int64     `json:"peak_tx_bytes_sec"`
	UptimePercent  float64   `json:"uptime_percent"`
	OfflineCount   int       `json:"offline_count"`
	TotalSamples   int       `json:"total_samples"`
	WindowStart    time.Time `json:"window_start"`
	WindowEnd      time.Time `json:"window_end"`
}
